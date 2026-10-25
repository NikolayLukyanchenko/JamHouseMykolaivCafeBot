package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/bot/keyboards"
	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/models"
	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/storage"
	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/utils"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Handler struct {
	bot      *tgbotapi.BotAPI
	storage  *storage.Storage
	sessions *SessionManager
}

func New(bot *tgbotapi.BotAPI, store *storage.Storage) *Handler {
	return &Handler{bot: bot, storage: store, sessions: NewSessionManager()}
}

func (h *Handler) HandleUpdate(ctx context.Context, update tgbotapi.Update) error {
	switch {
	case update.CallbackQuery != nil:
		return h.handleCallback(ctx, update.CallbackQuery)
	case update.Message != nil && update.Message.Chat != nil && !update.Message.Chat.IsPrivate():
		// In groups the bot only reacts to the command that binds the group
		// for purchase requests; everything else there is ignored.
		if update.Message.IsCommand() && update.Message.Command() == "purchase_here" {
			return h.bindPurchaseChat(ctx, update.Message)
		}
		return nil
	case update.Message != nil && update.Message.IsCommand():
		return h.handleCommand(ctx, update.Message)
	case update.Message != nil:
		return h.handleMessage(ctx, update.Message)
	default:
		return nil
	}
}

func (h *Handler) handleCommand(ctx context.Context, message *tgbotapi.Message) error {
	if message == nil || message.From == nil {
		return nil
	}
	user, allowed, err := h.authorize(ctx, message.From)
	if err != nil {
		return err
	}

	switch message.Command() {
	case "start":
		if !allowed {
			return h.sendText(message.Chat.ID, "Доступ заборонено. Зверніться до адміністратора.", nil)
		}
		return h.sendWelcome(message.Chat.ID, user)
	case "admin":
		if !allowed {
			return h.sendText(message.Chat.ID, "Доступ заборонено. Зверніться до адміністратора.", nil)
		}
		if !hasAnyRole(user.Role, models.RoleAdmin) {
			return h.sendText(message.Chat.ID, "Адмін-панель доступна лише адміністратору.", nil)
		}
		return h.sendAdminPanel(message.Chat.ID)
	case "backup", "dbinfo":
		if !allowed || !hasAnyRole(user.Role, models.RoleAdmin) {
			return h.sendText(message.Chat.ID, "Команда доступна лише адміністратору.", nil)
		}
		if message.Command() == "backup" {
			return h.sendBackup(ctx, message.Chat.ID)
		}
		return h.sendDBInfo(ctx, message.Chat.ID)
	case "report":
		if !allowed {
			return h.sendText(message.Chat.ID, "Доступ заборонено. Зверніться до адміністратора.", nil)
		}
		if !hasAnyRole(user.Role, models.RoleAdmin, models.RoleSellerHead) {
			return h.sendText(message.Chat.ID, "Звіт доступний лише головному касиру або адміністратору.", nil)
		}
		return h.sendText(message.Chat.ID, "Оберіть період для звіту.", keyboards.ReportPeriods(time.Now()))
	default:
		if !allowed {
			return h.sendText(message.Chat.ID, "Доступ заборонено. Зверніться до адміністратора.", nil)
		}
		return h.sendText(message.Chat.ID, "Я поки не знаю такої команди. Скористайтеся меню нижче або командами /start, /admin, /report.", replyKeyboard(user.Role))
	}
}

