package utils

import (
	"fmt"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func FormatMoney(value float64) string {
	return fmt.Sprintf("%.2f грн", value)
}

func FormatQuantity(value float64) string {
	formatted := strconv.FormatFloat(value, 'f', -1, 64)
	formatted = strings.TrimSuffix(formatted, ".0")
	return formatted
}

func ParsePositiveFloat(raw string) (float64, error) {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, ",", "."))
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("введіть число більше нуля")
	}
	return value, nil
}

func ParseNonNegativeFloat(raw string) (float64, error) {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, ",", "."))
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value < 0 {
		return 0, fmt.Errorf("введіть число не менше нуля")
	}
	return value, nil
}

func TelegramFullName(user *tgbotapi.User) string {
	if user == nil {
		return ""
	}
	fullName := strings.TrimSpace(strings.TrimSpace(user.FirstName + " " + user.LastName))
	if fullName == "" {
		return user.UserName
	}
	return fullName
}

func CategoryEmoji(category string) string {
	switch category {
	case "Напої":
		return "🥤"
	case "Їжа":
		return "🍽"
	case "Солодощі":
		return "🍰"
	default:
		return "•"
	}
}

// ParsePurchaseItemLine splits "Назва, одиниця" into name and optional unit.
// The unit is everything after the last comma, so names may contain commas
// only when a unit is given.
func ParsePurchaseItemLine(raw string) (name, unit string, ok bool) {
	raw = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(raw), "•-*"))
	if idx := strings.LastIndex(raw, ","); idx >= 0 {
		name = strings.TrimSpace(raw[:idx])
		unit = strings.TrimSpace(raw[idx+1:])
	} else {
		name = raw
	}
	if name == "" {
		return "", "", false
	}
	return name, unit, true
}
