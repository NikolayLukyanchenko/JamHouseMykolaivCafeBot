package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/bot/keyboards"
	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/models"
	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/utils"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const lowStockThreshold = 5

// startPurchaseRequest opens a fresh purchase request with the tap-to-select list.
func (h *Handler) startPurchaseRequest(ctx context.Context, chatID, userID int64) error {
	h.sessions.ResetPurchase(userID)
	text, markup, err := h.purchaseScreen(ctx, userID)
	if err != nil {
		return err
	}
	return h.sendText(chatID, text, &markup)
}

// refreshPurchaseScreen redraws the purchase request message in place.
func (h *Handler) refreshPurchaseScreen(ctx context.Context, chatID int64, messageID int, userID int64) error {
	text, markup, err := h.purchaseScreen(ctx, userID)
	if err != nil {
		return err
	}
	return h.editText(chatID, messageID, text, &markup)
}

func (h *Handler) purchaseScreen(ctx context.Context, userID int64) (string, tgbotapi.InlineKeyboardMarkup, error) {
	items, err := h.storage.ListPurchaseItems(ctx)
	if err != nil {
		return "", tgbotapi.InlineKeyboardMarkup{}, err
	}
	lowStock, err := h.storage.ListLowStockProducts(ctx, lowStockThreshold)
	if err != nil {
		return "", tgbotapi.InlineKeyboardMarkup{}, err
	}
	draft := h.sessions.get(userID).Purchase

	var b strings.Builder
	b.WriteString("🛒 Заявка на закупку\n\n")
	if len(lowStock) > 0 {
		b.WriteString("Низький залишок у меню:\n")
		for _, product := range lowStock {
			b.WriteString(fmt.Sprintf("• %s — %s %s\n", product.Name, utils.FormatQuantity(product.Stock), product.Unit))
		}
		b.WriteString("\n")
	}
	if len(items) == 0 {
		b.WriteString("Список позицій для закупки порожній. Адміністратор може заповнити його: /admin → «🛒 Позиції для закупки».\nПоки що можна описати потребу коментарем.\n")
	} else {
		b.WriteString("Натискайте на позиції, щоб додати їх до заявки. ➕/➖ змінюють кількість.\n")
	}
	if lines := purchaseLines(items, draft.Qty); len(lines) > 0 {
		b.WriteString("\nУ заявці:\n")
		b.WriteString(strings.Join(lines, "\n"))
		b.WriteString("\n")
	}
	if draft.Note != "" {
		b.WriteString("\n💬 Коментар: " + draft.Note + "\n")
	}
	return strings.TrimSpace(b.String()), keyboards.PurchaseRequest(items, draft.Qty), nil
}

func purchaseLines(items []models.PurchaseItem, qty map[int64]int) []string {
	var lines []string
	for _, item := range items {
		n := qty[item.ID]
		if n <= 0 {
			continue
		}
		line := fmt.Sprintf("• %s — %d", item.Name, n)
		if item.Unit != "" {
			line += " " + item.Unit
		}
		lines = append(lines, line)
	}
	return lines
}

func (h *Handler) handlePurchaseCallback(ctx context.Context, user models.User, callback *tgbotapi.CallbackQuery) error {
	chatID := callback.Message.Chat.ID
	messageID := callback.Message.MessageID
	data := callback.Data

	switch {
	case strings.HasPrefix(data, "pur:add:"), strings.HasPrefix(data, "pur:sub:"):
		itemID, err := strconv.ParseInt(data[len("pur:add:"):], 10, 64)
		if err != nil {
			return err
		}
		delta := 1
		if strings.HasPrefix(data, "pur:sub:") {
			delta = -1
		}
		h.sessions.AdjustPurchaseQty(user.UserID, itemID, delta)
		return h.refreshPurchaseScreen(ctx, chatID, messageID, user.UserID)
	case data == "pur:comment":
		h.sessions.SetState(user.UserID, stateAwaitPurchaseNote)
		return h.sendPrompt(chatID, "Напишіть коментар до заявки (наприклад, що ще потрібно або терміновість).")
	case data == "pur:cancel":
		h.sessions.ResetPurchase(user.UserID)
		h.sessions.ClearState(user.UserID)
		if err := h.editText(chatID, messageID, "❌ Заявку на закупку скасовано.", nil); err != nil {
			log.Printf("не вдалося оновити повідомлення заявки: %v", err)
		}
		return h.sendText(chatID, "Оберіть дію з меню.", replyKeyboard(user.Role))
	case data == "pur:send":
		return h.sendPurchaseRequest(ctx, user, chatID, messageID)
	default:
		return nil
	}
}

