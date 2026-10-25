package keyboards

import (
	"testing"
	"time"

	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/models"
)

func TestReportCalendar(t *testing.T) {
	today := time.Date(2026, 10, 2, 15, 0, 0, 0, time.UTC)
	kb := ReportCalendar(today, map[string]bool{"2026-10-01": true}, today)

	// title, weekdays, 5 weeks (Oct 2026 starts on Thursday), nav, home
	if len(kb.InlineKeyboard) != 9 {
		t.Fatalf("expected 9 rows, got %d", len(kb.InlineKeyboard))
	}
	week1 := kb.InlineKeyboard[2]
	if week1[2].Text != " " || week1[3].Text != "1•" || *week1[3].CallbackData != "report:day:2026-10-01" {
		t.Fatalf("1 Oct must be Thursday and marked: %+v", week1)
	}
	if week1[4].Text != "2" || week1[5].Text != "·" || *week1[5].CallbackData != "noop" {
		t.Fatalf("today selectable, future days disabled: %+v", week1)
	}
	if nav := kb.InlineKeyboard[7]; len(nav) != 1 || *nav[0].CallbackData != "report:cal:2026-09" {
		t.Fatalf("current month must have only a back button: %+v", nav)
	}

	past := ReportCalendar(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), nil, today)
	nav := past.InlineKeyboard[len(past.InlineKeyboard)-2]
	if len(nav) != 2 || *nav[1].CallbackData != "report:cal:2026-10" {
		t.Fatalf("past month must link forward: %+v", nav)
	}
}

func TestDayNavRow(t *testing.T) {
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	today := dayNavRow("my:day:", now, now)
	if len(today) != 1 || *today[0].CallbackData != "my:day:2026-10-02" {
		t.Fatalf("today: only a back button expected: %+v", today)
	}
	yesterday := dayNavRow("my:day:", now.AddDate(0, 0, -1), now)
	if len(yesterday) != 2 || *yesterday[1].CallbackData != "my:day:2026-10-03" {
		t.Fatalf("yesterday: back + forward expected: %+v", yesterday)
	}
	old := dayNavRow("my:day:", now.AddDate(0, 0, -5), now)
	if len(old) != 3 || old[1].Text != "Сьогодні" || *old[2].CallbackData != "my:day:2026-09-29" {
		t.Fatalf("older day: back + today + forward expected: %+v", old)
	}
}

func TestPOSLayout(t *testing.T) {
	short := []models.Product{{ID: 1, Name: "Лате", SellPrice: 70, Stock: 5}, {ID: 2, Name: "Чай", SellPrice: 30, Stock: 5}, {ID: 3, Name: "Какао", SellPrice: 50, Stock: 0}}
	kb := POS(POSTabTop, short, nil, 0)
	if len(kb.InlineKeyboard) != 2 || len(kb.InlineKeyboard[1]) != 3 {
		t.Fatalf("short names: tabs + one row of 3 expected, got %+v", kb.InlineKeyboard)
	}
	if kb.InlineKeyboard[1][2].Text != "⛔ Какао" {
		t.Fatalf("sold-out product must be marked: %q", kb.InlineKeyboard[1][2].Text)
	}

	long := append(short, models.Product{ID: 4, Name: "Капучино на кокосовому", SellPrice: 85, Stock: 5})
	cart := []models.OrderItem{{ProductID: 1, Name: "Лате", Qty: 2, SellPrice: 70}}
	kb = POS("0", long, cart, 140)
	if len(kb.InlineKeyboard[1]) != 2 {
		t.Fatalf("long names must use 2 columns: %+v", kb.InlineKeyboard[1])
	}
	if kb.InlineKeyboard[1][0].Text != "Лате 70 ·2" || kb.InlineKeyboard[0][1].Text != "▸🥤 Напої" {
		t.Fatalf("unexpected labels: %q %q", kb.InlineKeyboard[1][0].Text, kb.InlineKeyboard[0][1].Text)
	}
	last := kb.InlineKeyboard[len(kb.InlineKeyboard)-1]
	if *last[0].CallbackData != "pos:pay:cash" || last[1].Text != "💳 Карта 140" {
		t.Fatalf("unexpected payment row: %+v", last)
	}
}
