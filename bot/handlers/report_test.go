package handlers

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/models"
)

func TestBrowseReportsByDay(t *testing.T) {
	h, fake := newTestHandler(t)
	ctx := context.Background()
	latte, err := h.storage.CreateProduct(ctx, models.Product{Name: "Лате", Category: models.CategoryDrinks, CostPrice: 30, SellPrice: 70, Unit: "чашка", Stock: 10, IsActive: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := h.storage.RecordSale(ctx, 1, models.PaymentCard, []models.OrderItem{{ProductID: latte, Qty: 1}}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	today := now.Format("2006-01-02")
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")

	// "Мої продажі": today's checks with time, and a button to the previous day.
	sendTextUpdate(t, h, "Мої продажі за сьогодні")
	mine := fake.last(t)
	if !strings.Contains(mine.Text, "за сьогодні") || !strings.Contains(mine.Text, "🕐 ") || !strings.Contains(mine.Text, "Лате ×1") || !strings.Contains(mine.Markup, "my:day:"+yesterday) {
		t.Fatalf("unexpected my sales: %+v", mine)
	}
	sendCallbackUpdate(t, h, "my:day:"+yesterday)
	prev := fake.last(t)
	if prev.Method != "editMessageText" || !strings.Contains(prev.Text, "за вчора") || !strings.Contains(prev.Text, "Продажів за цей день немає") || !strings.Contains(prev.Markup, "my:day:"+today) {
		t.Fatalf("unexpected previous day: %+v", prev)
	}

	// Daily report: day navigation edits in place, checks button lists checks.
	sendCallbackUpdate(t, h, "report:today")
	if got := fake.last(t).Markup; !strings.Contains(got, "report:nav:"+yesterday) || !strings.Contains(got, "report:checks:"+today) {
		t.Fatalf("report must offer day navigation and checks: %q", got)
	}
	sendCallbackUpdate(t, h, "report:nav:"+yesterday)
	if got := fake.last(t); got.Method != "editMessageText" || !strings.Contains(got.Text, now.AddDate(0, 0, -1).Format("02.01.2006")) {
		t.Fatalf("unexpected navigated report: %+v", got)
	}
	sendCallbackUpdate(t, h, "report:checks:"+today)
	if got := fake.last(t).Text; !strings.Contains(got, "🧾 Чеки за "+now.Format("02.01.2006")) || !strings.Contains(got, "💳 70.00 грн") || !strings.Contains(got, "Лате ×1") {
		t.Fatalf("unexpected checks list: %q", got)
	}
}

func TestMonthlyReportFlow(t *testing.T) {
	h, fake := newTestHandler(t)
	ctx := context.Background()
	latte, err := h.storage.CreateProduct(ctx, models.Product{Name: "Лате", Category: models.CategoryDrinks, CostPrice: 30, SellPrice: 70, Unit: "чашка", Stock: 10, IsActive: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := h.storage.RecordSale(ctx, 1, models.PaymentCard, []models.OrderItem{{ProductID: latte, Qty: 2}}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	thisMonth := now.Format("2006-01")
	lastMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local).AddDate(0, -1, 0).Format("2006-01")

	sendTextUpdate(t, h, "Відправити звіт")
	if got := fake.last(t).Markup; !strings.Contains(got, "report:month:"+thisMonth) || !strings.Contains(got, "report:month:"+lastMonth) {
		t.Fatalf("report menu must offer this and last month: %q", got)
	}

	sendCallbackUpdate(t, h, "report:month:"+thisMonth)
	month := fake.last(t)
	if month.Method != "sendMessage" || !strings.Contains(month.Text, "📆 Звіт за") || !strings.Contains(month.Text, "(поточний") ||
		!strings.Contains(month.Text, "Виручка: 140.00 грн") || !strings.Contains(month.Text, "Днів з продажами: 1") || !strings.Contains(month.Text, "📅 По днях") {
		t.Fatalf("unexpected monthly report: %+v", month)
	}
	if !strings.Contains(month.Markup, "report:mnav:"+lastMonth) || strings.Contains(month.Markup, "▶️") {
		t.Fatalf("current month: back only, no forward: %q", month.Markup)
	}

	sendCallbackUpdate(t, h, "report:mnav:"+lastMonth)
	prev := fake.last(t)
	if prev.Method != "editMessageText" || !strings.Contains(prev.Text, "Продажів за цей місяць немає") || !strings.Contains(prev.Markup, "report:mnav:"+thisMonth) {
		t.Fatalf("unexpected previous month: %+v", prev)
	}

	before := len(fake.all())
	sendCallbackUpdate(t, h, "report:calnew:"+thisMonth)
	if got := fake.all()[before:]; len(got) != 1 || got[0].Method != "sendMessage" || !strings.Contains(got[0].Markup, "report:day:") {
		t.Fatalf("day picker must open as a new message: %+v", got)
	}
}
