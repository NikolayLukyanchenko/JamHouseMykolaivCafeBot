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

func TestFormatDailyReportVoided(t *testing.T) {
	report := models.DailyReport{Date: time.Date(2026, 10, 25, 0, 0, 0, 0, time.Local), TotalRevenue: 50, Checks: 1, VoidedChecks: 2, VoidedTotal: 120}
	if text := FormatDailyReport(report); !strings.Contains(text, "↩️ Скасовано чеків: 2 на 120.00 грн") {
		t.Fatalf("voided checks missing:\n%s", text)
	}
	empty := FormatDailyReport(models.DailyReport{Date: report.Date, VoidedChecks: 1, VoidedTotal: 70})
	if !strings.Contains(empty, "Продажів за цей день немає") || !strings.Contains(empty, "Скасовано чеків: 1") {
		t.Fatalf("voided checks missing on a day without sales:\n%s", empty)
	}
}

func TestFormatMonthlyReport(t *testing.T) {
	nov := time.Date(2026, 11, 1, 0, 0, 0, 0, time.Local)
	r := models.MonthlyReport{
		Month: nov,
		Summary: models.DailyReport{
			Date: nov, CashRevenue: 140, CardRevenue: 280, TotalRevenue: 420, CostTotal: 180, Profit: 240, Checks: 3, VoidedChecks: 1, VoidedTotal: 70,
			Items:   []models.DailyReportItem{{Name: "Лате", Category: models.CategoryDrinks, Unit: "чашка", Qty: 6, Revenue: 420, Cost: 180}},
			Sellers: []models.SellerSummary{{UserID: 1, Name: "Ольга", Checks: 3, Cash: 140, Card: 280, Total: 420}},
		},
		Days: []models.DayTotal{
			{Date: time.Date(2026, 11, 1, 0, 0, 0, 0, time.Local), Revenue: 210, Checks: 2},
			{Date: time.Date(2026, 11, 15, 0, 0, 0, 0, time.Local), Revenue: 210, Checks: 1},
		},
		PurchaseTotal: 310,
		ReceiptsSince: time.Date(2026, 11, 5, 9, 0, 0, 0, time.Local),
	}
	text := FormatMonthlyReport(r, time.Date(2026, 11, 20, 12, 0, 0, 0, time.Local))
	t.Log("\n" + text)
	for _, want := range []string{
		"📆 Звіт за Листопад 2026 (поточний, по 20.11)",
		"Виручка: 420.00 грн", "Прибуток: 240.00 грн (маржа 57%)", "Чеків: 3, середній чек: 140.00 грн",
		"Скасовано чеків: 1", "Днів з продажами: 2, середня виручка за день: 210.00 грн",
		"Закупки: 310.00 грн (журнал приходів ведеться з 05.11.2026)",
		"• Лате — 6 чашка · 420.00 грн", "• Ольга — 420.00 грн, чеків: 3",
		"• 01.11 Нд — 210.00 грн · чеків: 2", "• 15.11 Нд — 210.00 грн · чеків: 1",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("monthly report does not contain %q:\n%s", want, text)
		}
	}

	oct := FormatMonthlyReport(models.MonthlyReport{Month: time.Date(2026, 10, 1, 0, 0, 0, 0, time.Local), Summary: models.DailyReport{Date: time.Date(2026, 10, 1, 0, 0, 0, 0, time.Local)}, ReceiptsSince: r.ReceiptsSince}, time.Date(2026, 11, 20, 0, 0, 0, 0, time.Local))
	if !strings.Contains(oct, "📆 Звіт за Жовтень 2026\n") || !strings.Contains(oct, "Продажів за цей місяць немає") || !strings.Contains(oct, "журнал приходів за цей місяць ще не вівся") {
		t.Fatalf("unexpected empty past month:\n%s", oct)
	}
}
