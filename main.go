package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/bot/handlers"
	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/config"
	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/storage"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("не вдалося завантажити конфігурацію: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	store, err := storage.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("не вдалося відкрити базу даних: %v", err)
	}
	defer store.Close()

	if err := store.Init(ctx); err != nil {
		log.Fatalf("не вдалося ініціалізувати базу даних: %v", err)
	}

	if err := store.SeedAdmins(ctx, cfg.AdminIDs); err != nil {
		log.Fatalf("не вдалося додати адміністраторів: %v", err)
	}

	bot, err := tgbotapi.NewBotAPI(cfg.TelegramBotToken)
	if err != nil {
		log.Fatalf("не вдалося створити Telegram-бота: %v", err)
	}

	log.Printf("бот %s успішно запущений", bot.Self.UserName)

	h := handlers.New(bot, store)
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30

	updates := bot.GetUpdatesChan(u)
	defer bot.StopReceivingUpdates()

	for {
		select {
		case <-ctx.Done():
			log.Println("отримано сигнал завершення, бот завершує роботу")
			return
		case update, ok := <-updates:
			if !ok {
				log.Println("канал оновлень закрито")
				return
			}
			if err := h.HandleUpdate(ctx, update); err != nil {
				log.Printf("помилка обробки оновлення: %v", err)
			}
		}
	}
}
