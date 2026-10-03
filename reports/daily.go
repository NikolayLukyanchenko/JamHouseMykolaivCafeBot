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

// FormatUserSales renders "Мої продажі за сьогодні" for one seller.
func FormatUserSales(name string, date time.Time, summary models.UserSalesSummary) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("📅 Ваші продажі за %s\n👤 %s\n\n", date.Format("02.01.2006"), name))
	b.WriteString(fmt.Sprintf("💵 Готівка: %s\n", utils.FormatMoney(summary.CashTotal)))
	b.WriteString(fmt.Sprintf("💳 Карта: %s\n", utils.FormatMoney(summary.CardTotal)))
	b.WriteString(fmt.Sprintf("💰 Разом: %s\n", utils.FormatMoney(summary.GrandTotal)))
	b.WriteString(fmt.Sprintf("🧾 Кількість чеків: %d\n", summary.Checks))
	if len(summary.Items) > 0 {
		b.WriteString("\n🛍 Продані товари\n")
		writeItemsByCategory(&b, summary.Items, false)
	}
	b.WriteString("\nℹ️ Тут лише чеки, які провели ви. Підсумок усього кафе — у звіті за день.")
	return strings.TrimSpace(b.String())
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
