package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	TelegramBotToken string
	DBPath           string
	AdminIDs         []int64
	TelegramDebug    bool
}

func Load() (Config, error) {
	// Load .env if present (local development). Ignored in production / Railway.
	if err := godotenv.Load(); err == nil {
		log.Println("[config] завантажено .env файл")
	}

	cfg := Config{
		TelegramBotToken: strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")),
		DBPath:           strings.TrimSpace(os.Getenv("DB_PATH")),
		TelegramDebug:    strings.ToLower(strings.TrimSpace(os.Getenv("TELEGRAM_DEBUG"))) == "true",
	}
	if cfg.DBPath == "" {
		cfg.DBPath = "data.db"
	}
	if cfg.TelegramBotToken == "" {
		return Config{}, fmt.Errorf("змінна TELEGRAM_BOT_TOKEN обов'язкова")
	}
	adminIDs, err := ParseAdminIDs(os.Getenv("ADMIN_IDS"))
	if err != nil {
		return Config{}, err
	}
	cfg.AdminIDs = adminIDs

	// Log startup summary without secrets.
	log.Printf("[config] DB_PATH=%s ADMIN_IDS_COUNT=%d TELEGRAM_DEBUG=%v",
		cfg.DBPath, len(cfg.AdminIDs), cfg.TelegramDebug)

	return cfg, nil
}

func ParseAdminIDs(raw string) ([]int64, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	ids := make([]int64, 0, len(parts))
	seen := make(map[int64]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("невірне значення в ADMIN_IDS: %q", part)
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, nil
}
