package handlers

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/models"
)

func TestPOSOneTapSaleAndVoid(t *testing.T) {
	h, fake := newTestHandler(t)
	ctx := context.Background()
	mk := func(name, category string, price, stock float64) int64 {
		id, err := h.storage.CreateProduct(ctx, models.Product{Name: name, Category: category, CostPrice: price / 2, SellPrice: price, Unit: "шт", Stock: stock, IsActive: true})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	latte := mk("Лате", models.CategoryDrinks, 70, 2)
	croissant := mk("Круасан", models.CategoryFood, 55, 1)
	add := func(id int64) { sendCallbackUpdate(t, h, fmt.Sprintf("pos:add:%d", id)) }

	// The menu button opens the screen on the "Топ" tab, no category step.
	sendTextUpdate(t, h, "🧾 Продаж")
	screen := fake.last(t)
	if !strings.Contains(screen.Text, "Натискайте на товари") || !strings.Contains(screen.Markup, fmt.Sprintf("pos:add:%d", latte)) || !strings.Contains(screen.Markup, "▸⭐ Топ") {
		t.Fatalf("unexpected checkout screen: %+v", screen)
	}

	// Each tap adds one; no quantity typing; the same message is updated.
	add(latte)
	add(latte)
	screen = fake.last(t)
	if screen.Method != "editMessageText" || !strings.Contains(screen.Text, "Лате ×2 — 140.00 грн") || !strings.Contains(screen.Markup, "Лате 70 ·2") || !strings.Contains(screen.Markup, "💵 Готівка 140") {
		t.Fatalf("unexpected screen after two taps: %+v", screen)
	}
	if got := fake.lastAnswer(); got != "+ Лате" {
		t.Fatalf("unexpected toast %q", got)
	}
	add(latte)
	if got := fake.lastAnswer(); !strings.Contains(got, "Залишок «Лате» лише 2") {
		t.Fatalf("stock limit toast expected, got %q", got)
	}

	// Category tab switches in place.
	sendCallbackUpdate(t, h, "pos:tab:1")
	if got := fake.last(t); got.Method != "editMessageText" || !strings.Contains(got.Markup, "Круасан 55") || !strings.Contains(got.Markup, "▸🍽 Їжа") {
		t.Fatalf("unexpected food tab: %+v", got)
	}
	add(croissant)
	sendCallbackUpdate(t, h, fmt.Sprintf("pos:sub:%d", latte))
	if got := fake.last(t).Text; !strings.Contains(got, "Лате ×1") || !strings.Contains(got, "Круасан ×1") || !strings.Contains(got, "Разом: 125.00 грн") {
		t.Fatalf("unexpected order after ➖: %q", got)
	}

	// One tap pays: receipt with undo, and a fresh screen for the next customer.
	before := len(fake.all())
	sendCallbackUpdate(t, h, "pos:pay:card")
	after := fake.all()[before:]
	if len(after) != 2 || after[0].Method != "editMessageText" || !strings.Contains(after[0].Text, "Чек #1") || !strings.Contains(after[0].Markup, "pos:void:1") ||
		after[1].Method != "sendMessage" || !strings.Contains(after[1].Text, "Натискайте на товари") {
		t.Fatalf("unexpected messages after payment: %+v", after)
	}
	if p, _ := h.storage.GetProduct(ctx, croissant); p.Stock != 0 {
		t.Fatalf("stock not written off: %v", p.Stock)
	}
	sendCallbackUpdate(t, h, "pos:tab:1")
	if got := fake.last(t).Markup; !strings.Contains(got, "⛔ Круасан") {
		t.Fatalf("sold-out product must be marked: %q", got)
	}
	add(croissant)
	if got := fake.lastAnswer(); !strings.Contains(got, "немає в наявності") {
		t.Fatalf("sold-out toast expected, got %q", got)
	}
	report, _ := h.storage.GetDailyReport(ctx, time.Now())
	if report.Checks != 1 || report.TotalRevenue != 125 || report.CardRevenue != 125 {
		t.Fatalf("sale must be in the report: %+v", report)
	}

	// Undo within the window returns stock and removes the sale from totals.
	sendCallbackUpdate(t, h, "pos:void:1")
	if got := fake.last(t); got.Method != "editMessageText" || !strings.Contains(got.Text, "❌ Чек #1 на 125.00 грн скасовано") {
		t.Fatalf("unexpected void result: %+v", got)
	}
	if p, _ := h.storage.GetProduct(ctx, croissant); p.Stock != 1 {
		t.Fatalf("stock not returned: %v", p.Stock)
	}
	report, _ = h.storage.GetDailyReport(ctx, time.Now())
	if report.Checks != 0 || report.VoidedChecks != 1 {
		t.Fatalf("voided sale must leave the totals: %+v", report)
	}
	sendCallbackUpdate(t, h, "pos:void:1")
	if got := fake.lastAnswer(); !strings.HasPrefix(got, "!") || !strings.Contains(got, "вже скасовано") {
		t.Fatalf("second undo must be refused with an alert, got %q", got)
	}

	// Empty order cannot be paid.
	sendCallbackUpdate(t, h, "pos:pay:cash")
	if got := fake.lastAnswer(); got != "Замовлення порожнє" {
		t.Fatalf("unexpected toast %q", got)
	}
}

func TestPOSOldButtonsAndLabelsOpenNewScreen(t *testing.T) {
	h, fake := newTestHandler(t)
	for _, label := range []string{"Записати продаж", "Калькулятор замовлення"} {
		sendTextUpdate(t, h, label)
		if got := fake.last(t).Text; !strings.Contains(got, "🧾 Замовлення") {
			t.Fatalf("%q must open the checkout screen, got %q", label, got)
		}
	}
	sendCallbackUpdate(t, h, "cart:add_more")
	if got := fake.last(t).Text; !strings.Contains(got, "🧾 Замовлення") {
		t.Fatalf("old cart button must open the checkout screen, got %q", got)
	}
}
