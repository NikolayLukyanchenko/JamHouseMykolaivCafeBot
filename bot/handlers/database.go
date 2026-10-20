package handlers

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// backupFileName is also the Turso database name: `turso db import` names the
// database after the file.
const backupFileName = "bot-jam-house-cafe.db"

// sendBackup sends a consistent copy of the local SQLite database as a file.
func (h *Handler) sendBackup(ctx context.Context, chatID int64) error {
	if h.storage.Remote() {
		return h.sendText(chatID, "Бот уже працює з Turso. Резервні копії Turso робить сам (Point-in-Time Recovery у панелі Turso).", nil)
	}
	stats, err := h.storage.Stats(ctx)
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "backup-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, backupFileName)
	if err := h.storage.Backup(ctx, path); err != nil {
		return h.sendText(chatID, fmt.Sprintf("⚠️ Не вдалося зробити копію бази: %v", err), nil)
	}
	doc := tgbotapi.NewDocument(chatID, tgbotapi.FilePath(path))
	doc.Caption = fmt.Sprintf("💾 Копія бази (SQLite, режим WAL)\n%s", stats)
	if _, err := h.bot.Send(doc); err != nil {
		log.Printf("не вдалося надіслати копію бази: %v", err)
		return h.sendText(chatID, fmt.Sprintf("⚠️ Не вдалося надіслати файл: %v", err), nil)
	}
	return nil
}

func (h *Handler) sendDBInfo(ctx context.Context, chatID int64) error {
	stats, err := h.storage.Stats(ctx)
	if err != nil {
		return err
	}
	backend := "SQLite-файл на сервері"
	if h.storage.Remote() {
		backend = "Turso"
	}
	return h.sendText(chatID, fmt.Sprintf("🗄 База даних: %s\n\nКористувачів: %d\nТоварів: %d\nЧеків: %d\nПозицій у чеках: %d\nПозицій закупки: %d",
		backend, stats.Users, stats.Products, stats.Sales, stats.SaleItems, stats.PurchaseItems), nil)
}
