package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/bot/keyboards"
	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/models"
	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/utils"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Bundles are checkout shortcuts only: tapping one adds its products to the
// order, and everything after that (receipt, stock, reports) sees the
// individual products.

func (h *Handler) handleBundleAdmin(ctx context.Context, user models.User, callback *tgbotapi.CallbackQuery) error {
	chatID := callback.Message.Chat.ID
	messageID := callback.Message.MessageID
	action, arg, _ := strings.Cut(strings.TrimPrefix(callback.Data, "admin:"), ":")
	id, _ := strconv.ParseInt(arg, 10, 64)
	s := h.sessions.get(user.UserID)

	switch action {
	case "bundles":
		h.sessions.ClearState(user.UserID)
		return h.sendBundleList(ctx, chatID)
	case "bundle":
		return h.sendBundleCard(ctx, chatID, id, "")
	case "bundle_new":
		s.Bundle = bundleDraft{Qty: map[int64]int{}}
		return h.sendBundleBuilder(ctx, chatID, 0, user.UserID)
	case "bundle_edit":
		bundle, err := h.storage.GetBundle(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return h.sendBundleList(ctx, chatID)
		}
		if err != nil {
			return err
		}
		s.Bundle = bundleDraft{ID: id, Qty: map[int64]int{}}
		for _, item := range bundle.Items {
			s.Bundle.Qty[item.Product.ID] = item.Qty
			s.Bundle.Order = append(s.Bundle.Order, item.Product.ID)
		}
		return h.sendBundleBuilder(ctx, chatID, 0, user.UserID)
	case "bundle_add", "bundle_sub":
		if s.Bundle.Qty == nil {
			s.Bundle = bundleDraft{Qty: map[int64]int{}}
		}
		if action == "bundle_add" {
			if s.Bundle.Qty[id] == 0 {
				s.Bundle.Order = append(s.Bundle.Order, id)
			}
			s.Bundle.Qty[id]++
		} else if s.Bundle.Qty[id] > 0 {
			s.Bundle.Qty[id]--
		}
		return h.sendBundleBuilder(ctx, chatID, messageID, user.UserID)
	case "bundle_save":
		return h.saveBundle(ctx, chatID, user.UserID)
	case "bundle_rename":
		s.EditingBundleID = id
		s.State = stateAwaitBundleName
		return h.sendPrompt(chatID, "Введіть нову назву набору (наприклад: Кава + вершки).")
	case "bundle_del":
		bundle, err := h.storage.GetBundle(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return h.sendBundleList(ctx, chatID)
		}
		if err != nil {
			return err
		}
		markup := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Так, видалити", fmt.Sprintf("admin:bundle_delok:%d", id)),
			tgbotapi.NewInlineKeyboardButtonData("Ні", fmt.Sprintf("admin:bundle:%d", id)),
		))
		return h.sendText(chatID, fmt.Sprintf("Видалити набір «%s»? Товари, продажі та звіти це не зачепить.", bundle.Name), &markup)
	case "bundle_delok":
		if err := h.storage.DeleteBundle(ctx, id); err != nil {
			return err
		}
		return h.sendBundleList(ctx, chatID)
	default:
		return nil
	}
}

func (h *Handler) sendBundleList(ctx context.Context, chatID int64) error {
	bundles, err := h.storage.ListBundles(ctx)
	if err != nil {
		return err
	}
	text := "🧩 Набори для каси\n\nНабір — це кнопка в касі, яка одним натисканням додає кілька товарів (наприклад, Кава + Вершки). У чеку, залишках і звітах товари видно окремо, як завжди. Ціна набору — сума цін товарів."
	if len(bundles) == 0 {
		text += "\n\nНаборів ще немає."
	}
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(bundles)+2)
	for _, b := range bundles {
		label := fmt.Sprintf("🧩 %s · %s", b.Name, utils.FormatQuantity(b.Price()))
		if !keyboards.BundleAvailable(b) {
			label = "⛔ " + b.Name
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("admin:bundle:%d", b.ID))))
	}
	rows = append(rows,
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("➕ Створити набір", "admin:bundle_new")),
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("⬅️ До адмін-панелі", "admin:panel")),
	)
	markup := tgbotapi.NewInlineKeyboardMarkup(rows...)
	return h.sendText(chatID, text, &markup)
}

