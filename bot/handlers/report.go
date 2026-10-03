package handlers

import (
	"context"
	"strings"
	"time"

	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/bot/keyboards"
	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/models"
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
		day, err := parseDay(strings.TrimPrefix(data, "report:day:"))
		if err != nil {
			return err
		}
		return h.sendDailyReport(ctx, chatID, day)
	case strings.HasPrefix(data, "report:nav:"):
		day, err := parseDay(strings.TrimPrefix(data, "report:nav:"))
		if err != nil {
			return err
		}
		report, err := h.storage.GetDailyReport(ctx, day)
		if err != nil {
			return err
		}
		markup := keyboards.DailyReportActions(day, now)
		return h.showInPlace(chatID, messageID, reports.FormatDailyReport(report), &markup)
	case strings.HasPrefix(data, "report:checks:"):
		day, err := parseDay(strings.TrimPrefix(data, "report:checks:"))
		if err != nil {
			return err
		}
		checks, err := h.storage.ListChecks(ctx, day, 0)
		if err != nil {
			return err
		}
		return h.sendText(chatID, reports.FormatChecks(day, checks), nil)
	default:
		return nil
	}
}

func (h *Handler) sendDailyReport(ctx context.Context, chatID int64, date time.Time) error {
	report, err := h.storage.GetDailyReport(ctx, date)
	if err != nil {
		return err
	}
	markup := keyboards.DailyReportActions(date, time.Now())
	return h.sendText(chatID, reports.FormatDailyReport(report), &markup)
}

// sendMySales shows a seller's own sales for a day; messageID != 0 updates
// that message instead of sending a new one.
func (h *Handler) sendMySales(ctx context.Context, chatID int64, messageID int, user models.User, day time.Time) error {
	summary, err := h.storage.GetUserSalesSummary(ctx, user.UserID, day)
	if err != nil {
		return err
	}
	checks, err := h.storage.ListChecks(ctx, day, user.UserID)
	if err != nil {
		return err
	}
	now := time.Now()
	text := reports.FormatUserSales(user.FullName, day, now, summary, checks)
	markup := keyboards.MySalesActions(day, now)
	if messageID == 0 {
		return h.sendText(chatID, text, &markup)
	}
	return h.showInPlace(chatID, messageID, text, &markup)
}

// showInPlace edits a message when the new text fits into one message and
// falls back to sending it anew otherwise.
func (h *Handler) showInPlace(chatID int64, messageID int, text string, markup *tgbotapi.InlineKeyboardMarkup) error {
	if len(text) <= telegramMessageLimit {
		return h.editText(chatID, messageID, text, markup)
	}
	return h.sendText(chatID, text, markup)
}

func parseDay(raw string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", raw, time.Local)
}
