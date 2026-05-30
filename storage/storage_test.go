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
