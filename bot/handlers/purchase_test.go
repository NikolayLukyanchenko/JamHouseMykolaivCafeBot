package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/storage"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type sentMessage struct {
	Method string
	ChatID string
	Text   string
	Markup string
}

// fakeTelegram is a minimal Bot API server. Chat IDs in unreachable behave
// like users who never pressed /start.
type fakeTelegram struct {
	mu          sync.Mutex
	sent        []sentMessage
	unreachable map[string]bool
	nextID      int
}

func (f *fakeTelegram) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	method := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
	f.mu.Lock()
	defer f.mu.Unlock()
	switch method {
	case "getMe":
		fmt.Fprint(w, `{"ok":true,"result":{"id":999,"is_bot":true,"first_name":"Bot","username":"test_bot"}}`)
	case "sendMessage", "editMessageText":
		chatID := r.FormValue("chat_id")
		if f.unreachable[chatID] {
			fmt.Fprint(w, `{"ok":false,"error_code":400,"description":"Bad Request: chat not found"}`)
			return
		}
		f.sent = append(f.sent, sentMessage{Method: method, ChatID: chatID, Text: r.FormValue("text"), Markup: r.FormValue("reply_markup")})
		f.nextID++
		resp, _ := json.Marshal(map[string]any{"ok": true, "result": map[string]any{"message_id": f.nextID, "date": 0, "chat": map[string]any{"id": json.Number(chatID), "type": "private"}}})
		w.Write(resp)
	default:
		fmt.Fprint(w, `{"ok":true,"result":true}`)
	}
}

func (f *fakeTelegram) last(t *testing.T) sentMessage {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.sent) == 0 {
		t.Fatal("nothing was sent")
	}
	return f.sent[len(f.sent)-1]
}

func (f *fakeTelegram) all() []sentMessage {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]sentMessage(nil), f.sent...)
}

func newTestHandler(t *testing.T) (*Handler, *fakeTelegram) {
	t.Helper()
	ctx := context.Background()
	store, err := storage.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	if err := store.Init(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.SeedAdmins(ctx, []int64{1, 2}); err != nil {
		t.Fatal(err)
	}
	fake := &fakeTelegram{unreachable: map[string]bool{"2": true}}
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	bot, err := tgbotapi.NewBotAPIWithAPIEndpoint("token", srv.URL+"/bot%s/%s")
	if err != nil {
		t.Fatal(err)
	}
	return New(bot, store), fake
}

func sendTextUpdate(t *testing.T, h *Handler, text string) {
	t.Helper()
	update := tgbotapi.Update{Message: &tgbotapi.Message{MessageID: 1, From: &tgbotapi.User{ID: 1, FirstName: "Ольга"}, Chat: &tgbotapi.Chat{ID: 1}, Text: text}}
	if err := h.HandleUpdate(context.Background(), update); err != nil {
		t.Fatalf("message %q: %v", text, err)
	}
}

func sendCallbackUpdate(t *testing.T, h *Handler, data string) {
	t.Helper()
	update := tgbotapi.Update{CallbackQuery: &tgbotapi.CallbackQuery{ID: "cb", From: &tgbotapi.User{ID: 1, FirstName: "Ольга"}, Message: &tgbotapi.Message{MessageID: 50, Chat: &tgbotapi.Chat{ID: 1}}, Data: data}}
	if err := h.HandleUpdate(context.Background(), update); err != nil {
		t.Fatalf("callback %q: %v", data, err)
	}
}

func TestPurchaseRequestFlow(t *testing.T) {
	h, fake := newTestHandler(t)

	sendTextUpdate(t, h, "Замовити закупку")
	if got := fake.last(t).Text; !strings.Contains(got, "Список позицій для закупки порожній") {
		t.Fatalf("expected empty-list hint, got %q", got)
	}

	// Admin fills the purchase list, several items at once.
	sendCallbackUpdate(t, h, "admin:pitem_add")
	if got := fake.last(t).Markup; !strings.Contains(got, "nav:cancel") {
		t.Fatalf("prompt must have a cancel button, markup %q", got)
	}
	sendTextUpdate(t, h, "Молоко, л\nЦукор, кг")
	if got := fake.last(t).Text; !strings.Contains(got, "Додано позицій: 2") {
		t.Fatalf("unexpected reply after adding items: %q", got)
	}

	// Staff taps items instead of typing.
	sendTextUpdate(t, h, "Замовити закупку")
	if got := fake.last(t).Markup; !strings.Contains(got, "pur:add:1") || !strings.Contains(got, "pur:add:2") {
		t.Fatalf("expected item buttons, markup %q", got)
	}
	for _, data := range []string{"pur:add:1", "pur:add:1", "pur:add:2", "pur:sub:2"} {
		sendCallbackUpdate(t, h, data)
	}
	screen := fake.last(t)
	if screen.Method != "editMessageText" || !strings.Contains(screen.Text, "Молоко — 2 л") || strings.Contains(screen.Text, "Цукор —") {
		t.Fatalf("unexpected purchase screen: %+v", screen)
	}

	sendCallbackUpdate(t, h, "pur:comment")
	sendTextUpdate(t, h, "Терміново до п'ятниці")
	if got := fake.last(t).Text; !strings.Contains(got, "Терміново до п'ятниці") || !strings.Contains(got, "Молоко — 2 л") {
		t.Fatalf("comment must keep selected items, got %q", got)
	}

	sendCallbackUpdate(t, h, "pur:send")
	var request, result *sentMessage
	for _, m := range fake.all() {
		m := m
		if m.Method == "sendMessage" && strings.HasPrefix(m.Text, "🛒 Нова заявка на закупку") {
			request = &m
		}
		if strings.HasPrefix(m.Text, "✅ Заявку надіслано") {
			result = &m
		}
	}
	if request == nil || !strings.Contains(request.Text, "Молоко — 2 л") || !strings.Contains(request.Text, "Терміново") {
		t.Fatalf("admin did not receive the request: %+v", request)
	}
	if result == nil || !strings.Contains(result.Text, "Не доставлено") || !strings.Contains(result.Text, "/start") {
		t.Fatalf("requester must see which admin was unreachable: %+v", result)
	}
}

func TestMenuButtonInterruptsPrompt(t *testing.T) {
	h, fake := newTestHandler(t)

	sendCallbackUpdate(t, h, "admin:add_product")
	sendTextUpdate(t, h, "Залишки товарів")
	if got := fake.last(t).Text; got != "Список товарів порожній." {
		t.Fatalf("menu button must not be taken as product name, got %q", got)
	}

	sendCallbackUpdate(t, h, "admin:add_product")
	sendCallbackUpdate(t, h, "nav:cancel")
	if got := fake.last(t).Text; got != "Дію скасовано." {
		t.Fatalf("unexpected cancel reply %q", got)
	}
	if state := h.sessions.get(1).State; state != stateNone {
		t.Fatalf("state must be cleared, got %q", state)
	}
}
