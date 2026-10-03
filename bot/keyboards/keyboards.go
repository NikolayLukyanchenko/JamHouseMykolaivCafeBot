package keyboards

import (
	"fmt"
	"time"

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

func ReportPeriods(now time.Time) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Сьогодні", "report:today"),
			tgbotapi.NewInlineKeyboardButtonData("Вчора", "report:yesterday"),
		),
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("📅 Інша дата", "report:cal:"+now.Format("2006-01"))),
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("🏠 Головне меню", "nav:main")),
	)
}

// dayNavRow is "◀️ 02.10 | Сьогодні | 04.10 ▶️" for browsing reports day by
// day. The forward button is hidden on today; "Сьогодні" only on past days.
func dayNavRow(prefix string, day, now time.Time) []tgbotapi.InlineKeyboardButton {
	prev := day.AddDate(0, 0, -1)
	row := []tgbotapi.InlineKeyboardButton{tgbotapi.NewInlineKeyboardButtonData("◀️ "+prev.Format("02.01"), prefix+prev.Format("2006-01-02"))}
	if day.Format("2006-01-02") < now.Format("2006-01-02") {
		next := day.AddDate(0, 0, 1)
		if next.Format("2006-01-02") != now.Format("2006-01-02") {
			row = append(row, tgbotapi.NewInlineKeyboardButtonData("Сьогодні", prefix+now.Format("2006-01-02")))
		}
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(next.Format("02.01")+" ▶️", prefix+next.Format("2006-01-02")))
	}
	return row
}

// DailyReportActions is attached to a daily report.
func DailyReportActions(day, now time.Time) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		dayNavRow("report:nav:", day, now),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🧾 Чеки за день", "report:checks:"+day.Format("2006-01-02")),
			tgbotapi.NewInlineKeyboardButtonData("📅 Календар", "report:cal:"+day.Format("2006-01")),
		),
	)
}

// MySalesActions is attached to "Мої продажі".
func MySalesActions(day, now time.Time) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(dayNavRow("my:day:", day, now))
}

var monthNames = [...]string{"Січень", "Лютий", "Березень", "Квітень", "Травень", "Червень", "Липень", "Серпень", "Вересень", "Жовтень", "Листопад", "Грудень"}

// MonthTitle renders e.g. "Жовтень 2026".
func MonthTitle(month time.Time) string {
	return fmt.Sprintf("%s %d", monthNames[month.Month()-1], month.Year())
}

// ReportCalendar renders a Monday-first month grid. Days with sales are
// marked with "•"; future days are not selectable.
func ReportCalendar(month time.Time, salesDays map[string]bool, today time.Time) tgbotapi.InlineKeyboardMarkup {
	first := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, today.Location())
	todayKey := today.Format("2006-01-02")
	noop := func(label string) tgbotapi.InlineKeyboardButton {
		return tgbotapi.NewInlineKeyboardButtonData(label, "noop")
	}

	rows := [][]tgbotapi.InlineKeyboardButton{
		{noop(MonthTitle(first))},
		{noop("Пн"), noop("Вт"), noop("Ср"), noop("Чт"), noop("Пт"), noop("Сб"), noop("Нд")},
	}
	week := make([]tgbotapi.InlineKeyboardButton, 0, 7)
	for i := 0; i < (int(first.Weekday())+6)%7; i++ {
		week = append(week, noop(" "))
	}
	for day := first; day.Month() == first.Month(); day = day.AddDate(0, 0, 1) {
		key := day.Format("2006-01-02")
		switch {
		case key > todayKey:
			week = append(week, noop("·"))
		case salesDays[key]:
			week = append(week, tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("%d•", day.Day()), "report:day:"+key))
		default:
			week = append(week, tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("%d", day.Day()), "report:day:"+key))
		}
		if len(week) == 7 {
			rows = append(rows, week)
			week = make([]tgbotapi.InlineKeyboardButton, 0, 7)
		}
	}
	if len(week) > 0 {
		for len(week) < 7 {
			week = append(week, noop(" "))
		}
		rows = append(rows, week)
	}

	nav := []tgbotapi.InlineKeyboardButton{tgbotapi.NewInlineKeyboardButtonData("◀️ "+MonthTitle(first.AddDate(0, -1, 0)), "report:cal:"+first.AddDate(0, -1, 0).Format("2006-01"))}
	if next := first.AddDate(0, 1, 0); next.Format("2006-01") <= today.Format("2006-01") {
		nav = append(nav, tgbotapi.NewInlineKeyboardButtonData(MonthTitle(next)+" ▶️", "report:cal:"+next.Format("2006-01")))
	}
	rows = append(rows, nav, []tgbotapi.InlineKeyboardButton{tgbotapi.NewInlineKeyboardButtonData("🏠 Головне меню", "nav:main")})
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
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

func PurchaseItemsAdmin(items []models.PurchaseItem, groupBound bool) tgbotapi.InlineKeyboardMarkup {
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
	)
	if groupBound {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("🔕 Відв'язати групу для заявок", "admin:pchat_unbind")))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("⬅️ До адмін-панелі", "admin:panel")))
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
