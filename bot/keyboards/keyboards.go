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

func int64ToString(value int64) string {
	return fmt.Sprintf("%d", value)
}
