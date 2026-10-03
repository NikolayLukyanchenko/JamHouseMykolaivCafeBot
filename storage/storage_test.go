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

func TestReportUsesLocalDayForUTCTimestamps(t *testing.T) {
	kyiv, err := time.LoadLocation("Europe/Kyiv")
	if err != nil {
		t.Skipf("no tzdata: %v", err)
	}
	prevLocal := time.Local
	time.Local = kyiv
	defer func() { time.Local = prevLocal }()

	ctx := context.Background()
	store, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Init(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.SeedAdmins(ctx, []int64{1}); err != nil {
		t.Fatal(err)
	}
	// 22:30 UTC on 1 Oct is 01:30 on 2 Oct in Kyiv.
	if _, err := store.db.ExecContext(ctx, `INSERT INTO sales (user_id, total, cost_total, payment_method, created_at) VALUES (1, 100, 40, 'cash', '2026-10-01 22:30:00')`); err != nil {
		t.Fatal(err)
	}

	oct1, err := store.GetDailyReport(ctx, time.Date(2026, 10, 1, 12, 0, 0, 0, kyiv))
	if err != nil {
		t.Fatal(err)
	}
	oct2, err := store.GetDailyReport(ctx, time.Date(2026, 10, 2, 12, 0, 0, 0, kyiv))
	if err != nil {
		t.Fatal(err)
	}
	if oct1.TotalRevenue != 0 || oct2.TotalRevenue != 100 {
		t.Fatalf("sale must belong to 2 Oct Kyiv time: oct1=%v oct2=%v", oct1.TotalRevenue, oct2.TotalRevenue)
	}

	days, err := store.SalesDays(ctx, time.Date(2026, 10, 1, 0, 0, 0, 0, kyiv), time.Date(2026, 11, 1, 0, 0, 0, 0, kyiv))
	if err != nil {
		t.Fatal(err)
	}
	if !days["2026-10-02"] || days["2026-10-01"] {
		t.Fatalf("unexpected sales days: %v", days)
	}
}

func TestSettings(t *testing.T) {
	ctx := context.Background()
	store, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Init(ctx); err != nil {
		t.Fatal(err)
	}
	if v, err := store.GetSetting(ctx, "k"); err != nil || v != "" {
		t.Fatalf("unset key: %q %v", v, err)
	}
	if err := store.SetSetting(ctx, "k", "1"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetSetting(ctx, "k", "2"); err != nil {
		t.Fatal(err)
	}
	if v, _ := store.GetSetting(ctx, "k"); v != "2" {
		t.Fatalf("got %q", v)
	}
	if err := store.DeleteSetting(ctx, "k"); err != nil {
		t.Fatal(err)
	}
	if v, _ := store.GetSetting(ctx, "k"); v != "" {
		t.Fatalf("got %q after delete", v)
	}
}

func TestDailyReportBreakdown(t *testing.T) {
	ctx := context.Background()
	store, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Init(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.SeedAdmins(ctx, []int64{1, 2}); err != nil {
		t.Fatal(err)
	}
	latte, _ := store.CreateProduct(ctx, models.Product{Name: "Лате", Category: models.CategoryDrinks, CostPrice: 30, SellPrice: 70, Unit: "чашка", Stock: 100, IsActive: true})
	cake, _ := store.CreateProduct(ctx, models.Product{Name: "Чізкейк", Category: models.CategorySweets, CostPrice: 40, SellPrice: 90, Unit: "шт", Stock: 100, IsActive: true})

	mustSell := func(user int64, payment string, items ...models.OrderItem) {
		if _, _, err := store.RecordSale(ctx, user, payment, items); err != nil {
			t.Fatal(err)
		}
	}
	mustSell(1, models.PaymentCash, models.OrderItem{ProductID: latte, Qty: 2})
	mustSell(2, models.PaymentCard, models.OrderItem{ProductID: latte, Qty: 1}, models.OrderItem{ProductID: cake, Qty: 1})
	// A later price change must not rewrite today's numbers.
	if err := store.UpdateProductSellPrice(ctx, latte, 999); err != nil {
		t.Fatal(err)
	}

	report, err := store.GetDailyReport(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if report.Checks != 2 || report.TotalRevenue != 300 || report.CostTotal != 130 || report.CashRevenue != 140 || report.CardRevenue != 160 {
		t.Fatalf("unexpected totals: %+v", report)
	}
	if len(report.Items) != 2 {
		t.Fatalf("unexpected items: %+v", report.Items)
	}
	l := report.Items[0]
	if l.Name != "Лате" || l.Qty != 3 || l.Revenue != 210 || l.Cost != 90 || l.Unit != "чашка" || l.Category != models.CategoryDrinks {
		t.Fatalf("unexpected latte line: %+v", l)
	}
	if len(report.Sellers) != 2 || report.Sellers[0].UserID != 2 || report.Sellers[0].Total != 160 || report.Sellers[1].Checks != 1 {
		t.Fatalf("unexpected sellers: %+v", report.Sellers)
	}

	checks, err := store.ListChecks(ctx, time.Now(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(checks) != 2 || checks[0].UserID != 1 || len(checks[0].Items) != 1 || len(checks[1].Items) != 2 || checks[1].Total != 160 {
		t.Fatalf("unexpected checks: %+v", checks)
	}
	if checks[0].CreatedAt.IsZero() || checks[0].CreatedAt.Location() != time.Local {
		t.Fatalf("check time must be local: %v", checks[0].CreatedAt)
	}
	ownChecks, err := store.ListChecks(ctx, time.Now(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(ownChecks) != 1 || ownChecks[0].UserID != 2 {
		t.Fatalf("unexpected own checks: %+v", ownChecks)
	}

	mine, err := store.GetUserSalesSummary(ctx, 1, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if mine.Checks != 1 || mine.GrandTotal != 140 || len(mine.Items) != 1 || mine.Items[0].Qty != 2 {
		t.Fatalf("per-user summary must include only own sales: %+v", mine)
	}
}
