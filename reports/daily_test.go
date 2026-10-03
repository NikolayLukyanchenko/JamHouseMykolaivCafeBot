package reports

import (
	"strings"
	"testing"
	"time"

	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/models"
)

func TestFormatDailyReport(t *testing.T) {
	report := models.DailyReport{
		Date:         time.Date(2026, 5, 30, 0, 0, 0, 0, time.Local),
		CashRevenue:  100,
		CardRevenue:  50,
		TotalRevenue: 150,
		CostTotal:    60,
		Profit:       90,
		Checks:       3,
		Items: []models.DailyReportItem{
			{Name: "Лате", Category: models.CategoryDrinks, Unit: "чашка", Qty: 2, Revenue: 140, Cost: 60},
			{Name: "Старий товар", Qty: 1, Revenue: 10},
		},
		Sellers: []models.SellerSummary{{UserID: 1, Name: "Ольга", Checks: 3, Cash: 100, Card: 50, Total: 150}},
	}
	text := FormatDailyReport(report)
	t.Log("\n" + text)
	for _, want := range []string{
		"Звіт за 30.05.2026",
		"Виручка: 150.00 грн",
		"Готівка: 100.00 грн",
		"Прибуток: 90.00 грн (маржа 60%)",
		"Чеків: 3, середній чек: 50.00 грн",
		"🥤 Напої",
		"• Лате — 2 чашка · 140.00 грн · 60.00 грн · 80.00 грн",
		"• Інше",
		"• Старий товар — 1 · 10.00 грн",
		"• Ольга — 150.00 грн, чеків: 3",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("report text does not contain %q:\n%s", want, text)
		}
	}
}

func TestFormatDailyReportEmpty(t *testing.T) {
	text := FormatDailyReport(models.DailyReport{Date: time.Date(2026, 5, 30, 0, 0, 0, 0, time.Local)})
	if !strings.Contains(text, "Продажів за цей день немає") {
		t.Fatalf("unexpected empty report: %q", text)
	}
}

func TestFormatUserSales(t *testing.T) {
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.Local)
	checks := []models.SaleCheck{
		{ID: 7, Total: 45, PaymentMethod: models.PaymentCard, CreatedAt: time.Date(2026, 10, 3, 9, 5, 0, 0, time.Local), Items: []models.SaleItem{{Name: "Лате", Qty: 1}}},
		{ID: 9, Total: 30, PaymentMethod: models.PaymentCard, CreatedAt: time.Date(2026, 10, 3, 14, 40, 0, 0, time.Local), Items: []models.SaleItem{{Name: "Еспресо", Qty: 1}}},
	}
	text := FormatUserSales("Ольга", now, now, models.UserSalesSummary{
		CardTotal: 75, GrandTotal: 75, Checks: 2,
		Items: []models.DailyReportItem{{Name: "Лате", Category: models.CategoryDrinks, Unit: "чашка", Qty: 1, Revenue: 45, Cost: 30}},
	}, checks)
	t.Log("\n" + text)
	for _, want := range []string{"Ваші продажі за сьогодні", "👤 Ольга", "Кількість чеків: 2", "• Лате — 1 чашка · 45.00 грн", "🕐 09:05 · #7 · 💳 45.00 грн", "Еспресо ×1", "лише чеки, які провели ви"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text does not contain %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "30.00 грн ·") {
		t.Fatal("seller summary must not show cost")
	}

	empty := FormatUserSales("Ольга", now.AddDate(0, 0, -1), now, models.UserSalesSummary{}, nil)
	if !strings.Contains(empty, "за вчора") || !strings.Contains(empty, "Продажів за цей день немає") {
		t.Fatalf("unexpected empty day: %q", empty)
	}
}

func TestFormatChecks(t *testing.T) {
	text := FormatChecks(time.Date(2026, 10, 2, 0, 0, 0, 0, time.Local), []models.SaleCheck{
		{ID: 3, SellerName: "Ольга", Total: 90, PaymentMethod: models.PaymentCash, CreatedAt: time.Date(2026, 10, 2, 18, 30, 0, 0, time.Local), Items: []models.SaleItem{{Name: "Чізкейк", Qty: 1}}},
	})
	if !strings.Contains(text, "🧾 Чеки за 02.10.2026") || !strings.Contains(text, "🕐 18:30 · #3 · 💵 90.00 грн · Ольга") || !strings.Contains(text, "Чізкейк ×1") {
		t.Fatalf("unexpected checks text:\n%s", text)
	}
}