func (h *Handler) sendBundleCard(ctx context.Context, chatID, bundleID int64, notice string) error {
	bundle, err := h.storage.GetBundle(ctx, bundleID)
	if errors.Is(err, sql.ErrNoRows) {
		return h.sendBundleList(ctx, chatID)
	}
	if err != nil {
		return err
	}
	var b strings.Builder
	if notice != "" {
		b.WriteString("✅ " + notice + "\n\n")
	}
	b.WriteString(fmt.Sprintf("🧩 %s\n\nСклад:\n", bundle.Name))
	for _, item := range bundle.Items {
		b.WriteString(fmt.Sprintf("• %s ×%d — %s\n", item.Product.Name, item.Qty, utils.FormatMoney(item.Product.SellPrice*float64(item.Qty))))
		switch {
		case !item.Product.IsActive:
			b.WriteString("   🚫 прибрано з меню — набір недоступний у касі\n")
		case item.Product.Stock < float64(item.Qty):
			b.WriteString("   ⛔ немає на залишку\n")
		}
	}
	b.WriteString(fmt.Sprintf("\n💰 Ціна: %s (сума цін товарів)", utils.FormatMoney(bundle.Price())))
	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✏️ Назва", fmt.Sprintf("admin:bundle_rename:%d", bundleID)),
			tgbotapi.NewInlineKeyboardButtonData("🧩 Склад", fmt.Sprintf("admin:bundle_edit:%d", bundleID)),
		),
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("🗑 Видалити", fmt.Sprintf("admin:bundle_del:%d", bundleID))),
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("⬅️ До наборів", "admin:bundles")),
	)
	return h.sendText(chatID, b.String(), &markup)
}

// sendBundleBuilder shows (messageID 0) or redraws the composition screen.
func (h *Handler) sendBundleBuilder(ctx context.Context, chatID int64, messageID int, userID int64) error {
	products, err := h.storage.ListProducts(ctx, false)
	if err != nil {
		return err
	}
	draft := h.sessions.get(userID).Bundle
	byID := make(map[int64]models.Product, len(products))
	names := make(map[int64]string, len(products))
	for _, p := range products {
		byID[p.ID] = p
		names[p.ID] = p.Name
	}
	var b strings.Builder
	if draft.ID == 0 {
		b.WriteString("🧩 Новий набір\n\n")
	} else {
		b.WriteString("🧩 Склад набору\n\n")
	}
	var total float64
	var lines int
	for _, id := range draft.Order {
		p, ok := byID[id]
		if !ok || draft.Qty[id] <= 0 {
			continue
		}
		b.WriteString(fmt.Sprintf("• %s ×%d\n", p.Name, draft.Qty[id]))
		total += p.SellPrice * float64(draft.Qty[id])
		lines++
	}
	if lines == 0 {
		b.WriteString("Натискайте на товари, щоб додати їх у набір. ➖ прибирає.")
	} else {
		b.WriteString(fmt.Sprintf("\n💰 Ціна: %s", utils.FormatMoney(total)))
	}
	markup := keyboards.BundleBuilder(products, draft.Qty, draft.Order, names)
	if messageID == 0 {
		return h.sendText(chatID, b.String(), &markup)
	}
	return h.editText(chatID, messageID, b.String(), &markup)
}

func (h *Handler) saveBundle(ctx context.Context, chatID, userID int64) error {
	draft := h.sessions.get(userID).Bundle
	qty := make(map[int64]int)
	for id, n := range draft.Qty {
		if n > 0 {
			qty[id] = n
		}
	}
	if len(qty) == 0 {
		return h.sendText(chatID, "Додайте в набір хоча б один товар.", nil)
	}
	name := ""
	if draft.ID != 0 {
		bundle, err := h.storage.GetBundle(ctx, draft.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		name = bundle.Name
	}
	if name == "" {
		// A new bundle is named after its components, e.g. "Кава + Вершки";
		// it can be renamed from the bundle card.
		products, err := h.storage.ListProducts(ctx, true)
		if err != nil {
			return err
		}
		byID := make(map[int64]string, len(products))
		for _, p := range products {
			byID[p.ID] = p.Name
		}
		parts := make([]string, 0, len(qty))
		for _, id := range draft.Order {
			if qty[id] == 0 {
				continue
			}
			part := byID[id]
			if qty[id] > 1 {
				part += fmt.Sprintf(" ×%d", qty[id])
			}
			parts = append(parts, part)
		}
		name = strings.Join(parts, " + ")
	}
	id, err := h.storage.SaveBundle(ctx, draft.ID, name, qty)
	if err != nil {
		return err
	}
	h.sessions.get(userID).Bundle = bundleDraft{}
	return h.sendBundleCard(ctx, chatID, id, "Набір збережено. Він з'явиться у касі на вкладці «⭐ Топ».")
}