func (h *Handler) sendPurchaseRequest(ctx context.Context, user models.User, chatID int64, messageID int) error {
	items, err := h.storage.ListPurchaseItems(ctx)
	if err != nil {
		return err
	}
	draft := h.sessions.get(user.UserID).Purchase
	lines := purchaseLines(items, draft.Qty)
	if len(lines) == 0 && draft.Note == "" {
		return h.sendText(chatID, "Заявка порожня. Оберіть хоча б одну позицію або додайте коментар.", nil)
	}

	var b strings.Builder
	b.WriteString("🛒 Нова заявка на закупку\n\nВід: " + user.FullName)
	if user.Username != "" {
		b.WriteString(" (@" + user.Username + ")")
	}
	b.WriteString("\n")
	if len(lines) > 0 {
		b.WriteString("\n" + strings.Join(lines, "\n") + "\n")
	}
	if draft.Note != "" {
		b.WriteString("\n💬 Коментар: " + draft.Note + "\n")
	}
	requestText := strings.TrimSpace(b.String())

	delivered, failed, err := h.deliverPurchaseRequest(ctx, user, requestText)
	if err != nil {
		return err
	}
	if len(delivered) == 0 {
		text := "⚠️ Заявку не надіслано — її нікому доставити.\n\n" + h.purchaseSetupHint()
		if len(failed) > 0 {
			text = "⚠️ Заявку не вдалося доставити. Вона збережена — спробуйте надіслати ще раз.\n\n" + strings.Join(failed, "\n") + "\n\n" + h.purchaseSetupHint()
		}
		return h.sendText(chatID, text, nil)
	}

	h.sessions.ResetPurchase(user.UserID)
	h.sessions.ClearState(user.UserID)
	if err := h.editText(chatID, messageID, requestText, nil); err != nil {
		log.Printf("не вдалося оновити повідомлення заявки: %v", err)
	}
	result := "✅ Заявку надіслано: " + strings.Join(delivered, ", ") + "."
	if len(failed) > 0 {
		result += "\n\n⚠️ Не доставлено:\n" + strings.Join(failed, "\n")
	}
	return h.sendText(chatID, result, replyKeyboard(user.Role))
}

const (
	settingPurchaseChatID    = "purchase_chat_id"
	settingPurchaseChatTitle = "purchase_chat_title"
)

func (h *Handler) purchaseSetupHint() string {
	return fmt.Sprintf("Як налаштувати: створіть групу в Telegram (наприклад, «Закупка JamHouse»), додайте туди бота і надішліть у групі команду /purchase_here@%s — після цього всі заявки надходитимуть у цю групу.", h.bot.Self.UserName)
}

// purchaseChat returns the group bound for purchase requests, or 0.
func (h *Handler) purchaseChat(ctx context.Context) (int64, string, error) {
	raw, err := h.storage.GetSetting(ctx, settingPurchaseChatID)
	if err != nil || raw == "" {
		return 0, "", err
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, "", nil
	}
	title, err := h.storage.GetSetting(ctx, settingPurchaseChatTitle)
	return id, title, err
}