func (h *Handler) handleMessage(ctx context.Context, message *tgbotapi.Message) error {
	if message == nil || message.From == nil {
		return nil
	}
	user, allowed, err := h.authorize(ctx, message.From)
	if err != nil {
		return err
	}
	if !allowed {
		return h.sendText(message.Chat.ID, "Доступ заборонено. Зверніться до адміністратора.", nil)
	}

	// A main-menu button always wins over a pending text prompt, so tapping
	// a menu button is never swallowed as the answer to a previous question.
	if isMenuButton(message.Text) {
		h.sessions.ClearState(user.UserID)
	} else if err := h.handleStateMessage(ctx, user, message); err != errStateNotHandled {
		return err
	}

	switch strings.TrimSpace(message.Text) {
	case keyboards.SaleButton, "Записати продаж", "Калькулятор замовлення":
		return h.openPOS(ctx, message.Chat.ID, user.UserID)
	case "Меню для клієнтів":
		return h.sendCustomerMenu(ctx, message.Chat.ID)
	case "Мої продажі за сьогодні":
		return h.sendMySales(ctx, message.Chat.ID, 0, user, time.Now())
	case "Залишки товарів":
		return h.sendStocks(ctx, message.Chat.ID, false)
	case "Замовити закупку":
		return h.startPurchaseRequest(ctx, message.Chat.ID, user.UserID)
	case "Відправити звіт":
		if !hasAnyRole(user.Role, models.RoleAdmin) {
			return h.sendText(message.Chat.ID, "Кнопка звіту доступна лише адміністратору.", replyKeyboard(user.Role))
		}
		return h.sendText(message.Chat.ID, "Оберіть період для звіту.", keyboards.ReportPeriods(time.Now()))
	default:
		return h.sendText(message.Chat.ID, "Не зрозумів повідомлення. Оберіть дію з меню нижче або скористайтеся /start.", replyKeyboard(user.Role))
	}
}

var errStateNotHandled = fmt.Errorf("state not handled")

