package keyboards

import (
	"fmt"
	"strconv"
	"unicode/utf8"

	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/models"
	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/utils"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// POSTabTop is the "⭐ Популярне" tab; category tabs use their index in
// models.Categories() so callback data stays short and ASCII.
const POSTabTop = "top"

// longNameRunes switches the product grid from 3 to 2 columns so names are
// not cut off on phones.
const longNameRunes = 12

// POS renders the one-message checkout screen: category tabs, a product grid
// where each tap adds one item, ➖ buttons for what is in the order, and
// one-tap payment buttons.
func POS(activeTab string, products []models.Product, cart []models.OrderItem, total float64) tgbotapi.InlineKeyboardMarkup {
	rows := [][]tgbotapi.InlineKeyboardButton{posTabs(activeTab)}

	inCart := make(map[int64]int, len(cart))
	for _, item := range cart {
		inCart[item.ProductID] = int(item.Qty)
	}
	perRow := 3
	for _, p := range products {
		if utf8.RuneCountInString(p.Name) > longNameRunes {
			perRow = 2
			break
		}
	}
	var row []tgbotapi.InlineKeyboardButton
	for _, p := range products {
		label := fmt.Sprintf("%s %s", p.Name, utils.FormatQuantity(p.SellPrice))
		if p.Stock < 1 {
			label = "⛔ " + p.Name
		}
		if n := inCart[p.ID]; n > 0 {
			label += fmt.Sprintf(" ·%d", n)
		}
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(label, "pos:add:"+strconv.FormatInt(p.ID, 10)))
		if len(row) == perRow {
			rows = append(rows, row)
			row = nil
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}

	if len(cart) > 0 {
		row = nil
		for _, item := range cart {
			row = append(row, tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("➖ %s ·%d", item.Name, int(item.Qty)), "pos:sub:"+strconv.FormatInt(item.ProductID, 10)))
			if len(row) == 2 {
				rows = append(rows, row)
				row = nil
			}
		}
		row = append(row, tgbotapi.NewInlineKeyboardButtonData("🗑 Очистити", "pos:clear"))
		rows = append(rows, row)
		amount := utils.FormatQuantity(total)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💵 Готівка "+amount, "pos:pay:"+models.PaymentCash),
			tgbotapi.NewInlineKeyboardButtonData("💳 Карта "+amount, "pos:pay:"+models.PaymentCard),
		))
	}
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func posTabs(active string) []tgbotapi.InlineKeyboardButton {
	type tab struct{ key, label string }
	tabs := []tab{{POSTabTop, "⭐ Топ"}}
	for i, category := range models.Categories() {
		tabs = append(tabs, tab{strconv.Itoa(i), utils.CategoryEmoji(category) + " " + category})
	}
	buttons := make([]tgbotapi.InlineKeyboardButton, 0, len(tabs))
	for _, t := range tabs {
		label := t.label
		if t.key == active {
			label = "▸" + label
		}
		buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData(label, "pos:tab:"+t.key))
	}
	return buttons
}

// VoidSale is attached to a receipt for undoing a mistaken sale.
func VoidSale(saleID int64) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("↩️ Скасувати чек", "pos:void:"+strconv.FormatInt(saleID, 10)),
	))
}