// deliverPurchaseRequest sends the request to the bound group. Without a
// group, or when the group is unreachable, it falls back to the other admins.
func (h *Handler) deliverPurchaseRequest(ctx context.Context, user models.User, text string) (delivered, failed []string, err error) {
	groupID, groupTitle, err := h.purchaseChat(ctx)
	if err != nil {
		return nil, nil, err
	}
	if groupID != 0 {
		sendErr := h.sendText(groupID, text, nil)
		var tgErr *tgbotapi.Error
		if errors.As(sendErr, &tgErr) && tgErr.MigrateToChatID != 0 {
			// The group became a supergroup and got a new ID.
			groupID = tgErr.MigrateToChatID
			if err := h.storage.SetSetting(ctx, settingPurchaseChatID, strconv.FormatInt(groupID, 10)); err != nil {
				log.Printf("не вдалося оновити ID групи закупки: %v", err)
			}
			sendErr = h.sendText(groupID, text, nil)
		}
		if sendErr == nil {
			return []string{"група «" + groupTitle + "»"}, nil, nil
		}
		log.Printf("не вдалося відправити заявку в групу %d: %v", groupID, sendErr)
		failed = append(failed, fmt.Sprintf("• група «%s»: %s", groupTitle, deliveryErrorHint(sendErr)))
	}

	admins, err := h.storage.ListAdmins(ctx)
	if err != nil {
		return nil, nil, err
	}
	for _, admin := range admins {
		if admin.UserID == user.UserID {
			continue
		}
		if err := h.sendText(admin.UserID, text, nil); err != nil {
			log.Printf("не вдалося відправити заявку на закупку адміну %d: %v", admin.UserID, err)
			failed = append(failed, fmt.Sprintf("• %s (ID %d): %s", admin.FullName, admin.UserID, deliveryErrorHint(err)))
			continue
		}
		delivered = append(delivered, admin.FullName)
	}
	return delivered, failed, nil
}

// bindPurchaseChat handles /purchase_here sent in a group by an admin.
func (h *Handler) bindPurchaseChat(ctx context.Context, message *tgbotapi.Message) error {
	user, allowed, err := h.authorize(ctx, message.From)
	if err != nil {
		return err
	}
	if !allowed || !hasAnyRole(user.Role, models.RoleAdmin) {
		return h.sendText(message.Chat.ID, "Налаштувати групу для заявок може лише адміністратор бота.", nil)
	}
	if err := h.storage.SetSetting(ctx, settingPurchaseChatID, strconv.FormatInt(message.Chat.ID, 10)); err != nil {
		return err
	}
	if err := h.storage.SetSetting(ctx, settingPurchaseChatTitle, message.Chat.Title); err != nil {
		return err
	}
	log.Printf("група для заявок на закупку: %d (%s), налаштував %d", message.Chat.ID, message.Chat.Title, user.UserID)
	return h.sendText(message.Chat.ID, "✅ Готово! Заявки на закупку тепер надходитимуть у цю групу.", nil)
}

func (h *Handler) unbindPurchaseChat(ctx context.Context) error {
	if err := h.storage.DeleteSetting(ctx, settingPurchaseChatID); err != nil {
		return err
	}
	return h.storage.DeleteSetting(ctx, settingPurchaseChatTitle)
}

// deliveryErrorHint turns common Telegram send errors into an actionable hint.
func deliveryErrorHint(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "chat not found"), strings.Contains(msg, "bot can't initiate conversation"):
		return "користувач ще не запускав бота — йому треба відкрити бота і натиснути /start"
	case strings.Contains(msg, "bot was blocked"):
		return "користувач заблокував бота"
	default:
		return msg
	}
}

// --- admin: managing the purchase item list ---

func (h *Handler) sendPurchaseItemsAdmin(ctx context.Context, chatID int64, notice string) error {
	items, err := h.storage.ListPurchaseItems(ctx)
	if err != nil {
		return err
	}
	groupID, groupTitle, err := h.purchaseChat(ctx)
	if err != nil {
		return err
	}
	text := "🛒 Позиції для закупки\n\nЦі позиції працівники обирають кнопками у «Замовити закупку». Натисніть на позицію, щоб змінити або видалити її."
	if len(items) == 0 {
		text = "🛒 Позиції для закупки\n\nСписок порожній. Натисніть «➕ Додати позиції»."
	}
	if groupID != 0 {
		text += "\n\n📨 Заявки надходять у групу «" + groupTitle + "»."
	} else {
		text += "\n\n📨 Група для заявок не налаштована — заявки отримують інші адміністратори.\n" + h.purchaseSetupHint()
	}
	if notice != "" {
		text = "✅ " + notice + "\n\n" + text
	}
	markup := keyboards.PurchaseItemsAdmin(items, groupID != 0)
	return h.sendText(chatID, text, &markup)
}