func (h *Handler) handleStateMessage(ctx context.Context, user models.User, message *tgbotapi.Message) error {
	s := h.sessions.get(user.UserID)
	text := strings.TrimSpace(message.Text)
	if s.State == stateNone {
		return errStateNotHandled
	}
	if strings.EqualFold(text, "скасувати") {
		return h.cancelInput(ctx, user, message.Chat.ID)
	}

	switch s.State {
	case stateAwaitPurchaseNote:
		h.sessions.ClearState(user.UserID)
		s.Purchase.Note = text
		return h.startPurchaseScreenKeepingDraft(ctx, message.Chat.ID, user.UserID)
	case stateAwaitPItemsAdd:
		if !hasAnyRole(user.Role, models.RoleAdmin) {
			return h.sendText(message.Chat.ID, "Лише адміністратор може змінювати список закупки.", nil)
		}
		h.sessions.ClearState(user.UserID)
		return h.addPurchaseItems(ctx, message.Chat.ID, text)
	case stateAwaitPItemEdit:
		if !hasAnyRole(user.Role, models.RoleAdmin) {
			return h.sendText(message.Chat.ID, "Лише адміністратор може змінювати список закупки.", nil)
		}
		name, unit, ok := utils.ParsePurchaseItemLine(text)
		if !ok {
			return h.sendPrompt(message.Chat.ID, "Назва не може бути порожньою. Введіть ще раз.\n\n"+purchaseItemFormatHint)
		}
		h.sessions.ClearState(user.UserID)
		if err := h.storage.UpdatePurchaseItem(ctx, s.EditingPurchaseItemID, name, unit); err != nil {
			return err
		}
		return h.sendPurchaseItemsAdmin(ctx, message.Chat.ID, "Позицію оновлено.")
	case stateAwaitProductName:
		if !hasAnyRole(user.Role, models.RoleAdmin) {
			return h.sendText(message.Chat.ID, "Лише адміністратор може редагувати товари.", nil)
		}
		s.DraftProduct.Name = text
		s.State = stateNone
		return h.sendText(message.Chat.ID, "Оберіть категорію товару.", keyboards.Categories("admin:category:"))
	case stateAwaitProductCost:
		value, err := utils.ParsePositiveFloat(text)
		if err != nil {
			return h.sendPrompt(message.Chat.ID, "Введіть коректну собівартість числом.")
		}
		s.DraftProduct.CostPrice = value
		s.State = stateAwaitProductSell
		return h.sendPrompt(message.Chat.ID, "Введіть ціну продажу.")
	case stateAwaitProductSell:
		value, err := utils.ParsePositiveFloat(text)
		if err != nil {
			return h.sendPrompt(message.Chat.ID, "Введіть коректну ціну продажу числом.")
		}
		s.DraftProduct.SellPrice = value
		s.State = stateAwaitProductUnit
		return h.sendPrompt(message.Chat.ID, "Введіть одиницю виміру (наприклад: шт, чашка, банка, порція, 100г).")
	case stateAwaitProductUnit:
		s.DraftProduct.Unit = text
		s.State = stateAwaitProductStock
		return h.sendPrompt(message.Chat.ID, "Введіть початковий залишок товару.")
	case stateAwaitProductStock:
		value, err := utils.ParseNonNegativeFloat(text)
		if err != nil {
			return h.sendPrompt(message.Chat.ID, "Введіть коректний залишок числом.")
		}
		s.DraftProduct.Stock = value
		productID, err := h.storage.CreateProduct(ctx, models.Product{Name: s.DraftProduct.Name, Category: s.DraftProduct.Category, CostPrice: s.DraftProduct.CostPrice, SellPrice: s.DraftProduct.SellPrice, Unit: s.DraftProduct.Unit, Stock: s.DraftProduct.Stock, IsActive: true})
		if err != nil {
			return err
		}
		h.sessions.ResetDraft(user.UserID)
		h.sessions.ClearState(user.UserID)
		return h.sendText(message.Chat.ID, fmt.Sprintf("✅ Товар успішно додано (ID: %d).", productID), keyboards.AdminPanel())
	case stateAwaitEditName:
		h.sessions.ClearState(user.UserID)
		if err := h.storage.UpdateProductName(ctx, s.EditingProductID, text); err != nil {
			return err
		}
		return h.sendAdminProductCard(ctx, message.Chat.ID, s.EditingProductID, "Назву товару оновлено.")
	case stateAwaitEditCost:
		value, err := utils.ParsePositiveFloat(text)
		if err != nil {
			return h.sendPrompt(message.Chat.ID, "Введіть коректну собівартість числом.")
		}
		h.sessions.ClearState(user.UserID)
		if err := h.storage.UpdateProductCostPrice(ctx, s.EditingProductID, value); err != nil {
			return err
		}
		return h.sendAdminProductCard(ctx, message.Chat.ID, s.EditingProductID, "Собівартість оновлено.")
	case stateAwaitEditSell:
		value, err := utils.ParsePositiveFloat(text)
		if err != nil {
			return h.sendPrompt(message.Chat.ID, "Введіть коректну ціну продажу числом.")
		}
		h.sessions.ClearState(user.UserID)
		if err := h.storage.UpdateProductSellPrice(ctx, s.EditingProductID, value); err != nil {
			return err
		}
		return h.sendAdminProductCard(ctx, message.Chat.ID, s.EditingProductID, "Ціну продажу оновлено.")
	case stateAwaitBundleName:
		if !hasAnyRole(user.Role, models.RoleAdmin) {
			return h.sendText(message.Chat.ID, "Лише адміністратор може змінювати набори.", nil)
		}
		if text == "" {
			return h.sendPrompt(message.Chat.ID, "Назва не може бути порожньою.")
		}
		h.sessions.ClearState(user.UserID)
		if err := h.storage.RenameBundle(ctx, s.EditingBundleID, text); err != nil {
			return err
		}
		return h.sendBundleCard(ctx, message.Chat.ID, s.EditingBundleID, "Назву набору оновлено.")
	case stateAwaitEditUnit:
		if text == "" {
			return h.sendPrompt(message.Chat.ID, "Одиниця виміру не може бути порожньою. Введіть, наприклад: шт, чашка, порція, 100г.")
		}
		h.sessions.ClearState(user.UserID)
		if err := h.storage.UpdateProductUnit(ctx, s.EditingProductID, text); err != nil {
			return err
		}
		return h.sendAdminProductCard(ctx, message.Chat.ID, s.EditingProductID, "Одиницю виміру оновлено.")
	case stateAwaitSetStock:
		value, err := utils.ParseNonNegativeFloat(text)
		if err != nil {
			return h.sendPrompt(message.Chat.ID, "Введіть коректний залишок числом.")
		}
		h.sessions.ClearState(user.UserID)
		if err := h.storage.UpdateProductStock(ctx, s.EditingProductID, value); err != nil {
			return err
		}
		return h.sendAdminProductCard(ctx, message.Chat.ID, s.EditingProductID, "Залишок оновлено.")
	case stateAwaitIncreaseStock:
		value, err := utils.ParsePositiveFloat(text)
		if err != nil {
			return h.sendPrompt(message.Chat.ID, "Введіть коректну кількість для поповнення.")
		}
		h.sessions.ClearState(user.UserID)
		if err := h.storage.IncreaseProductStock(ctx, s.EditingProductID, value); err != nil {
			return err
		}
		return h.sendAdminProductCard(ctx, message.Chat.ID, s.EditingProductID, "Залишок поповнено.")
	default:
		return errStateNotHandled
	}
}

