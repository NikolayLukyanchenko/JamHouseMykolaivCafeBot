package handlers

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/models"
)

func TestHideAndRestoreProduct(t *testing.T) {
	h, fake := newTestHandler(t)
	ctx := context.Background()
	id, err := h.storage.CreateProduct(ctx, models.Product{Name: "Какао", Category: models.CategoryDrinks, CostPrice: 20, SellPrice: 50, Unit: "чашка", Stock: 5, IsActive: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := h.storage.RecordSale(ctx, 1, models.PaymentCash, []models.OrderItem{{ProductID: id, Qty: 1}}); err != nil {
		t.Fatal(err)
	}

	sendCallbackUpdate(t, h, fmt.Sprintf("admin:edit_select:%d", id))
	if got := fake.last(t).Markup; !strings.Contains(got, fmt.Sprintf("admin:action:hide:%d", id)) {
		t.Fatalf("card must offer removing from menu: %q", got)
	}
	sendCallbackUpdate(t, h, fmt.Sprintf("admin:action:hide:%d", id))
	if got := fake.last(t); !strings.Contains(got.Text, "Прибрати «Какао» з меню?") || !strings.Contains(got.Markup, fmt.Sprintf("admin:action:hideok:%d", id)) {
		t.Fatalf("removal must be confirmed: %+v", got)
	}
	sendCallbackUpdate(t, h, fmt.Sprintf("admin:action:hideok:%d", id))
	if got := fake.last(t); !strings.Contains(got.Text, "Товар прибрано з меню") || !strings.Contains(got.Markup, fmt.Sprintf("admin:action:show:%d", id)) {
		t.Fatalf("unexpected card after removal: %+v", got)
	}

	sendTextUpdate(t, h, "Меню для клієнтів")
	if got := fake.last(t).Text; strings.Contains(got, "Какао") {
		t.Fatalf("hidden product is still in the customer menu: %q", got)
	}
	sendCallbackUpdate(t, h, "cart:category:"+models.CategoryDrinks)
	if got := fake.last(t).Markup; strings.Contains(got, "Какао") {
		t.Fatalf("hidden product can still be sold: %q", got)
	}
	report, err := h.storage.GetDailyReport(ctx, time.Now())
	if err != nil || len(report.Items) != 1 || report.Items[0].Name != "Какао" {
		t.Fatalf("past sales must stay in reports: %+v %v", report.Items, err)
	}

	sendCallbackUpdate(t, h, fmt.Sprintf("admin:action:show:%d", id))
	sendTextUpdate(t, h, "Меню для клієнтів")
	if got := fake.last(t).Text; !strings.Contains(got, "Какао") {
		t.Fatalf("restored product must be back in the menu: %q", got)
	}
}
