package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"sentipulse/plugin-execution/internal/model"
	"sentipulse/plugin-execution/internal/registry"
)

func TestManagerReloadLoadsAndUnloadsManifest(t *testing.T) {
	root := t.TempDir()
	pluginDir := filepath.Join(root, "demo")
	if err := os.Mkdir(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir plugin dir: %v", err)
	}

	manifestPath := filepath.Join(pluginDir, "plugin.json")
	content := []byte(`{
		"id": "demo",
		"name": "Demo",
		"version": "1.0.0",
		"type": "test-process",
		"enabled": true,
		"command": "noop",
		"timeout": "1s"
	}`)
	if err := os.WriteFile(manifestPath, content, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	manager := NewManager(root, ManagerOptions{ScanInterval: time.Second})
	if err := manager.Reload(context.Background()); err != nil {
		t.Fatalf("reload with manifest: %v", err)
	}
	requireStatus(t, manager.Registry(), "demo", model.StatusEnabled)

	if err := os.Remove(manifestPath); err != nil {
		t.Fatalf("remove manifest: %v", err)
	}
	if err := manager.Reload(context.Background()); err != nil {
		t.Fatalf("reload after remove: %v", err)
	}
	if states := manager.Registry().States(); len(states) != 0 {
		t.Fatalf("states after unload = %d, want 0", len(states))
	}
}

func requireStatus(t *testing.T, registry *registry.Registry, id string, want model.Status) {
	t.Helper()
	for _, state := range registry.States() {
		if state.Metadata.ID == id {
			if state.Status != want {
				t.Fatalf("status for %s = %s, want %s; last error=%s", id, state.Status, want, state.LastError)
			}
			return
		}
	}
	t.Fatalf("state for %s not found", id)
}