func (h *Handler) handleCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) error {
	if callback == nil || callback.From == nil || callback.Message == nil {
		return nil
	}
	if callback.Data == "noop" || !callback.Message.Chat.IsPrivate() {
		return h.answerCallback(callback.ID, "")
	}
	user, allowed, err := h.authorize(ctx, callback.From)
	if err != nil {
		return err
	}
	if !allowed {
		_ = h.answerCallback(callback.ID, "Доступ заборонено")
		return h.sendText(callback.Message.Chat.ID, "Доступ заборонено. Зверніться до адміністратора.", nil)
	}
	data := callback.Data
	if strings.HasPrefix(data, "pos:") {
		// The checkout screen answers with its own short toasts.
		return h.handlePOSCallback(ctx, user, callback)
	}
	_ = h.answerCallback(callback.ID, "Готово")

	if strings.HasPrefix(data, "admin:") && !hasAnyRole(user.Role, models.RoleAdmin) {
		return h.sendText(callback.Message.Chat.ID, "Адмін-панель доступна лише адміністратору.", nil)
	}
	switch {
	case data == "nav:main":
		h.sessions.ClearState(user.UserID)
		return h.sendWelcome(callback.Message.Chat.ID, user)
	case data == "nav:cancel":
		return h.cancelInput(ctx, user, callback.Message.Chat.ID)
	case strings.HasPrefix(data, "pur:"):
		return h.handlePurchaseCallback(ctx, user, callback)
	case data == "admin:panel":
		h.sessions.ClearState(user.UserID)
		return h.sendAdminPanel(callback.Message.Chat.ID)
	case strings.HasPrefix(data, "admin:bundle"):
		return h.handleBundleAdmin(ctx, user, callback)
	case data == "admin:pitems", data == "admin:pchat_unbind", strings.HasPrefix(data, "admin:pitem"):
		return h.handlePurchaseItemsAdminCallback(ctx, user, callback.Message.Chat.ID, data)
	case data == "admin:add_product":
		h.sessions.ResetDraft(user.UserID)
		h.sessions.SetState(user.UserID, stateAwaitProductName)
		return h.sendPrompt(callback.Message.Chat.ID, "Введіть назву нового товару українською мовою.")
	case data == "admin:list_products":
		return h.sendStocks(ctx, callback.Message.Chat.ID, true)
	case data == "admin:edit_product", data == "admin:replenish_stock":
		return h.sendAdminProductPicker(ctx, callback.Message.Chat.ID, data == "admin:replenish_stock")
	case strings.HasPrefix(data, "admin:category:"):
		s := h.sessions.get(user.UserID)
		s.DraftProduct.Category = strings.TrimPrefix(data, "admin:category:")
		s.State = stateAwaitProductCost
		return h.sendPrompt(callback.Message.Chat.ID, "Введіть собівартість товару.")
	case strings.HasPrefix(data, "admin:edit_select:"):
		productID, err := strconv.ParseInt(strings.TrimPrefix(data, "admin:edit_select:"), 10, 64)
		if err != nil {
			return err
		}
		return h.sendAdminProductCard(ctx, callback.Message.Chat.ID, productID, "")
	case strings.HasPrefix(data, "admin:replenish_select:"):
		productID, err := strconv.ParseInt(strings.TrimPrefix(data, "admin:replenish_select:"), 10, 64)
		if err != nil {
			return err
		}
		s := h.sessions.get(user.UserID)
		s.EditingProductID = productID
		s.State = stateAwaitIncreaseStock
		return h.sendPrompt(callback.Message.Chat.ID, "Введіть кількість, на яку потрібно поповнити залишок.")
	case strings.HasPrefix(data, "admin:action:"):
		parts := strings.Split(data, ":")
		if len(parts) != 4 {
			return nil
		}
		action := parts[2]
		productID, err := strconv.ParseInt(parts[3], 10, 64)
		if err != nil {
			return err
		}
		s := h.sessions.get(user.UserID)
		s.EditingProductID = productID
		switch action {
		case "name":
			s.State = stateAwaitEditName
			return h.sendPrompt(callback.Message.Chat.ID, "Введіть нову назву товару.")
		case "cost":
			s.State = stateAwaitEditCost
			return h.sendPrompt(callback.Message.Chat.ID, "Введіть нову собівартість товару.")
		case "sell":
			s.State = stateAwaitEditSell
			return h.sendPrompt(callback.Message.Chat.ID, "Введіть нову ціну продажу товару.")
		case "unit":
			product, err := h.storage.GetProduct(ctx, productID)
			if err != nil {
				return err
			}
			s.State = stateAwaitEditUnit
			return h.sendPrompt(callback.Message.Chat.ID, fmt.Sprintf("Зараз: «%s». Введіть нову одиницю виміру (наприклад: шт, чашка, банка, порція, 100г).", product.Unit))
		case "stock":
			s.State = stateAwaitSetStock
			return h.sendPrompt(callback.Message.Chat.ID, "Введіть новий фактичний залишок товару.")
		case "hide":
			product, err := h.storage.GetProduct(ctx, productID)
			if err != nil {
				return err
			}
			markup := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("Так, прибрати", fmt.Sprintf("admin:action:hideok:%d", productID)),
				tgbotapi.NewInlineKeyboardButtonData("Ні", fmt.Sprintf("admin:edit_select:%d", productID)),
			))
			return h.sendText(callback.Message.Chat.ID, fmt.Sprintf("Прибрати «%s» з меню? Товар зникне з продажу та меню для клієнтів. Історія продажів і звіти збережуться, товар можна буде повернути.", product.Name), &markup)
		case "hideok":
			if err := h.storage.SetProductActive(ctx, productID, false); err != nil {
				return err
			}
			return h.sendAdminProductCard(ctx, callback.Message.Chat.ID, productID, "Товар прибрано з меню.")
		case "show":
			if err := h.storage.SetProductActive(ctx, productID, true); err != nil {
				return err
			}
			return h.sendAdminProductCard(ctx, callback.Message.Chat.ID, productID, "Товар повернуто в меню.")
		default:
			return nil
		}
	case strings.HasPrefix(data, "cart:"):
		// Buttons of the old step-by-step sale screen still in chat history.
		return h.openPOS(ctx, callback.Message.Chat.ID, user.UserID)
	case strings.HasPrefix(data, "my:day:"):
		day, err := parseDay(strings.TrimPrefix(data, "my:day:"))
		if err != nil {
			return err
		}
		return h.sendMySales(ctx, callback.Message.Chat.ID, callback.Message.MessageID, user, day)
	case strings.HasPrefix(data, "report:"):
		if !hasAnyRole(user.Role, models.RoleAdmin, models.RoleSellerHead) {
			return h.sendText(callback.Message.Chat.ID, "Звіт доступний лише головному касиру або адміністратору.", nil)
		}
		return h.handleReportCallback(ctx, callback.Message.Chat.ID, callback.Message.MessageID, data)
	default:
		return h.sendText(callback.Message.Chat.ID, "Невідома дія. Спробуйте ще раз з головного меню.", replyKeyboard(user.Role))
	}
}

