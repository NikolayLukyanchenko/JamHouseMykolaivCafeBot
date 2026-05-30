package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	TelegramBotToken string
	DBPath           string
	AdminIDs         []int64
}

func Load() (Config, error) {
	_ = godotenv.Load()

	cfg := Config{
		TelegramBotToken: strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")),
		DBPath:           strings.TrimSpace(os.Getenv("DB_PATH")),
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
