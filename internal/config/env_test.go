package config

import (
	"errors"
	"testing"
)

// Under CAPIGO_NO_CONFIG=1 the store must not even resolve a path: a host may
// run the CLI with no HOME at all.
func TestNoConfigNeverResolvesPath(t *testing.T) {
	t.Setenv(NoConfigEnv, "1")
	orig := ConfigPath
	ConfigPath = func() (string, error) {
		t.Fatal("ConfigPath called under CAPIGO_NO_CONFIG=1")
		return "", nil
	}
	t.Cleanup(func() { ConfigPath = orig })

	cfg, err := Load()
	if err != nil || cfg == nil || len(cfg.Profiles) != 0 {
		t.Fatalf("Load = (%+v, %v), want an empty config", cfg, err)
	}
	if err := Save(&Config{Version: 1}); !errors.Is(err, ErrDisabled) {
		t.Fatalf("Save err = %v, want ErrDisabled", err)
	}
}

func TestNoConfigOnlyExactOne(t *testing.T) {
	for value, want := range map[string]bool{"1": true, "0": false, "": false, "true": false, " 1": false} {
		t.Setenv(NoConfigEnv, value)
		if got := NoConfig(); got != want {
			t.Errorf("NoConfig() with %q = %v, want %v", value, got, want)
		}
	}
}
