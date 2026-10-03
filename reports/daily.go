package reports

import (
	"fmt"
	"strings"
	"time"

	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/models"
	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/utils"
)

const otherCategory = "Інше"

func FormatDailyReport(report models.DailyReport) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("📊 Звіт за %s\n\n", report.Date.Format("02.01.2006")))
	if report.Checks == 0 {
		b.WriteString("Продажів за цей день немає.")
		return b.String()
	}

	b.WriteString(fmt.Sprintf("💰 Виручка: %s\n", utils.FormatMoney(report.TotalRevenue)))
	b.WriteString(fmt.Sprintf("    💵 Готівка: %s\n", utils.FormatMoney(report.CashRevenue)))
	b.WriteString(fmt.Sprintf("    💳 Карта: %s\n", utils.FormatMoney(report.CardRevenue)))
	b.WriteString(fmt.Sprintf("📦 Собівартість: %s\n", utils.FormatMoney(report.CostTotal)))
	b.WriteString(fmt.Sprintf("📈 Прибуток: %s", utils.FormatMoney(report.Profit)))
	if report.TotalRevenue > 0 {
		b.WriteString(fmt.Sprintf(" (маржа %.0f%%)", report.Profit/report.TotalRevenue*100))
	}
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("🧾 Чеків: %d, середній чек: %s\n", report.Checks, utils.FormatMoney(report.TotalRevenue/float64(report.Checks))))

	b.WriteString("\n🛍 Продані товари\n")
	b.WriteString("(кількість · виручка · собівартість · прибуток)\n")
	writeItemsByCategory(&b, report.Items, true)

	if len(report.Sellers) > 0 {
		b.WriteString("\n👥 По продавцях\n")
		for _, seller := range report.Sellers {
			name := seller.Name
			if name == "" {
				name = fmt.Sprintf("ID %d", seller.UserID)
			}
			b.WriteString(fmt.Sprintf("• %s — %s, чеків: %d (💵 %s / 💳 %s)\n", name, utils.FormatMoney(seller.Total), seller.Checks, utils.FormatMoney(seller.Cash), utils.FormatMoney(seller.Card)))
		}
	}
	return strings.TrimSpace(b.String())
}

// DayLabel renders "сьогодні", "вчора" or the date.
func DayLabel(date, now time.Time) string {
	switch date.Format("2006-01-02") {
	case now.Format("2006-01-02"):
		return "сьогодні"
	case now.AddDate(0, 0, -1).Format("2006-01-02"):
		return "вчора"
	default:
		return date.Format("02.01.2006")
	}
}

// FormatUserSales renders "Мої продажі" for one seller and one day.
func FormatUserSales(name string, date, now time.Time, summary models.UserSalesSummary, checks []models.SaleCheck) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("📅 Ваші продажі за %s\n👤 %s\n\n", DayLabel(date, now), name))
	if summary.Checks == 0 {
		b.WriteString("Продажів за цей день немає.")
		return b.String()
	}
	b.WriteString(fmt.Sprintf("💵 Готівка: %s\n", utils.FormatMoney(summary.CashTotal)))
	b.WriteString(fmt.Sprintf("💳 Карта: %s\n", utils.FormatMoney(summary.CardTotal)))
	b.WriteString(fmt.Sprintf("💰 Разом: %s\n", utils.FormatMoney(summary.GrandTotal)))
	b.WriteString(fmt.Sprintf("🧾 Кількість чеків: %d\n", summary.Checks))
	if len(summary.Items) > 0 {
		b.WriteString("\n🛍 Продані товари\n")
		writeItemsByCategory(&b, summary.Items, false)
	}
	if len(checks) > 0 {
		b.WriteString("\n🧾 Чеки\n")
		writeChecks(&b, checks, false)
	}
	b.WriteString("\nℹ️ Тут лише чеки, які провели ви. Підсумок усього кафе — у звіті за день.")
	return strings.TrimSpace(b.String())
}

// FormatChecks renders all checks of a day with time and seller.
func FormatChecks(date time.Time, checks []models.SaleCheck) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("🧾 Чеки за %s\n\n", date.Format("02.01.2006")))
	if len(checks) == 0 {
		b.WriteString("Продажів за цей день немає.")
		return b.String()
	}
	writeChecks(&b, checks, true)
	return strings.TrimSpace(b.String())
}

func writeChecks(b *strings.Builder, checks []models.SaleCheck, withSeller bool) {
	for _, check := range checks {
		payment := "💵"
		if check.PaymentMethod == models.PaymentCard {
			payment = "💳"
		}
		b.WriteString(fmt.Sprintf("🕐 %s · #%d · %s %s", check.CreatedAt.Format("15:04"), check.ID, payment, utils.FormatMoney(check.Total)))
		if withSeller && check.SellerName != "" {
			b.WriteString(" · " + check.SellerName)
		}
		b.WriteString("\n")
		if len(check.Items) > 0 {
			names := make([]string, 0, len(check.Items))
			for _, item := range check.Items {
				names = append(names, fmt.Sprintf("%s ×%s", item.Name, utils.FormatQuantity(item.Qty)))
			}
			b.WriteString("    " + strings.Join(names, ", ") + "\n")
		}
	}
}

func writeItemsByCategory(b *strings.Builder, items []models.DailyReportItem, withCost bool) {
	categories := append(models.Categories(), otherCategory)
	for _, category := range categories {
		var group []models.DailyReportItem
		for _, item := range items {
			itemCategory := item.Category
			if !isKnownCategory(itemCategory) {
				itemCategory = otherCategory
			}
			if itemCategory == category {
				group = append(group, item)
			}
		}
		if len(group) == 0 {
			continue
		}
		b.WriteString(fmt.Sprintf("\n%s %s\n", utils.CategoryEmoji(category), category))
		for _, item := range group {
			qty := utils.FormatQuantity(item.Qty)
			if item.Unit != "" {
				qty += " " + item.Unit
			}
			if withCost {
				b.WriteString(fmt.Sprintf("• %s — %s · %s · %s · %s\n", item.Name, qty, utils.FormatMoney(item.Revenue), utils.FormatMoney(item.Cost), utils.FormatMoney(item.Revenue-item.Cost)))
			} else {
				b.WriteString(fmt.Sprintf("• %s — %s · %s\n", item.Name, qty, utils.FormatMoney(item.Revenue)))
			}
		}
	}
}

func isKnownCategory(category string) bool {
	for _, known := range models.Categories() {
		if category == known {
			return true
		}
	}
	return false
}