func (h *Handler) authorize(ctx context.Context, tgUser *tgbotapi.User) (models.User, bool, error) {
	if tgUser == nil {
		return models.User{}, false, nil
	}
	user, err := h.storage.GetUser(ctx, tgUser.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			return models.User{}, false, nil
		}
		return models.User{}, false, err
	}
	user.Username = tgUser.UserName
	user.FullName = utils.TelegramFullName(tgUser)
	if err := h.storage.TouchUser(ctx, user); err != nil {
		return models.User{}, false, err
	}
	return user, true, nil
}

func (h *Handler) sendWelcome(chatID int64, user models.User) error {
	return h.sendText(chatID, fmt.Sprintf("Вітаю, %s! Оберіть дію з головного меню.", user.FullName), replyKeyboard(user.Role))
}

func (h *Handler) sendAdminPanel(chatID int64) error {
	return h.sendText(chatID, "Адмін-панель: оберіть потрібну дію.", keyboards.AdminPanel())
}

func (h *Handler) sendCustomerMenu(ctx context.Context, chatID int64) error {
	products, err := h.storage.ListProducts(ctx, false)
	if err != nil {
		return err
	}
	if len(products) == 0 {
		return h.sendText(chatID, "Меню поки порожнє. Додайте товари через адмін-панель.", nil)
	}
	var builder strings.Builder
	builder.WriteString("📜 Меню для клієнтів\n\n")
	for _, category := range models.Categories() {
		found := false
		for _, product := range products {
			if product.Category != category {
				continue
			}
			if !found {
				builder.WriteString(fmt.Sprintf("%s %s\n", utils.CategoryEmoji(category), category))
				found = true
			}
			builder.WriteString(fmt.Sprintf("• %s — %s / %s\n", product.Name, utils.FormatMoney(product.SellPrice), product.Unit))
		}
		if found {
			builder.WriteString("\n")
		}
	}
	return h.sendText(chatID, strings.TrimSpace(builder.String()), nil)
}

