package handlers

import (
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func sendCommand(t *testing.T, h *Handler, command string) {
	t.Helper()
	sendTextUpdateWithEntities(t, h, command, []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: len(command)}})
}

func TestBackupAndDBInfoCommands(t *testing.T) {
	h, fake := newTestHandler(t)

	sendCommand(t, h, "/dbinfo")
	if got := fake.last(t).Text; !strings.Contains(got, "SQLite-файл") || !strings.Contains(got, "Користувачів: 2") {
		t.Fatalf("unexpected dbinfo: %q", got)
	}

	sendCommand(t, h, "/backup")
	doc := fake.last(t)
	if doc.Method != "sendDocument" || doc.ChatID != "1" || !strings.Contains(doc.Text, "users=2") || doc.Markup == "0" || doc.Markup == "" {
		t.Fatalf("backup file was not sent: %+v", doc)
	}
}
