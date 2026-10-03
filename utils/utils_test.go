package utils

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestParsePurchaseItemLine(t *testing.T) {
	cases := []struct {
		raw, name, unit string
		ok              bool
	}{
		{"Молоко, л", "Молоко", "л", true},
		{"  Вершки 33%  ", "Вершки 33%", "", true},
		{"- Стаканчики 250 мл, уп", "Стаканчики 250 мл", "уп", true},
		{"Цукор,", "Цукор", "", true},
		{"   ", "", "", false},
		{", кг", "", "", false},
	}
	for _, c := range cases {
		name, unit, ok := ParsePurchaseItemLine(c.raw)
		if name != c.name || unit != c.unit || ok != c.ok {
			t.Errorf("ParsePurchaseItemLine(%q) = %q, %q, %v; want %q, %q, %v", c.raw, name, unit, ok, c.name, c.unit, c.ok)
		}
	}
}

func TestSplitMessage(t *testing.T) {
	if got := SplitMessage("коротко", 100); len(got) != 1 || got[0] != "коротко" {
		t.Fatalf("short text must stay whole: %q", got)
	}
	got := SplitMessage("рядок1\nрядок2\nрядок3", 26)
	if len(got) != 2 || got[0] != "рядок1\nрядок2" || got[1] != "рядок3" {
		t.Fatalf("must split on line breaks: %q", got)
	}
	got = SplitMessage("ааааа", 5) // 10 bytes, no line breaks
	for _, part := range got {
		if !utf8.ValidString(part) || len(part) > 5 {
			t.Fatalf("invalid part %q in %q", part, got)
		}
	}
	if strings.Join(got, "") != "ааааа" {
		t.Fatalf("text lost: %q", got)
	}
}
