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
	text := FormatUserSales("Ольга", time.Date(2026, 10, 3, 12, 0, 0, 0, time.Local), models.UserSalesSummary{
		CardTotal: 75, GrandTotal: 75, Checks: 2,
		Items: []models.DailyReportItem{{Name: "Лате", Category: models.CategoryDrinks, Unit: "чашка", Qty: 1, Revenue: 75, Cost: 30}},
	})
	t.Log("\n" + text)
	for _, want := range []string{"Ваші продажі за 03.10.2026", "👤 Ольга", "Кількість чеків: 2", "• Лате — 1 чашка · 75.00 грн", "лише чеки, які провели ви"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text does not contain %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "30.00") {
		t.Fatal("seller summary must not show cost")
	}
}
