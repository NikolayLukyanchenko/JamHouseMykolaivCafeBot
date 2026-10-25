package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/bot/keyboards"
	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/models"
	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/storage"
	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/utils"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	// voidWindow is how long a seller can undo their own sale.
	voidWindow = 3 * time.Minute
	// popularPeriod and popularLimit define the "⭐ Топ" tab.
	popularPeriod = 14 * 24 * time.Hour
	popularLimit  = 9
)

// openPOS sends a fresh checkout screen on the "⭐ Топ" tab.
func (h *Handler) openPOS(ctx context.Context, chatID, userID int64) error {
	h.sessions.ClearState(userID)
	h.sessions.get(userID).PosTab = keyboards.POSTabTop
	text, markup, err := h.posScreen(ctx, userID)
	if err != nil {
		return err
	}
	return h.sendText(chatID, text, &markup)
}

func (h *Handler) refreshPOS(ctx context.Context, chatID int64, messageID int, userID int64) error {
	text, markup, err := h.posScreen(ctx, userID)
	if err != nil {
		return err
	}
	return h.editText(chatID, messageID, text, &markup)
}

func (h *Handler) posScreen(ctx context.Context, userID int64) (string, tgbotapi.InlineKeyboardMarkup, error) {
	tab := h.sessions.get(userID).PosTab
	var products []models.Product
	var bundles []models.Bundle
	var err error
	if idx, convErr := strconv.Atoi(tab); convErr == nil && idx >= 0 && idx < len(models.Categories()) {
		products, err = h.storage.ListProductsByCategory(ctx, models.Categories()[idx])
	} else {
		// Bundles are shortcuts for the most common orders, so they open
		// first, on the "⭐ Топ" tab.
		tab = keyboards.POSTabTop
		if bundles, err = h.storage.ListBundles(ctx); err == nil {
			products, err = h.storage.TopProducts(ctx, time.Now().Add(-popularPeriod), popularLimit)
		}
	}
	if err != nil {
		return "", tgbotapi.InlineKeyboardMarkup{}, err
	}

	cart := h.sessions.Cart(userID)
	var b strings.Builder
	b.WriteString("🧾 Замовлення\n\n")
	if len(cart) == 0 {
		b.WriteString("Натискайте на товари: кожне натискання додає 1 шт.\n➖ прибирає одну, оплата — одним натисканням.")
	} else {
		for _, item := range cart {
			b.WriteString(fmt.Sprintf("• %s ×%s — %s\n", item.Name, utils.FormatQuantity(item.Qty), utils.FormatMoney(item.Qty*item.SellPrice)))
		}
		b.WriteString(fmt.Sprintf("\n💰 Разом: %s", utils.FormatMoney(cartTotal(cart))))
	}
	if len(products) == 0 && len(bundles) == 0 {
		b.WriteString("\n\nУ цій вкладці поки немає товарів.")
	}
	return b.String(), keyboards.POS(tab, bundles, products, cart, cartTotal(cart)), nil
}

func (h *Handler) handlePOSCallback(ctx context.Context, user models.User, callback *tgbotapi.CallbackQuery) error {
	chatID := callback.Message.Chat.ID
	messageID := callback.Message.MessageID
	action, arg, _ := strings.Cut(strings.TrimPrefix(callback.Data, "pos:"), ":")

	switch action {
	case "tab":
		_ = h.answerCallback(callback.ID, "")
		h.sessions.get(user.UserID).PosTab = arg
		return h.refreshPOS(ctx, chatID, messageID, user.UserID)

	case "add", "sub":
		productID, err := strconv.ParseInt(arg, 10, 64)
		if err != nil {
			return err
		}
		product, err := h.storage.GetProduct(ctx, productID)
		if err != nil {
			return err
		}
		if action == "sub" {
			h.sessions.AdjustCart(user.UserID, productID, -1)
			_ = h.answerCallback(callback.ID, "− "+product.Name)
			return h.refreshPOS(ctx, chatID, messageID, user.UserID)
		}
		inCart := h.qtyInCart(user.UserID, productID)
		switch {
		case !product.IsActive || product.Stock < 1:
			return h.answerCallback(callback.ID, "⛔ «"+product.Name+"» немає в наявності")
		case product.Stock < inCart+1:
			return h.answerCallback(callback.ID, fmt.Sprintf("Залишок «%s» лише %s %s", product.Name, utils.FormatQuantity(product.Stock), product.Unit))
		}
		h.sessions.AddToCart(user.UserID, models.OrderItem{ProductID: product.ID, Name: product.Name, Category: product.Category, Qty: 1, SellPrice: product.SellPrice, CostPrice: product.CostPrice, Unit: product.Unit})
		_ = h.answerCallback(callback.ID, "+ "+product.Name)
		return h.refreshPOS(ctx, chatID, messageID, user.UserID)

	case "set":
		bundleID, err := strconv.ParseInt(arg, 10, 64)
		if err != nil {
			return err
		}
		bundle, err := h.storage.GetBundle(ctx, bundleID)
		if errors.Is(err, sql.ErrNoRows) {
			_ = h.answerCallback(callback.ID, "Набір видалено")
			return h.refreshPOS(ctx, chatID, messageID, user.UserID)
		}
		if err != nil {
			return err
		}
		if len(bundle.Items) == 0 {
			return h.answerCallback(callback.ID, "Набір порожній")
		}
		// All or nothing: check every component before adding any.
		for _, item := range bundle.Items {
			p := item.Product
			switch {
			case !p.IsActive || p.Stock < float64(item.Qty):
				return h.answerCallback(callback.ID, "⛔ «"+p.Name+"» немає в наявності")
			case p.Stock < h.qtyInCart(user.UserID, p.ID)+float64(item.Qty):
				return h.answerCallback(callback.ID, fmt.Sprintf("Залишок «%s» лише %s %s", p.Name, utils.FormatQuantity(p.Stock), p.Unit))
			}
		}
		for _, item := range bundle.Items {
			p := item.Product
			h.sessions.AddToCart(user.UserID, models.OrderItem{ProductID: p.ID, Name: p.Name, Category: p.Category, Qty: float64(item.Qty), SellPrice: p.SellPrice, CostPrice: p.CostPrice, Unit: p.Unit})
		}
		_ = h.answerCallback(callback.ID, "+ "+bundle.Name)
		return h.refreshPOS(ctx, chatID, messageID, user.UserID)

	case "clear":
		h.sessions.ClearCart(user.UserID)
		_ = h.answerCallback(callback.ID, "Замовлення очищено")
		return h.refreshPOS(ctx, chatID, messageID, user.UserID)

	case "pay":
		return h.payPOS(ctx, user, callback, arg)

	case "void":
		saleID, err := strconv.ParseInt(arg, 10, 64)
		if err != nil {
			return err
		}
		return h.voidSale(ctx, user, callback, saleID)

	default:
		return h.answerCallback(callback.ID, "")
	}
}

