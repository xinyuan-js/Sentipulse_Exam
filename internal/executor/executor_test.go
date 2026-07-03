package executor

import (
	"context"
	"errors"
	"testing"
	"time"

	"sentipulse/plugin-execution/internal/model"
	"sentipulse/plugin-execution/internal/registry"
)

type stubPlugin struct {
	metadata model.Metadata
	result   map[string]any
	err      error
}

func (s stubPlugin) Descriptor() model.Metadata {
	return s.metadata
}

func (s stubPlugin) Run(context.Context, map[string]any) (map[string]any, error) {
	if s.err != nil {
		return nil, s.err
	}
	return model.CloneAnyMap(s.result), nil
}

func (s stubPlugin) Close() error {
	return nil
}

func TestExecutorUsesFallbackWithoutStoppingOtherPlugins(t *testing.T) {
	reg := registry.NewRegistry()
	okManifest := testManifest("ok", "1.0.0")
	failingManifest := testManifest("failing", "1.0.0")
	failingManifest.Fallback = map[string]any{"mode": "fallback"}

	reg.Replace([]model.LoadedPlugin{
		{Manifest: okManifest, Plugin: stubPlugin{metadata: okManifest.Metadata(), result: map[string]any{"ok": true}}},
		{Manifest: failingManifest, Plugin: stubPlugin{metadata: failingManifest.Metadata(), err: errors.New("boom")}},
	})

	executor := NewExecutor(reg, Options{Parallelism: 2})
	summary, err := executor.Execute(context.Background(), map[string]any{"input": true})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if summary.Successful != 1 || summary.Failed != 1 || summary.Degraded != 1 {
		t.Fatalf("unexpected summary: successful=%d failed=%d degraded=%d", summary.Successful, summary.Failed, summary.Degraded)
	}
	if len(summary.Results) != 2 {
		t.Fatalf("got %d results, want 2", len(summary.Results))
	}
	requireStatus(t, reg, "failing", model.StatusError)
}

func testManifest(id string, version string) model.Manifest {
	return model.Manifest{
		ID:         id,
		Name:       id,
		Version:    version,
		Type:       "test",
		Enabled:    true,
		Command:    "test",
		WorkingDir: ".",
		Timeout:    time.Second,
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
