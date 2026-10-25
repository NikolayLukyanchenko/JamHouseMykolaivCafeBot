package handlers

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/models"
)

func TestBundlesAreCheckoutShortcutsOnly(t *testing.T) {
	h, fake := newTestHandler(t)
	ctx := context.Background()
	mk := func(name string, price, stock float64) int64 {
		id, err := h.storage.CreateProduct(ctx, models.Product{Name: name, Category: models.CategoryDrinks, CostPrice: price / 3, SellPrice: price, Unit: "шт", Stock: stock, IsActive: true})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	coffee := mk("Кава", 45, 10)
	cream := mk("Вершки", 10, 1)
	topping := mk("Топінг", 15, 5)

	// Admin composes "Кава + Вершки + Топінг" by tapping products.
	sendCallbackUpdate(t, h, "admin:bundle_new")
	for _, id := range []int64{coffee, cream, topping, topping} {
		sendCallbackUpdate(t, h, fmt.Sprintf("admin:bundle_add:%d", id))
	}
	sendCallbackUpdate(t, h, fmt.Sprintf("admin:bundle_sub:%d", topping))
	if got := fake.last(t); got.Method != "editMessageText" || !strings.Contains(got.Text, "Топінг ×1") || !strings.Contains(got.Text, "Ціна: 70.00 грн") {
		t.Fatalf("unexpected builder: %+v", got)
	}
	sendCallbackUpdate(t, h, "admin:bundle_save")
	if got := fake.last(t).Text; !strings.Contains(got, "🧩 Кава + Вершки + Топінг") || !strings.Contains(got, "Набір збережено") {
		t.Fatalf("unexpected bundle card: %q", got)
	}
	bundles, _ := h.storage.ListBundles(ctx)
	if len(bundles) != 1 {
		t.Fatalf("expected one bundle, got %+v", bundles)
	}
	bundleID := bundles[0].ID

	// Rename from the card.
	sendCallbackUpdate(t, h, fmt.Sprintf("admin:bundle_rename:%d", bundleID))
	sendTextUpdate(t, h, "Кава з усім")
	if got := fake.last(t).Text; !strings.Contains(got, "🧩 Кава з усім") {
		t.Fatalf("rename failed: %q", got)
	}

	// Checkout: the bundle is on the "Топ" tab and one tap adds the products.
	sendTextUpdate(t, h, "🧾 Продаж")
	if got := fake.last(t).Markup; !strings.Contains(got, "🧩 Кава з усім 70") || !strings.Contains(got, fmt.Sprintf("pos:set:%d", bundleID)) {
		t.Fatalf("bundle missing on the checkout screen: %q", got)
	}
	sendCallbackUpdate(t, h, fmt.Sprintf("pos:set:%d", bundleID))
	screen := fake.last(t)
	if !strings.Contains(screen.Text, "Кава ×1") || !strings.Contains(screen.Text, "Вершки ×1") || !strings.Contains(screen.Text, "Топінг ×1") || !strings.Contains(screen.Text, "Разом: 70.00 грн") {
		t.Fatalf("bundle must add its products as separate lines: %q", screen.Text)
	}
	// Only one cream is in stock: a second bundle is refused as a whole.
	sendCallbackUpdate(t, h, fmt.Sprintf("pos:set:%d", bundleID))
	if got := fake.lastAnswer(); !strings.Contains(got, "Залишок «Вершки» лише 1") {
		t.Fatalf("expected stock toast, got %q", got)
	}
	if got := h.sessions.Cart(1); len(got) != 3 || got[0].Qty != 1 {
		t.Fatalf("refused bundle must not change the order: %+v", got)
	}

	sendCallbackUpdate(t, h, "pos:pay:cash")
	report, err := h.storage.GetDailyReport(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if report.Checks != 1 || report.TotalRevenue != 70 || len(report.Items) != 3 {
		t.Fatalf("report must show the individual products: %+v", report)
	}
	for _, item := range report.Items {
		if strings.Contains(item.Name, "Кава з усім") {
			t.Fatal("bundle must not appear in reports")
		}
	}
	if p, _ := h.storage.GetProduct(ctx, cream); p.Stock != 0 {
		t.Fatalf("component stock must be written off: %v", p.Stock)
	}

	// Cream is sold out: the bundle is marked ⛔ in the checkout.
	sendTextUpdate(t, h, "🧾 Продаж")
	if got := fake.last(t).Markup; !strings.Contains(got, "⛔ Кава з усім") {
		t.Fatalf("unavailable bundle must be marked: %q", got)
	}

	// Deleting a bundle keeps products and sales intact.
	sendCallbackUpdate(t, h, fmt.Sprintf("admin:bundle_delok:%d", bundleID))
	if bundles, _ := h.storage.ListBundles(ctx); len(bundles) != 0 {
		t.Fatal("bundle not deleted")
	}
	if p, err := h.storage.GetProduct(ctx, coffee); err != nil || !p.IsActive {
		t.Fatalf("deleting a bundle must not touch products: %+v %v", p, err)
	}
	sendCallbackUpdate(t, h, fmt.Sprintf("pos:set:%d", bundleID))
	if got := fake.lastAnswer(); got != "Набір видалено" {
		t.Fatalf("stale bundle button: unexpected toast %q", got)
	}
}