// payPOS records the sale in one tap. Mistakes are undone with the
// "↩️ Скасувати чек" button on the receipt instead of a confirmation step.
func (h *Handler) payPOS(ctx context.Context, user models.User, callback *tgbotapi.CallbackQuery, paymentMethod string) error {
	chatID := callback.Message.Chat.ID
	if paymentMethod != models.PaymentCash && paymentMethod != models.PaymentCard {
		return h.answerCallback(callback.ID, "")
	}
	cart := h.sessions.Cart(user.UserID)
	if len(cart) == 0 {
		return h.answerCallback(callback.ID, "Замовлення порожнє")
	}
	sale, items, err := h.storage.RecordSale(ctx, user.UserID, paymentMethod, cart)
	if err != nil {
		_ = h.answerAlert(callback.ID, fmt.Sprintf("Продаж не проведено: %v", err))
		return h.refreshPOS(ctx, chatID, callback.Message.MessageID, user.UserID)
	}
	log.Printf("Продаж #%d: user=%d total=%.2f payment=%s items=%d", sale.ID, user.UserID, sale.Total, sale.PaymentMethod, len(items))
	h.sessions.ClearCart(user.UserID)
	_ = h.answerCallback(callback.ID, fmt.Sprintf("✅ Чек #%d", sale.ID))

	receipt := h.saleReceiptText(sale, items) + fmt.Sprintf("\n\n↩️ Скасувати можна протягом %d хв.", int(voidWindow.Minutes()))
	voidMarkup := keyboards.VoidSale(sale.ID)
	if err := h.editText(chatID, callback.Message.MessageID, receipt, &voidMarkup); err != nil {
		log.Printf("не вдалося показати чек #%d: %v", sale.ID, err)
	}
	// The next customer starts right away on a fresh screen.
	return h.openPOS(ctx, chatID, user.UserID)
}

func (h *Handler) voidSale(ctx context.Context, user models.User, callback *tgbotapi.CallbackQuery, saleID int64) error {
	isAdmin := hasAnyRole(user.Role, models.RoleAdmin)
	sale, err := h.storage.VoidSale(ctx, saleID, user.UserID, isAdmin, voidWindow, time.Now())
	switch {
	case errors.Is(err, storage.ErrVoidTooLate):
		return h.answerAlert(callback.ID, fmt.Sprintf("Минуло більше %d хв — скасувати чек може лише адміністратор.", int(voidWindow.Minutes())))
	case errors.Is(err, storage.ErrVoidForbidden), errors.Is(err, storage.ErrSaleNotFound):
		return h.answerAlert(callback.ID, err.Error())
	case err != nil:
		return err
	}
	log.Printf("Чек #%d скасовано: by=%d seller=%d total=%.2f", sale.ID, user.UserID, sale.UserID, sale.Total)
	_ = h.answerCallback(callback.ID, fmt.Sprintf("Чек #%d скасовано", sale.ID))
	return h.editText(callback.Message.Chat.ID, callback.Message.MessageID,
		fmt.Sprintf("❌ Чек #%d на %s скасовано. Товари повернуто на залишок.", sale.ID, utils.FormatMoney(sale.Total)), nil)
}

func (h *Handler) answerAlert(callbackID, text string) error {
	_, err := h.bot.Request(tgbotapi.NewCallbackWithAlert(callbackID, text))
	return err
}
