package keyboards

import (
	"fmt"

	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/models"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func MainMenu(role string) tgbotapi.ReplyKeyboardMarkup {
	rows := [][]tgbotapi.KeyboardButton{
		{tgbotapi.NewKeyboardButton("Записати продаж"), tgbotapi.NewKeyboardButton("Калькулятор замовлення")},
		{tgbotapi.NewKeyboardButton("Меню для клієнтів"), tgbotapi.NewKeyboardButton("Мої продажі за сьогодні")},
		{tgbotapi.NewKeyboardButton("Залишки товарів"), tgbotapi.NewKeyboardButton("Замовити закупку")},
	}
	if role == models.RoleAdmin {
		rows = append(rows, []tgbotapi.KeyboardButton{tgbotapi.NewKeyboardButton("Відправити звіт")})
	}
	menu := tgbotapi.NewReplyKeyboard(rows...)
	menu.ResizeKeyboard = true
	menu.OneTimeKeyboard = false
	return menu
}

func AdminPanel() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➕ Додати товар", "admin:add_product"),
			tgbotapi.NewInlineKeyboardButtonData("✏️ Редагувати товар", "admin:edit_product"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📦 Поповнити залишки", "admin:replenish_stock"),
			tgbotapi.NewInlineKeyboardButtonData("📋 Список товарів", "admin:list_products"),
		),
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("🛒 Позиції для закупки", "admin:pitems")),
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("🏠 Головне меню", "nav:main")),
	)
}

func Categories(prefix string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🥤 Напої", prefix+models.CategoryDrinks),
			tgbotapi.NewInlineKeyboardButtonData("🍽 Їжа", prefix+models.CategoryFood),
		),
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("🍰 Солодощі", prefix+models.CategorySweets)),
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("🏠 Головне меню", "nav:main")),
	)
}

func ProductButtons(products []models.Product, prefix string, backData string) tgbotapi.InlineKeyboardMarkup {
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(products)+2)
	for _, product := range products {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(product.Name, prefix+int64ToString(product.ID))))
	}
	rows = append(rows,
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("⬅️ Назад", backData)),
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("🏠 Головне меню", "nav:main")),
	)
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func CartActions() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("➕ Додати ще", "cart:add_more")),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🗑 Очистити", "cart:clear"),
			tgbotapi.NewInlineKeyboardButtonData("✅ Оплатити", "cart:pay"),
		),
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("🏠 Головне меню", "nav:main")),
	)
}

func ConfirmClearCart() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Так, очистити", "cart:confirm_clear"),
			tgbotapi.NewInlineKeyboardButtonData("Ні, повернутись", "cart:show"),
		),
	)
}

func PaymentMethods() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💵 Готівка", "cart:payment:cash"),
			tgbotapi.NewInlineKeyboardButtonData("💳 Карта", "cart:payment:card"),
		),
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("⬅️ Назад", "cart:show")),
	)
}

func ConfirmSale() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Підтвердити", "cart:confirm_sale"),
			tgbotapi.NewInlineKeyboardButtonData("Скасувати", "cart:show"),
		),
	)
}

func ReportPeriods() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Сьогодні", "report:today"),
			tgbotapi.NewInlineKeyboardButtonData("Вчора", "report:yesterday"),
		),
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("🏠 Головне меню", "nav:main")),
	)
}

// Cancel is attached to every prompt that waits for typed input.
func Cancel() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("❌ Скасувати", "nav:cancel")),
	)
}

// PurchaseRequest renders the tap-to-select purchase list. Selected items get
// ➖/➕ buttons around them to adjust the quantity.
func PurchaseRequest(items []models.PurchaseItem, qty map[int64]int) tgbotapi.InlineKeyboardMarkup {
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(items)+3)
	for _, item := range items {
		id := int64ToString(item.ID)
		n := qty[item.ID]
		if n <= 0 {
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(item.Name, "pur:add:"+id)))
			continue
		}
		label := fmt.Sprintf("✅ %s — %d", item.Name, n)
		if item.Unit != "" {
			label += " " + item.Unit
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➖", "pur:sub:"+id),
			tgbotapi.NewInlineKeyboardButtonData(label, "pur:add:"+id),
			tgbotapi.NewInlineKeyboardButtonData("➕", "pur:add:"+id),
		))
	}
	rows = append(rows,
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("📝 Додати коментар", "pur:comment")),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Скасувати", "pur:cancel"),
			tgbotapi.NewInlineKeyboardButtonData("📤 Надіслати", "pur:send"),
		),
	)
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func PurchaseItemsAdmin(items []models.PurchaseItem) tgbotapi.InlineKeyboardMarkup {
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(items)+2)
	for _, item := range items {
		label := item.Name
		if item.Unit != "" {
			label += ", " + item.Unit
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(label, "admin:pitem:"+int64ToString(item.ID))))
	}
	rows = append(rows,
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("➕ Додати позиції", "admin:pitem_add")),
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("⬅️ До адмін-панелі", "admin:panel")),
	)
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func PurchaseItemCard(itemID int64) tgbotapi.InlineKeyboardMarkup {
	id := int64ToString(itemID)
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✏️ Змінити", "admin:pitem_edit:"+id),
			tgbotapi.NewInlineKeyboardButtonData("🗑 Видалити", "admin:pitem_del:"+id),
		),
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("⬅️ До списку", "admin:pitems")),
	)
}

func ConfirmDeletePurchaseItem(itemID int64) tgbotapi.InlineKeyboardMarkup {
	id := int64ToString(itemID)
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Так, видалити", "admin:pitem_delok:"+id),
			tgbotapi.NewInlineKeyboardButtonData("Ні", "admin:pitem:"+id),
		),
	)
}

func int64ToString(value int64) string {
	return fmt.Sprintf("%d", value)
}
