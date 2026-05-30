package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/bot/keyboards"
	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/models"
	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/reports"
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
	case "report":
		if !allowed {
			return h.sendText(message.Chat.ID, "Доступ заборонено. Зверніться до адміністратора.", nil)
		}
		if !hasAnyRole(user.Role, models.RoleAdmin, models.RoleSellerHead) {
			return h.sendText(message.Chat.ID, "Звіт доступний лише головному касиру або адміністратору.", nil)
		}
		return h.sendText(message.Chat.ID, "Оберіть період для звіту.", keyboards.ReportPeriods())
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

	if err := h.handleStateMessage(ctx, user, message); err != nil {
		if err == errStateNotHandled {
			// continue with regular routing
		} else {
			return err
		}
	} else {
		return nil
	}

	switch strings.TrimSpace(message.Text) {
	case "Записати продаж", "Калькулятор замовлення":
		return h.sendCartAndCategories(message.Chat.ID, user.UserID)
	case "Меню для клієнтів":
		return h.sendCustomerMenu(ctx, message.Chat.ID)
	case "Мої продажі за сьогодні":
		return h.sendMySalesSummary(ctx, message.Chat.ID, user)
	case "Залишки товарів":
		return h.sendStocks(ctx, message.Chat.ID, false)
	case "Замовити закупку":
		return h.startPurchaseRequest(ctx, message.Chat.ID, user.UserID)
	case "Відправити звіт":
		if !hasAnyRole(user.Role, models.RoleAdmin) {
			return h.sendText(message.Chat.ID, "Кнопка звіту доступна лише адміністратору.", replyKeyboard(user.Role))
		}
		return h.sendText(message.Chat.ID, "Оберіть період для звіту.", keyboards.ReportPeriods())
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
		h.sessions.ClearState(user.UserID)
		h.sessions.ResetDraft(user.UserID)
		return h.sendText(message.Chat.ID, "Дію скасовано.", replyKeyboard(user.Role))
	}

	switch s.State {
	case stateAwaitOrderQty:
		qty, err := utils.ParsePositiveFloat(text)
		if err != nil {
			return h.sendText(message.Chat.ID, "Некоректна кількість. Введіть число більше нуля.", nil)
		}
		product, err := h.storage.GetProduct(ctx, s.SelectedProductID)
		if err != nil {
			return err
		}
		if product.Stock < qty+h.qtyInCart(user.UserID, product.ID) {
			return h.sendText(message.Chat.ID, fmt.Sprintf("Недостатньо залишку. Доступно лише %s %s.", utils.FormatQuantity(product.Stock), product.Unit), nil)
		}
		h.sessions.AddToCart(user.UserID, models.OrderItem{ProductID: product.ID, Name: product.Name, Category: product.Category, Qty: qty, SellPrice: product.SellPrice, CostPrice: product.CostPrice, Unit: product.Unit})
		h.sessions.ClearState(user.UserID)
		return h.sendCartSummary(message.Chat.ID, user.UserID)
	case stateAwaitPurchaseText:
		h.sessions.ClearState(user.UserID)
		return h.forwardPurchaseRequest(ctx, user, message.Chat.ID, text)
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
			return h.sendText(message.Chat.ID, "Введіть коректну собівартість числом.", nil)
		}
		s.DraftProduct.CostPrice = value
		s.State = stateAwaitProductSell
		return h.sendText(message.Chat.ID, "Введіть ціну продажу.", nil)
	case stateAwaitProductSell:
		value, err := utils.ParsePositiveFloat(text)
		if err != nil {
			return h.sendText(message.Chat.ID, "Введіть коректну ціну продажу числом.", nil)
		}
		s.DraftProduct.SellPrice = value
		s.State = stateAwaitProductUnit
		return h.sendText(message.Chat.ID, "Введіть одиницю виміру (наприклад: шт, чашка, банка, порція, 100г).", nil)
	case stateAwaitProductUnit:
		s.DraftProduct.Unit = text
		s.State = stateAwaitProductStock
		return h.sendText(message.Chat.ID, "Введіть початковий залишок товару.", nil)
	case stateAwaitProductStock:
		value, err := utils.ParseNonNegativeFloat(text)
		if err != nil {
			return h.sendText(message.Chat.ID, "Введіть коректний залишок числом.", nil)
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
			return h.sendText(message.Chat.ID, "Введіть коректну собівартість числом.", nil)
		}
		h.sessions.ClearState(user.UserID)
		if err := h.storage.UpdateProductCostPrice(ctx, s.EditingProductID, value); err != nil {
			return err
		}
		return h.sendAdminProductCard(ctx, message.Chat.ID, s.EditingProductID, "Собівартість оновлено.")
	case stateAwaitEditSell:
		value, err := utils.ParsePositiveFloat(text)
		if err != nil {
			return h.sendText(message.Chat.ID, "Введіть коректну ціну продажу числом.", nil)
		}
		h.sessions.ClearState(user.UserID)
		if err := h.storage.UpdateProductSellPrice(ctx, s.EditingProductID, value); err != nil {
			return err
		}
		return h.sendAdminProductCard(ctx, message.Chat.ID, s.EditingProductID, "Ціну продажу оновлено.")
	case stateAwaitSetStock:
		value, err := utils.ParseNonNegativeFloat(text)
		if err != nil {
			return h.sendText(message.Chat.ID, "Введіть коректний залишок числом.", nil)
		}
		h.sessions.ClearState(user.UserID)
		if err := h.storage.UpdateProductStock(ctx, s.EditingProductID, value); err != nil {
			return err
		}
		return h.sendAdminProductCard(ctx, message.Chat.ID, s.EditingProductID, "Залишок оновлено.")
	case stateAwaitIncreaseStock:
		value, err := utils.ParsePositiveFloat(text)
		if err != nil {
			return h.sendText(message.Chat.ID, "Введіть коректну кількість для поповнення.", nil)
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
	user, allowed, err := h.authorize(ctx, callback.From)
	if err != nil {
		return err
	}
	if !allowed {
		_ = h.answerCallback(callback.ID, "Доступ заборонено")
		return h.sendText(callback.Message.Chat.ID, "Доступ заборонено. Зверніться до адміністратора.", nil)
	}
	_ = h.answerCallback(callback.ID, "Готово")

	data := callback.Data
	switch {
	case data == "nav:main":
		h.sessions.ClearState(user.UserID)
		return h.sendWelcome(callback.Message.Chat.ID, user)
	case data == "admin:add_product":
		if !hasAnyRole(user.Role, models.RoleAdmin) {
			return h.sendText(callback.Message.Chat.ID, "Адмін-панель доступна лише адміністратору.", nil)
		}
		h.sessions.ResetDraft(user.UserID)
		h.sessions.SetState(user.UserID, stateAwaitProductName)
		return h.sendText(callback.Message.Chat.ID, "Введіть назву нового товару українською мовою.", nil)
	case data == "admin:list_products":
		return h.sendStocks(ctx, callback.Message.Chat.ID, true)
	case data == "admin:edit_product", data == "admin:replenish_stock":
		return h.sendAdminProductPicker(ctx, callback.Message.Chat.ID, data == "admin:replenish_stock")
	case strings.HasPrefix(data, "admin:category:"):
		s := h.sessions.get(user.UserID)
		s.DraftProduct.Category = strings.TrimPrefix(data, "admin:category:")
		s.State = stateAwaitProductCost
		return h.sendText(callback.Message.Chat.ID, "Введіть собівартість товару.", nil)
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
		return h.sendText(callback.Message.Chat.ID, "Введіть кількість, на яку потрібно поповнити залишок.", nil)
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
			return h.sendText(callback.Message.Chat.ID, "Введіть нову назву товару.", nil)
		case "cost":
			s.State = stateAwaitEditCost
			return h.sendText(callback.Message.Chat.ID, "Введіть нову собівартість товару.", nil)
		case "sell":
			s.State = stateAwaitEditSell
			return h.sendText(callback.Message.Chat.ID, "Введіть нову ціну продажу товару.", nil)
		case "stock":
			s.State = stateAwaitSetStock
			return h.sendText(callback.Message.Chat.ID, "Введіть новий фактичний залишок товару.", nil)
		default:
			return nil
		}
	case data == "cart:add_more":
		return h.sendCartAndCategories(callback.Message.Chat.ID, user.UserID)
	case data == "cart:show":
		return h.sendCartSummary(callback.Message.Chat.ID, user.UserID)
	case data == "cart:clear":
		return h.sendText(callback.Message.Chat.ID, "Очистити кошик?", keyboards.ConfirmClearCart())
	case data == "cart:confirm_clear":
		h.sessions.ClearCart(user.UserID)
		return h.sendText(callback.Message.Chat.ID, "Кошик очищено.", nil)
	case data == "cart:pay":
		cart := h.sessions.Cart(user.UserID)
		if len(cart) == 0 {
			return h.sendText(callback.Message.Chat.ID, "Кошик порожній. Додайте товари перед оплатою.", nil)
		}
		return h.sendText(callback.Message.Chat.ID, fmt.Sprintf("До сплати: %s\nОберіть спосіб оплати.", utils.FormatMoney(cartTotal(cart))), keyboards.PaymentMethods())
	case strings.HasPrefix(data, "cart:payment:"):
		paymentMethod := strings.TrimPrefix(data, "cart:payment:")
		s := h.sessions.get(user.UserID)
		s.PaymentMethod = paymentMethod
		paymentLabel := "Готівка"
		if paymentMethod == models.PaymentCard {
			paymentLabel = "Карта"
		}
		return h.sendText(callback.Message.Chat.ID, fmt.Sprintf("Підтвердити продаж на суму %s (%s)?", utils.FormatMoney(cartTotal(h.sessions.Cart(user.UserID))), paymentLabel), keyboards.ConfirmSale())
	case data == "cart:confirm_sale":
		s := h.sessions.get(user.UserID)
		sale, items, err := h.storage.RecordSale(ctx, user.UserID, s.PaymentMethod, s.Cart)
		if err != nil {
			return h.sendText(callback.Message.Chat.ID, fmt.Sprintf("Не вдалося провести продаж: %v", err), nil)
		}
		log.Printf("Продаж #%d: user=%d total=%.2f payment=%s items=%d", sale.ID, user.UserID, sale.Total, sale.PaymentMethod, len(items))
		h.sessions.ClearCart(user.UserID)
		h.sessions.ClearState(user.UserID)
		return h.sendText(callback.Message.Chat.ID, h.saleReceiptText(sale, items), replyKeyboard(user.Role))
	case strings.HasPrefix(data, "cart:category:"):
		category := strings.TrimPrefix(data, "cart:category:")
		return h.sendProductsForCategory(ctx, callback.Message.Chat.ID, category)
	case strings.HasPrefix(data, "cart:product:"):
		productID, err := strconv.ParseInt(strings.TrimPrefix(data, "cart:product:"), 10, 64)
		if err != nil {
			return err
		}
		product, err := h.storage.GetProduct(ctx, productID)
		if err != nil {
			return err
		}
		s := h.sessions.get(user.UserID)
		s.SelectedProductID = productID
		s.State = stateAwaitOrderQty
		return h.sendText(callback.Message.Chat.ID, fmt.Sprintf("Введіть кількість для товару «%s». Доступно: %s %s.", product.Name, utils.FormatQuantity(product.Stock), product.Unit), nil)
	case data == "report:today":
		return h.sendDailyReport(ctx, callback.Message.Chat.ID, time.Now())
	case data == "report:yesterday":
		return h.sendDailyReport(ctx, callback.Message.Chat.ID, time.Now().AddDate(0, 0, -1))
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

func (h *Handler) sendCartAndCategories(chatID, userID int64) error {
	text := "🧮 Калькулятор замовлення\n\nОберіть категорію товарів."
	cart := h.sessions.Cart(userID)
	if len(cart) > 0 {
		text = h.cartSummaryText(cart) + "\n\nОберіть категорію, щоб додати ще товар."
	}
	return h.sendText(chatID, text, keyboards.Categories("cart:category:"))
}

func (h *Handler) sendProductsForCategory(ctx context.Context, chatID int64, category string) error {
	products, err := h.storage.ListProductsByCategory(ctx, category)
	if err != nil {
		return err
	}
	if len(products) == 0 {
		return h.sendText(chatID, "У цій категорії поки немає активних товарів.", keyboards.Categories("cart:category:"))
	}
	return h.sendText(chatID, fmt.Sprintf("Категорія: %s\nОберіть товар.", category), keyboards.ProductButtons(products, "cart:product:", "cart:add_more"))
}

func (h *Handler) sendCartSummary(chatID, userID int64) error {
	cart := h.sessions.Cart(userID)
	if len(cart) == 0 {
		return h.sendText(chatID, "Кошик порожній. Додайте товари до замовлення.", keyboards.Categories("cart:category:"))
	}
	return h.sendText(chatID, h.cartSummaryText(cart), keyboards.CartActions())
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

func (h *Handler) sendMySalesSummary(ctx context.Context, chatID int64, user models.User) error {
	summary, err := h.storage.GetUserSalesSummary(ctx, user.UserID, time.Now())
	if err != nil {
		return err
	}
	text := fmt.Sprintf("📅 Ваші продажі за сьогодні\n\n💵 Готівка: %s\n💳 Карта: %s\n💰 Разом: %s\n🧾 Кількість чеків: %d", utils.FormatMoney(summary.CashTotal), utils.FormatMoney(summary.CardTotal), utils.FormatMoney(summary.GrandTotal), summary.Checks)
	return h.sendText(chatID, text, nil)
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

func (h *Handler) startPurchaseRequest(ctx context.Context, chatID, userID int64) error {
	products, err := h.storage.ListLowStockProducts(ctx, 5)
	if err != nil {
		return err
	}
	var builder strings.Builder
	builder.WriteString("🛒 Запит на закупку\n\n")
	if len(products) == 0 {
		builder.WriteString("Товарів з низьким залишком не знайдено.\n\n")
	} else {
		builder.WriteString("Низький залишок:\n")
		for _, product := range products {
			builder.WriteString(fmt.Sprintf("• %s — %s %s\n", product.Name, utils.FormatQuantity(product.Stock), product.Unit))
		}
		builder.WriteString("\n")
	}
	builder.WriteString("Надішліть текстовий запит адміністратору. Для скасування напишіть «Скасувати».\n")
	h.sessions.SetState(userID, stateAwaitPurchaseText)
	return h.sendText(chatID, strings.TrimSpace(builder.String()), nil)
}

func (h *Handler) forwardPurchaseRequest(ctx context.Context, user models.User, chatID int64, text string) error {
	admins, err := h.storage.ListAdmins(ctx)
	if err != nil {
		return err
	}
	messageText := fmt.Sprintf("🛒 Новий запит на закупку\n\nВід: %s (@%s, ID: %d)\n\n%s", user.FullName, user.Username, user.UserID, text)
	for _, admin := range admins {
		if err := h.sendText(admin.UserID, messageText, nil); err != nil {
			log.Printf("не вдалося відправити запит адміну %d: %v", admin.UserID, err)
		}
	}
	return h.sendText(chatID, "✅ Запит на закупку надіслано адміністратору.", nil)
}

func (h *Handler) sendDailyReport(ctx context.Context, chatID int64, date time.Time) error {
	report, err := h.storage.GetDailyReport(ctx, date)
	if err != nil {
		return err
	}
	return h.sendText(chatID, reports.FormatDailyReport(report), nil)
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
	return h.sendText(chatID, title, keyboards.ProductButtons(products, prefix, "admin:list_products"))
}

func (h *Handler) sendAdminProductCard(ctx context.Context, chatID, productID int64, notice string) error {
	product, err := h.storage.GetProduct(ctx, productID)
	if err != nil {
		return err
	}
	text := fmt.Sprintf("📦 %s\n\nКатегорія: %s\nСобівартість: %s\nЦіна продажу: %s\nОдиниця: %s\nЗалишок: %s %s", product.Name, product.Category, utils.FormatMoney(product.CostPrice), utils.FormatMoney(product.SellPrice), product.Unit, utils.FormatQuantity(product.Stock), product.Unit)
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
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("⬅️ До адмін-панелі", "nav:main")),
	)
	return h.sendText(chatID, text, &markup)
}

func (h *Handler) cartSummaryText(cart []models.OrderItem) string {
	var builder strings.Builder
	builder.WriteString("🧾 Поточне замовлення\n\n")
	for _, item := range cart {
		builder.WriteString(fmt.Sprintf("• %s — %s %s × %s = %s\n", item.Name, utils.FormatQuantity(item.Qty), item.Unit, utils.FormatMoney(item.SellPrice), utils.FormatMoney(item.Qty*item.SellPrice)))
	}
	builder.WriteString(fmt.Sprintf("\n💰 Разом: %s", utils.FormatMoney(cartTotal(cart))))
	return builder.String()
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

func (h *Handler) sendText(chatID int64, text string, markup any) error {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = ""
	if markup != nil {
		msg.ReplyMarkup = markup
	}
	_, err := h.bot.Send(msg)
	return err
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
