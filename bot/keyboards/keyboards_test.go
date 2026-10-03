package keyboards

import (
	"testing"
	"time"
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
