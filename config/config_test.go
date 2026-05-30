package config

import "testing"

func TestParseAdminIDs(t *testing.T) {
	ids, err := ParseAdminIDs("123, 456,123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 2 || ids[0] != 123 || ids[1] != 456 {
		t.Fatalf("unexpected ids: %#v", ids)
	}
}
