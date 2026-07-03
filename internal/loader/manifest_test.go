package loader

import (
	"strings"
	"testing"
)

func TestParseManifestRejectsTrailingJSON(t *testing.T) {
	content := []byte(`{
		"id": "demo",
		"name": "Demo",
		"version": "1.0.0",
		"type": "test-process",
		"command": "noop"
	} {}`)

	_, err := parseManifest("plugins/demo/plugin.json", content)
	if err == nil {
		t.Fatal("expected trailing JSON error")
	}
	if !strings.Contains(err.Error(), "multiple JSON values") {
		t.Fatalf("error = %v, want multiple JSON values", err)
	}
}

func TestParseManifestRejectsInvalidDependencyVersion(t *testing.T) {
	content := []byte(`{
		"id": "demo",
		"name": "Demo",
		"version": "1.0.0",
		"type": "test-process",
		"command": "noop",
		"depends_on": [
			{"id": "base", "version": ">=banana"}
		]
	}`)

	_, err := parseManifest("plugins/demo/plugin.json", content)
	if err == nil {
		t.Fatal("expected invalid dependency version error")
	}
	if !strings.Contains(err.Error(), "invalid version segment") {
		t.Fatalf("error = %v, want invalid version segment", err)
	}
}
