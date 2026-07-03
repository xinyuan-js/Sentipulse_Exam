package config

import "testing"

func TestParseCLIRejectsNonPositiveInterval(t *testing.T) {
	if _, err := ParseCLI([]string{"-interval", "0s"}); err == nil {
		t.Fatal("expected non-positive interval error")
	}
}

func TestParseCLIRejectsNonPositiveExecutionTimeout(t *testing.T) {
	if _, err := ParseCLI([]string{"-execution-timeout", "0s"}); err == nil {
		t.Fatal("expected non-positive execution-timeout error")
	}
}

func TestParseCLIParsesRuntimeToggles(t *testing.T) {
	cfg, err := ParseCLI([]string{"-enable", "a,b", "-disable", " c ,, d "})
	if err != nil {
		t.Fatalf("parse cli: %v", err)
	}
	if got, want := len(cfg.EnableIDs), 2; got != want {
		t.Fatalf("enable id count = %d, want %d", got, want)
	}
	if got, want := len(cfg.DisableIDs), 2; got != want {
		t.Fatalf("disable id count = %d, want %d", got, want)
	}
}