func (h *Handler) sendPurchaseItemCard(ctx context.Context, chatID, itemID int64) error {
	item, err := h.storage.GetPurchaseItem(ctx, itemID)
	if errors.Is(err, sql.ErrNoRows) {
		return h.sendPurchaseItemsAdmin(ctx, chatID, "")
	}
	if err != nil {
		return err
	}
	unit := item.Unit
	if unit == "" {
		unit = "—"
	}
	markup := keyboards.PurchaseItemCard(itemID)
	return h.sendText(chatID, fmt.Sprintf("🛒 %s\nОдиниця: %s", item.Name, unit), &markup)
}

const purchaseItemFormatHint = "Формат: «Назва, одиниця» (одиниця необов'язкова), наприклад:\nМолоко, л\nСтаканчики 250 мл, уп\nЦукор"

func (h *Handler) handlePurchaseItemsAdminCallback(ctx context.Context, user models.User, chatID int64, data string) error {
	switch {
	case data == "admin:pitems":
		h.sessions.ClearState(user.UserID)
		return h.sendPurchaseItemsAdmin(ctx, chatID, "")
	case data == "admin:pchat_unbind":
		if err := h.unbindPurchaseChat(ctx); err != nil {
			return err
		}
		return h.sendPurchaseItemsAdmin(ctx, chatID, "Групу для заявок відв'язано.")
	case data == "admin:pitem_add":
		h.sessions.SetState(user.UserID, stateAwaitPItemsAdd)
		return h.sendPrompt(chatID, "Надішліть одну або кілька позицій — кожну з нового рядка.\n\n"+purchaseItemFormatHint)
	}

	idx := strings.LastIndex(data, ":")
	itemID, err := strconv.ParseInt(data[idx+1:], 10, 64)
	if err != nil {
		return err
	}
	switch data[:idx+1] {
	case "admin:pitem:":
		return h.sendPurchaseItemCard(ctx, chatID, itemID)
	case "admin:pitem_edit:":
		s := h.sessions.get(user.UserID)
		s.EditingPurchaseItemID = itemID
		s.State = stateAwaitPItemEdit
		return h.sendPrompt(chatID, "Введіть нову назву позиції.\n\n"+purchaseItemFormatHint)
	case "admin:pitem_del:":
		item, err := h.storage.GetPurchaseItem(ctx, itemID)
		if errors.Is(err, sql.ErrNoRows) {
			return h.sendPurchaseItemsAdmin(ctx, chatID, "")
		}
		if err != nil {
			return err
		}
		markup := keyboards.ConfirmDeletePurchaseItem(itemID)
		return h.sendText(chatID, fmt.Sprintf("Видалити позицію «%s» зі списку закупки?", item.Name), &markup)
	case "admin:pitem_delok:":
		if err := h.storage.DeletePurchaseItem(ctx, itemID); err != nil {
			return err
		}
		return h.sendPurchaseItemsAdmin(ctx, chatID, "Позицію видалено.")
	default:
		return nil
	}
}

func (h *Handler) addPurchaseItems(ctx context.Context, chatID int64, text string) error {
	var added []string
	for _, line := range strings.Split(text, "\n") {
		name, unit, ok := utils.ParsePurchaseItemLine(line)
		if !ok {
			continue
		}
		if _, err := h.storage.CreatePurchaseItem(ctx, name, unit); err != nil {
			return err
		}
		added = append(added, name)
	}
	if len(added) == 0 {
		return h.sendPrompt(chatID, "Не знайшов жодної назви. Надішліть позиції ще раз.\n\n"+purchaseItemFormatHint)
	}
	return h.sendPurchaseItemsAdmin(ctx, chatID, fmt.Sprintf("Додано позицій: %d.", len(added)))
}
