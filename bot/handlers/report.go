package handlers

import (
	"context"
	"strings"
	"time"

	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/bot/keyboards"
	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/reports"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (h *Handler) handleReportCallback(ctx context.Context, chatID int64, messageID int, data string) error {
	now := time.Now()
	switch {
	case data == "report:today":
		return h.sendDailyReport(ctx, chatID, now)
	case data == "report:yesterday":
		return h.sendDailyReport(ctx, chatID, now.AddDate(0, 0, -1))
	case strings.HasPrefix(data, "report:cal:"):
		month, err := time.ParseInLocation("2006-01", strings.TrimPrefix(data, "report:cal:"), time.Local)
		if err != nil {
			return err
		}
		salesDays, err := h.storage.SalesDays(ctx, month, month.AddDate(0, 1, 0))
		if err != nil {
			return err
		}
		markup := keyboards.ReportCalendar(month, salesDays, now)
		return h.editText(chatID, messageID, "📅 Оберіть день для звіту.\n• — дні, коли були продажі.", &markup)
	case strings.HasPrefix(data, "report:day:"):
		day, err := time.ParseInLocation("2006-01-02", strings.TrimPrefix(data, "report:day:"), time.Local)
		if err != nil {
			return err
		}
		return h.sendDailyReport(ctx, chatID, day)
	default:
		return nil
	}
}

func (h *Handler) sendDailyReport(ctx context.Context, chatID int64, date time.Time) error {
	report, err := h.storage.GetDailyReport(ctx, date)
	if err != nil {
		return err
	}
	markup := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("📅 Інша дата", "report:cal:"+date.Format("2006-01")),
	))
	return h.sendText(chatID, reports.FormatDailyReport(report), &markup)
}
