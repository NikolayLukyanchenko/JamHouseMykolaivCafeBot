package utils

import "testing"

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
