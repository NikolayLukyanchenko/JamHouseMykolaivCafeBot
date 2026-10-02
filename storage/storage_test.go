package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/models"
)

func TestRecordSaleUpdatesStockAndDailyReport(t *testing.T) {
	ctx := context.Background()
	store, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer store.Close()

	if err := store.Init(ctx); err != nil {
		t.Fatalf("Init() error: %v", err)
	}
	if err := store.SeedAdmins(ctx, []int64{1}); err != nil {
		t.Fatalf("SeedAdmins() error: %v", err)
	}

	productID, err := store.CreateProduct(ctx, models.Product{
		Name:      "Лате",
		Category:  models.CategoryDrinks,
		CostPrice: 30,
		SellPrice: 70,
		Unit:      "чашка",
		Stock:     10,
		IsActive:  true,
	})
	if err != nil {
		t.Fatalf("CreateProduct() error: %v", err)
	}

	sale, items, err := store.RecordSale(ctx, 1, models.PaymentCash, []models.OrderItem{{ProductID: productID, Qty: 2}})
	if err != nil {
		t.Fatalf("RecordSale() error: %v", err)
	}
	if sale.Total != 140 || sale.CostTotal != 60 || len(items) != 1 {
		t.Fatalf("unexpected sale result: %+v %#v", sale, items)
	}

	product, err := store.GetProduct(ctx, productID)
	if err != nil {
		t.Fatalf("GetProduct() error: %v", err)
	}
	if product.Stock != 8 {
		t.Fatalf("unexpected stock after sale: %v", product.Stock)
	}

	report, err := store.GetDailyReport(ctx, time.Now())
	if err != nil {
		t.Fatalf("GetDailyReport() error: %v", err)
	}
	if report.TotalRevenue != 140 || report.CostTotal != 60 || report.Profit != 80 {
		t.Fatalf("unexpected report totals: %+v", report)
	}
	if len(report.Items) != 1 || report.Items[0].Qty != 2 {
		t.Fatalf("unexpected report items: %+v", report.Items)
	}
}

func TestPurchaseItemsCRUD(t *testing.T) {
	ctx := context.Background()
	store, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer store.Close()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	milkID, err := store.CreatePurchaseItem(ctx, "Молоко", "л")
	if err != nil {
		t.Fatalf("CreatePurchaseItem() error: %v", err)
	}
	if _, err := store.CreatePurchaseItem(ctx, "Вершки", ""); err != nil {
		t.Fatalf("CreatePurchaseItem() error: %v", err)
	}

	items, err := store.ListPurchaseItems(ctx)
	if err != nil {
		t.Fatalf("ListPurchaseItems() error: %v", err)
	}
	if len(items) != 2 || items[0].Name != "Вершки" || items[1].Name != "Молоко" || items[1].Unit != "л" {
		t.Fatalf("unexpected items: %#v", items)
	}

	if err := store.UpdatePurchaseItem(ctx, milkID, "Молоко 2.5%", "пак"); err != nil {
		t.Fatalf("UpdatePurchaseItem() error: %v", err)
	}
	item, err := store.GetPurchaseItem(ctx, milkID)
	if err != nil {
		t.Fatalf("GetPurchaseItem() error: %v", err)
	}
	if item.Name != "Молоко 2.5%" || item.Unit != "пак" {
		t.Fatalf("unexpected item after update: %#v", item)
	}

	if err := store.DeletePurchaseItem(ctx, milkID); err != nil {
		t.Fatalf("DeletePurchaseItem() error: %v", err)
	}
	items, err = store.ListPurchaseItems(ctx)
	if err != nil {
		t.Fatalf("ListPurchaseItems() error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item after delete, got %d", len(items))
	}
}
