package storage

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"os"
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

func TestBackupProducesImportableCopy(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := New(filepath.Join(dir, "data.db"))
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
	id, err := store.CreateProduct(ctx, models.Product{Name: "Лате", Category: models.CategoryDrinks, CostPrice: 30, SellPrice: 70, Unit: "чашка", Stock: 10, IsActive: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.RecordSale(ctx, 1, models.PaymentCash, []models.OrderItem{{ProductID: id, Qty: 2}}); err != nil {
		t.Fatal(err)
	}
	want, err := store.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(dir, "bot-jam-house-cafe.db")
	if err := store.Backup(ctx, dest); err != nil {
		t.Fatalf("Backup() error: %v", err)
	}
	header := make([]byte, 20)
	f, err := os.Open(dest)
	if err != nil {
		t.Fatal(err)
	}
	_, err = io.ReadFull(f, header)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	// Bytes 18-19 of the SQLite header are 2 for WAL mode.
	if header[18] != 2 || header[19] != 2 {
		t.Fatalf("backup is not in WAL mode: %v", header[18:20])
	}
	if _, err := os.Stat(dest + "-wal"); err == nil {
		if fi, _ := os.Stat(dest + "-wal"); fi.Size() > 0 {
			t.Fatal("WAL must be checkpointed into the main file")
		}
	}

	copyStore, err := New(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer copyStore.Close()
	got, err := copyStore.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got != want || got.Sales != 1 || got.Products != 1 {
		t.Fatalf("backup stats %v, want %v", got, want)
	}
	if err := store.Backup(ctx, dest); err == nil {
		t.Fatal("backup must not overwrite an existing file")
	}
}

func TestTopProductsAndVoidSale(t *testing.T) {
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
	mk := func(name string) int64 {
		id, err := store.CreateProduct(ctx, models.Product{Name: name, Category: models.CategoryDrinks, CostPrice: 10, SellPrice: 50, Unit: "шт", Stock: 20, IsActive: true})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	americano, latte, tea := mk("Американо"), mk("Лате"), mk("Чай")
	hidden := mk("Сезонний")
	if err := store.SetProductActive(ctx, hidden, false); err != nil {
		t.Fatal(err)
	}

	sale, _, err := store.RecordSale(ctx, 1, models.PaymentCard, []models.OrderItem{{ProductID: latte, Qty: 3}, {ProductID: tea, Qty: 1}})
	if err != nil {
		t.Fatal(err)
	}
	top, err := store.TopProducts(ctx, time.Now().AddDate(0, 0, -14), 9)
	if err != nil {
		t.Fatal(err)
	}
	if len(top) != 3 || top[0].ID != latte || top[1].ID != tea || top[2].ID != americano {
		t.Fatalf("unexpected top order: %+v", top)
	}

	now := time.Now()
	if _, err := store.VoidSale(ctx, sale.ID, 2, false, 3*time.Minute, now); !errors.Is(err, ErrVoidForbidden) {
		t.Fatalf("another seller must not void: %v", err)
	}
	if _, err := store.VoidSale(ctx, sale.ID, 1, false, 3*time.Minute, now.Add(10*time.Minute)); !errors.Is(err, ErrVoidTooLate) {
		t.Fatalf("void after the window must fail: %v", err)
	}
	voided, err := store.VoidSale(ctx, sale.ID, 1, false, 3*time.Minute, now)
	if err != nil {
		t.Fatalf("VoidSale: %v", err)
	}
	if voided.Total != 200 {
		t.Fatalf("unexpected voided sale: %+v", voided)
	}
	if p, _ := store.GetProduct(ctx, latte); p.Stock != 20 {
		t.Fatalf("stock must be returned, got %v", p.Stock)
	}
	report, err := store.GetDailyReport(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if report.Checks != 0 || report.TotalRevenue != 0 || len(report.Items) != 0 || report.VoidedChecks != 1 || report.VoidedTotal != 200 {
		t.Fatalf("voided sale must leave totals and be counted separately: %+v", report)
	}
	if _, err := store.VoidSale(ctx, sale.ID, 1, true, 0, now); !errors.Is(err, ErrSaleNotFound) {
		t.Fatalf("second void must report not found: %v", err)
	}

	old, _, err := store.RecordSale(ctx, 2, models.PaymentCash, []models.OrderItem{{ProductID: americano, Qty: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.VoidSale(ctx, old.ID, 1, true, 3*time.Minute, now.Add(time.Hour)); err != nil {
		t.Fatalf("admin may void any check at any time: %v", err)
	}
}

func TestBundlesCRUD(t *testing.T) {
	ctx := context.Background()
	store, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Init(ctx); err != nil {
		t.Fatal(err)
	}
	coffee, _ := store.CreateProduct(ctx, models.Product{Name: "Кава", Category: models.CategoryDrinks, CostPrice: 15, SellPrice: 45, Unit: "чашка", Stock: 10, IsActive: true})
	cream, _ := store.CreateProduct(ctx, models.Product{Name: "Вершки", Category: models.CategoryDrinks, CostPrice: 5, SellPrice: 10, Unit: "порція", Stock: 10, IsActive: true})
	topping, _ := store.CreateProduct(ctx, models.Product{Name: "Топінг", Category: models.CategoryDrinks, CostPrice: 5, SellPrice: 15, Unit: "порція", Stock: 10, IsActive: true})

	id, err := store.SaveBundle(ctx, 0, "Кава + Вершки", map[int64]int{coffee: 1, cream: 1})
	if err != nil {
		t.Fatalf("SaveBundle: %v", err)
	}
	if _, err := store.SaveBundle(ctx, 0, "Кава + Вершки + Топінг", map[int64]int{coffee: 1, cream: 1, topping: 1}); err != nil {
		t.Fatal(err)
	}
	bundles, err := store.ListBundles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(bundles) != 2 || bundles[0].Name != "Кава + Вершки" || len(bundles[0].Items) != 2 || bundles[0].Price() != 55 || len(bundles[1].Items) != 3 || bundles[1].Price() != 70 {
		t.Fatalf("unexpected bundles: %+v", bundles)
	}

	if _, err := store.SaveBundle(ctx, id, "Подвійна кава з вершками", map[int64]int{coffee: 2, cream: 1}); err != nil {
		t.Fatal(err)
	}
	b, err := store.GetBundle(ctx, id)
	if err != nil || b.Name != "Подвійна кава з вершками" || b.Price() != 100 {
		t.Fatalf("unexpected bundle after update: %+v %v", b, err)
	}
	if err := store.RenameBundle(ctx, id, "Кава x2 + вершки"); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteBundle(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetBundle(ctx, id); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deleted bundle must be gone: %v", err)
	}
	if bundles, _ := store.ListBundles(ctx); len(bundles) != 1 {
		t.Fatalf("expected 1 bundle left, got %d", len(bundles))
	}
}

func TestWeightedCost(t *testing.T) {
	cases := []struct{ stock, cost, qty, unit, want float64 }{
		{2, 25, 3, 27, 26.2}, // the cola example
		{0, 25, 3, 27, 27},   // nothing left: purchase price becomes the cost
		{-1, 25, 3, 27, 27},
		{5, 26.2, 5, 26.2, 26.2},
		{3, 10, 1, 11, 10.25},
		{1, 10, 2, 10.01, 10.01}, // (10 + 20.02) / 3 = 10.0066 -> 10.01
	}
	for _, c := range cases {
		if got := WeightedCost(c.stock, c.cost, c.qty, c.unit); got != c.want {
			t.Errorf("WeightedCost(%v, %v, %v, %v) = %v, want %v", c.stock, c.cost, c.qty, c.unit, got, c.want)
		}
	}
}

func TestReceiveStockKeepsPastSalesCost(t *testing.T) {
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
	cola, _ := store.CreateProduct(ctx, models.Product{Name: "Кола", Category: models.CategoryDrinks, CostPrice: 25, SellPrice: 29, Unit: "банка", Stock: 3, IsActive: true})
	if _, _, err := store.RecordSale(ctx, 1, models.PaymentCash, []models.OrderItem{{ProductID: cola, Qty: 1}}); err != nil {
		t.Fatal(err)
	}

	r, err := store.ReceiveStock(ctx, cola, 3, 27, 1)
	if err != nil {
		t.Fatalf("ReceiveStock: %v", err)
	}
	if r.StockBefore != 2 || r.CostBefore != 25 || r.CostAfter != 26.2 || r.ProductName != "Кола" || r.Unit != "банка" {
		t.Fatalf("unexpected receipt: %+v", r)
	}
	p, _ := store.GetProduct(ctx, cola)
	if p.Stock != 5 || p.CostPrice != 26.2 || p.SellPrice != 29 {
		t.Fatalf("unexpected product after receipt: %+v", p)
	}

	if _, _, err := store.RecordSale(ctx, 1, models.PaymentCash, []models.OrderItem{{ProductID: cola, Qty: 1}}); err != nil {
		t.Fatal(err)
	}
	report, err := store.GetDailyReport(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	// One can at the old cost 25, one at the new average 26.20.
	if report.CostTotal != 51.2 || report.TotalRevenue != 58 {
		t.Fatalf("unexpected report: cost %v revenue %v", report.CostTotal, report.TotalRevenue)
	}

	receipts, err := store.ListReceipts(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(receipts) != 1 || receipts[0].UserName == "" || receipts[0].CreatedAt.IsZero() || receipts[0].Qty != 3 || receipts[0].UnitCost != 27 {
		t.Fatalf("unexpected receipt log: %+v", receipts)
	}
	if other, _ := store.ListReceipts(ctx, time.Now().AddDate(0, 0, -1)); len(other) != 0 {
		t.Fatalf("yesterday must be empty: %+v", other)
	}
}

func TestMonthlyReport(t *testing.T) {
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
	if err := store.SeedAdmins(ctx, []int64{1, 2}); err != nil {
		t.Fatal(err)
	}
	latte, _ := store.CreateProduct(ctx, models.Product{Name: "Лате", Category: models.CategoryDrinks, CostPrice: 30, SellPrice: 70, Unit: "чашка", Stock: 100, IsActive: true})

	// created_at is UTC; Kyiv is UTC+2 in November and UTC+3 in October.
	sale := func(user int64, payment, createdAt string, qty float64) {
		res, err := store.db.ExecContext(ctx, `INSERT INTO sales (user_id, total, cost_total, payment_method, created_at) VALUES (?, ?, ?, ?, ?)`, user, 70*qty, 30*qty, payment, createdAt)
		if err != nil {
			t.Fatal(err)
		}
		id, _ := res.LastInsertId()
		if _, err := store.db.ExecContext(ctx, `INSERT INTO sale_items (sale_id, product_id, name, qty, sell_price, cost_price) VALUES (?, ?, 'Лате', ?, 70, 30)`, id, latte, qty); err != nil {
			t.Fatal(err)
		}
	}
	sale(1, models.PaymentCash, "2026-10-31 21:30:00", 1) // 31.10 23:30 Kyiv -> October
	sale(1, models.PaymentCash, "2026-10-31 22:30:00", 2) // 01.11 00:30 Kyiv -> November
	sale(2, models.PaymentCard, "2026-11-01 10:00:00", 1)
	sale(1, models.PaymentCard, "2026-11-15 12:00:00", 3)
	sale(2, models.PaymentCash, "2026-11-30 22:30:00", 1) // 01.12 00:30 Kyiv -> December
	if _, err := store.db.ExecContext(ctx, `INSERT INTO stock_receipts (product_id, product_name, unit, qty, unit_cost, stock_before, cost_before, cost_after, user_id, created_at)
VALUES (?, 'Лате', 'чашка', 10, 31, 0, 30, 31, 1, '2026-11-05 09:00:00')`, latte); err != nil {
		t.Fatal(err)
	}

	r, err := store.GetMonthlyReport(ctx, time.Date(2026, 11, 20, 12, 0, 0, 0, kyiv))
	if err != nil {
		t.Fatalf("GetMonthlyReport: %v", err)
	}
	s := r.Summary
	if !r.Month.Equal(time.Date(2026, 11, 1, 0, 0, 0, 0, kyiv)) || s.Checks != 3 || s.TotalRevenue != 420 || s.CashRevenue != 140 || s.CardRevenue != 280 || s.CostTotal != 180 {
		t.Fatalf("unexpected totals: %+v", s)
	}
	if len(s.Items) != 1 || s.Items[0].Qty != 6 || len(s.Sellers) != 2 {
		t.Fatalf("unexpected items/sellers: %+v %+v", s.Items, s.Sellers)
	}
	if len(r.Days) != 2 || r.Days[0].Date.Day() != 1 || r.Days[0].Checks != 2 || r.Days[0].Revenue != 210 || r.Days[1].Date.Day() != 15 || r.Days[1].Revenue != 210 {
		t.Fatalf("unexpected days: %+v", r.Days)
	}
	if r.PurchaseTotal != 310 || r.ReceiptsSince.IsZero() {
		t.Fatalf("unexpected purchases: %v since %v", r.PurchaseTotal, r.ReceiptsSince)
	}

	oct, err := store.GetMonthlyReport(ctx, time.Date(2026, 10, 1, 0, 0, 0, 0, kyiv))
	if err != nil {
		t.Fatal(err)
	}
	if oct.Summary.Checks != 1 || len(oct.Days) != 1 || oct.Days[0].Date.Day() != 31 || oct.PurchaseTotal != 0 {
		t.Fatalf("unexpected October: %+v", oct)
	}
}
