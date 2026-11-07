package handlers

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/models"
)

func TestReplenishWithWeightedCostAndReceiptLog(t *testing.T) {
	h, fake := newTestHandler(t)
	ctx := context.Background()
	cola, err := h.storage.CreateProduct(ctx, models.Product{Name: "Кола", Category: models.CategoryDrinks, CostPrice: 25, SellPrice: 29, Unit: "банка", Stock: 2, IsActive: true})
	if err != nil {
		t.Fatal(err)
	}

	// 📦 Поповнити залишки → Кола → 3 → price 27.
	sendCallbackUpdate(t, h, fmt.Sprintf("admin:replenish_select:%d", cola))
	sendTextUpdate(t, h, "3")
	if got := fake.last(t); !strings.Contains(got.Text, "Ціна закупки «Кола» за 1 банка?") || !strings.Contains(got.Markup, "Ціна та сама (25.00)") || !strings.Contains(got.Markup, "nav:cancel") {
		t.Fatalf("unexpected price prompt: %+v", got)
	}
	sendTextUpdate(t, h, "abc")
	if got := fake.last(t).Text; !strings.Contains(got, "Введіть ціну закупки") {
		t.Fatalf("invalid price must be asked again: %q", got)
	}
	sendTextUpdate(t, h, "27")
	card := fake.last(t).Text
	for _, want := range []string{"Залишок поповнено: 2 → 5 банка", "Собівартість: 25.00 грн → 26.20 грн", "(2 × 25.00 + 3 × 27.00) / 5", "Собівартість: 26.20 грн", "Ціна продажу: 29.00 грн"} {
		if !strings.Contains(card, want) {
			t.Fatalf("card does not contain %q:\n%s", want, card)
		}
	}

	// Same price: one tap, the cost stays.
	sendCallbackUpdate(t, h, fmt.Sprintf("admin:replenish_select:%d", cola))
	sendTextUpdate(t, h, "5")
	sendCallbackUpdate(t, h, "admin:rcv_same")
	if p, _ := h.storage.GetProduct(ctx, cola); p.Stock != 10 || p.CostPrice != 26.2 {
		t.Fatalf("unexpected product after same-price receipt: %+v", p)
	}
	// A stale "same price" button does nothing harmful.
	sendCallbackUpdate(t, h, "admin:rcv_same")
	if p, _ := h.storage.GetProduct(ctx, cola); p.Stock != 10 {
		t.Fatalf("stale button must not add stock: %v", p.Stock)
	}

	// Cancel at the price step changes nothing.
	sendCallbackUpdate(t, h, fmt.Sprintf("admin:replenish_select:%d", cola))
	sendTextUpdate(t, h, "4")
	sendCallbackUpdate(t, h, "nav:cancel")
	if p, _ := h.storage.GetProduct(ctx, cola); p.Stock != 10 {
		t.Fatalf("cancelled receipt must not change stock: %v", p.Stock)
	}

	// Receipt log: today's two receipts, total spent, day navigation.
	sendCallbackUpdate(t, h, "admin:rcpt")
	log := fake.last(t)
	for _, want := range []string{"📥 Журнал приходів за сьогодні", "Кола +3 банка по 27.00 грн", "собівартість 25.00 грн → 26.20 грн", "Кола +5 банка по 26.20 грн", "Закуплено на: 212.00 грн"} {
		if !strings.Contains(log.Text, want) {
			t.Fatalf("log does not contain %q:\n%s", want, log.Text)
		}
	}
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	if !strings.Contains(log.Markup, "admin:rcpt:"+yesterday) {
		t.Fatalf("log must link to the previous day: %q", log.Markup)
	}
	sendCallbackUpdate(t, h, "admin:rcpt:"+yesterday)
	if got := fake.last(t); got.Method != "editMessageText" || !strings.Contains(got.Text, "за вчора") || !strings.Contains(got.Text, "Приходів за цей день немає") {
		t.Fatalf("unexpected previous day: %+v", got)
	}
}
