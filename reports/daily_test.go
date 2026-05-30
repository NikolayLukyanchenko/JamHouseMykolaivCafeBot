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
		Items:        []models.DailyReportItem{{Name: "Лате", Qty: 2}},
	}
	text := FormatDailyReport(report)
	for _, want := range []string{"Звіт за 30.05.2026", "Готівка: 100.00 грн", "Лате — 2"} {
		if !strings.Contains(text, want) {
			t.Fatalf("report text %q does not contain %q", text, want)
		}
	}
}
