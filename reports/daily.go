package reports

import (
	"fmt"
	"strings"

	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/models"
	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/utils"
)

func FormatDailyReport(report models.DailyReport) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("📊 Звіт за %s\n\n", report.Date.Format("02.01.2006")))
	builder.WriteString(fmt.Sprintf("💵 Готівка: %s\n", utils.FormatMoney(report.CashRevenue)))
	builder.WriteString(fmt.Sprintf("💳 Карта: %s\n", utils.FormatMoney(report.CardRevenue)))
	builder.WriteString(fmt.Sprintf("💰 Загальна виручка: %s\n", utils.FormatMoney(report.TotalRevenue)))
	builder.WriteString(fmt.Sprintf("📦 Собівартість: %s\n", utils.FormatMoney(report.CostTotal)))
	builder.WriteString(fmt.Sprintf("📈 Прибуток: %s\n", utils.FormatMoney(report.Profit)))
	builder.WriteString("\n🧾 Продані товари:\n")
	if len(report.Items) == 0 {
		builder.WriteString("— Продажів за цей день немає.")
		return builder.String()
	}
	for _, item := range report.Items {
		builder.WriteString(fmt.Sprintf("• %s — %s\n", item.Name, utils.FormatQuantity(item.Qty)))
	}
	return strings.TrimSpace(builder.String())
}