func (h *Handler) sendStocks(ctx context.Context, chatID int64, includeCost bool) error {
	products, err := h.storage.ListProducts(ctx, includeCost)
	if err != nil {
		return err
	}
	if len(products) == 0 {
		return h.sendText(chatID, "Список товарів порожній.", nil)
	}
	var builder strings.Builder
	builder.WriteString("📦 Залишки товарів\n\n")
	for _, product := range products {
		builder.WriteString(fmt.Sprintf("• %s (%s) — %s %s, ціна %s", product.Name, product.Category, utils.FormatQuantity(product.Stock), product.Unit, utils.FormatMoney(product.SellPrice)))
		if includeCost {
			builder.WriteString(fmt.Sprintf(", собівартість %s", utils.FormatMoney(product.CostPrice)))
		}
		if !product.IsActive {
			builder.WriteString(" [неактивний]")
		}
		builder.WriteString("\n")
	}
	return h.sendText(chatID, strings.TrimSpace(builder.String()), nil)
}

func (h *Handler) sendAdminProductPicker(ctx context.Context, chatID int64, replenish bool) error {
	products, err := h.storage.ListProducts(ctx, true)
	if err != nil {
		return err
	}
	if len(products) == 0 {
		return h.sendText(chatID, "Список товарів порожній.", keyboards.AdminPanel())
	}
	prefix := "admin:edit_select:"
	title := "Оберіть товар для редагування."
	if replenish {
		prefix = "admin:replenish_select:"
		title = "Оберіть товар для поповнення залишку."
	}
	for i := range products {
		if !products[i].IsActive {
			products[i].Name = "🚫 " + products[i].Name
		}
	}
	return h.sendText(chatID, title, keyboards.ProductButtons(products, prefix, "admin:list_products"))
}

func (h *Handler) sendAdminProductCard(ctx context.Context, chatID, productID int64, notice string) error {
	product, err := h.storage.GetProduct(ctx, productID)
	if err != nil {
		return err
	}
	text := fmt.Sprintf("📦 %s\n\nКатегорія: %s\nСобівартість: %s\nЦіна продажу: %s\nОдиниця: %s\nЗалишок: %s %s", product.Name, product.Category, utils.FormatMoney(product.CostPrice), utils.FormatMoney(product.SellPrice), product.Unit, utils.FormatQuantity(product.Stock), product.Unit)
	visibility := tgbotapi.NewInlineKeyboardButtonData("🗑 Прибрати з меню", fmt.Sprintf("admin:action:hide:%d", productID))
	if !product.IsActive {
		text += "\n\n🚫 Прибрано з меню — не продається і не показується клієнтам."
		visibility = tgbotapi.NewInlineKeyboardButtonData("♻️ Повернути в меню", fmt.Sprintf("admin:action:show:%d", productID))
	}
	if notice != "" {
		text = "✅ " + notice + "\n\n" + text
	}
	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✏️ Назва", fmt.Sprintf("admin:action:name:%d", productID)),
			tgbotapi.NewInlineKeyboardButtonData("💸 Собівартість", fmt.Sprintf("admin:action:cost:%d", productID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💰 Ціна продажу", fmt.Sprintf("admin:action:sell:%d", productID)),
			tgbotapi.NewInlineKeyboardButtonData("📦 Залишок", fmt.Sprintf("admin:action:stock:%d", productID)),
		),
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("📏 Одиниця виміру", fmt.Sprintf("admin:action:unit:%d", productID))),
		tgbotapi.NewInlineKeyboardRow(visibility),
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("⬅️ До адмін-панелі", "admin:panel")),
	)
	return h.sendText(chatID, text, &markup)
}

