package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("LISTEN_ADDR", "")
	t.Setenv("STATE_FILE", "")
	t.Setenv("RETENTION_INTERVAL", "not-a-duration")
	cfg := Load()
	if cfg.ListenAddr != ":8080" || cfg.StateFile != "./data/state.json" || cfg.RetentionInterval <= 0 {
		t.Fatalf("unexpected defaults: %#v", cfg)
	}
}
