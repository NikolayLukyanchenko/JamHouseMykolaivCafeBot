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

func TestLoadTurso(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "x")
	t.Setenv("TURSO_DATABASE_URL", "libsql://bot-jam-house-cafe-org.turso.io")
	t.Setenv("TURSO_AUTH_TOKEN", "")
	if _, err := Load(); err == nil {
		t.Fatal("libsql:// URL without token must be rejected")
	}
	t.Setenv("TURSO_AUTH_TOKEN", "secret")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TursoURL != "libsql://bot-jam-house-cafe-org.turso.io" || cfg.TursoAuthToken != "secret" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}