func (h *Handler) saleReceiptText(sale models.Sale, items []models.SaleItem) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("✅ Продаж успішно записано. Чек #%d\n\n", sale.ID))
	for _, item := range items {
		builder.WriteString(fmt.Sprintf("• %s — %s × %s = %s\n", item.Name, utils.FormatQuantity(item.Qty), utils.FormatMoney(item.SellPrice), utils.FormatMoney(item.Qty*item.SellPrice)))
	}
	paymentLabel := "💵 Готівка"
	if sale.PaymentMethod == models.PaymentCard {
		paymentLabel = "💳 Карта"
	}
	builder.WriteString(fmt.Sprintf("\n%s\n💰 Разом: %s", paymentLabel, utils.FormatMoney(sale.Total)))
	return builder.String()
}

// sendText sends text, splitting it into several messages when it exceeds
// Telegram's length limit; the markup goes with the last part.
func (h *Handler) sendText(chatID int64, text string, markup any) error {
	parts := utils.SplitMessage(text, telegramMessageLimit)
	for i, part := range parts {
		msg := tgbotapi.NewMessage(chatID, part)
		if markup != nil && i == len(parts)-1 {
			msg.ReplyMarkup = markup
		}
		if _, err := h.bot.Send(msg); err != nil {
			return err
		}
	}
	return nil
}

// telegramMessageLimit is in bytes, which is never less than Telegram's
// limit of 4096 UTF-16 code units for the same text.
const telegramMessageLimit = 4000

// sendPrompt asks for typed input and offers a "Скасувати" button.
func (h *Handler) sendPrompt(chatID int64, text string) error {
	markup := keyboards.Cancel()
	return h.sendText(chatID, text, &markup)
}

func (h *Handler) editText(chatID int64, messageID int, text string, markup *tgbotapi.InlineKeyboardMarkup) error {
	edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
	edit.ReplyMarkup = markup
	_, err := h.bot.Send(edit)
	if err != nil && strings.Contains(err.Error(), "message is not modified") {
		return nil
	}
	return err
}

// cancelInput aborts whatever text input the user was asked for.
func (h *Handler) cancelInput(ctx context.Context, user models.User, chatID int64) error {
	s := h.sessions.get(user.UserID)
	state := s.State
	h.sessions.ClearState(user.UserID)
	switch state {
	case stateAwaitPurchaseNote:
		// Back to the purchase request being composed; keep selected items.
		return h.startPurchaseScreenKeepingDraft(ctx, chatID, user.UserID)
	case stateAwaitPItemsAdd, stateAwaitPItemEdit:
		return h.sendPurchaseItemsAdmin(ctx, chatID, "")
	}
	h.sessions.ResetDraft(user.UserID)
	return h.sendText(chatID, "Дію скасовано.", replyKeyboard(user.Role))
}

func (h *Handler) startPurchaseScreenKeepingDraft(ctx context.Context, chatID, userID int64) error {
	text, markup, err := h.purchaseScreen(ctx, userID)
	if err != nil {
		return err
	}
	return h.sendText(chatID, text, &markup)
}

func isMenuButton(text string) bool {
	switch strings.TrimSpace(text) {
	case keyboards.SaleButton, "Записати продаж", "Калькулятор замовлення", "Меню для клієнтів", "Мої продажі за сьогодні",
		"Залишки товарів", "Замовити закупку", "Відправити звіт":
		return true
	}
	return false
}

func (h *Handler) answerCallback(callbackID, text string) error {
	_, err := h.bot.Request(tgbotapi.NewCallback(callbackID, text))
	return err
}

func replyKeyboard(role string) *tgbotapi.ReplyKeyboardMarkup {
	keyboard := keyboards.MainMenu(role)
	return &keyboard
}

func hasAnyRole(role string, allowed ...string) bool {
	for _, candidate := range allowed {
		if role == candidate {
			return true
		}
	}
	return false
}

func cartTotal(cart []models.OrderItem) float64 {
	var total float64
	for _, item := range cart {
		total += item.Qty * item.SellPrice
	}
	return total
}

func (h *Handler) qtyInCart(userID, productID int64) float64 {
	var qty float64
	for _, item := range h.sessions.Cart(userID) {
		if item.ProductID == productID {
			qty += item.Qty
		}
	}
	return qty
}
