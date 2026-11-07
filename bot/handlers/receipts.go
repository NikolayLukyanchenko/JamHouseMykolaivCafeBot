package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/bot/keyboards"
	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/models"
	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/reports"
	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/utils"
)

// receiveStock finishes "📦 Поповнити залишки": adds the entered quantity,
// recalculates the cost as a weighted average and logs the receipt.
func (h *Handler) receiveStock(ctx context.Context, chatID int64, user models.User, unitCost float64) error {
	s := h.sessions.get(user.UserID)
	productID, qty := s.EditingProductID, s.ReceiveQty
	h.sessions.ClearState(user.UserID)
	s.ReceiveQty = 0
	if qty <= 0 {
		return h.sendAdminPanel(chatID)
	}
	r, err := h.storage.ReceiveStock(ctx, productID, qty, unitCost, user.UserID)
	if err != nil {
		return err
	}
	notice := fmt.Sprintf("Залишок поповнено: %s → %s %s\nСобівартість: %s → %s",
		utils.FormatQuantity(r.StockBefore), utils.FormatQuantity(r.StockBefore+r.Qty), r.Unit,
		utils.FormatMoney(r.CostBefore), utils.FormatMoney(r.CostAfter))
	if r.StockBefore > 0 && r.UnitCost != r.CostBefore {
		notice += fmt.Sprintf("\n(%s × %.2f + %s × %.2f) / %s",
			utils.FormatQuantity(r.StockBefore), r.CostBefore, utils.FormatQuantity(r.Qty), r.UnitCost, utils.FormatQuantity(r.StockBefore+r.Qty))
	}
	return h.sendAdminProductCard(ctx, chatID, productID, notice)
}

// sendReceiptLog shows the receipt log for a day: "admin:rcpt" opens today in
// a new message, "admin:rcpt:YYYY-MM-DD" updates the message in place.
func (h *Handler) sendReceiptLog(ctx context.Context, chatID int64, messageID int, data string) error {
	now := time.Now()
	day := now
	if raw, ok := strings.CutPrefix(data, "admin:rcpt:"); ok {
		parsed, err := parseDay(raw)
		if err != nil {
			return err
		}
		day = parsed
	} else {
		messageID = 0
	}
	receipts, err := h.storage.ListReceipts(ctx, day)
	if err != nil {
		return err
	}
	text := reports.FormatReceipts(day, now, receipts)
	markup := keyboards.ReceiptLogNav(day, now)
	if messageID == 0 {
		return h.sendText(chatID, text, &markup)
	}
	return h.showInPlace(chatID, messageID, text, &markup)
}
