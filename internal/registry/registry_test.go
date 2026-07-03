package registry

import (
	"context"
	"testing"
	"time"

	"sentipulse/plugin-execution/internal/model"
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

type closeCountingPlugin struct {
	metadata model.Metadata
	closes   *int
}

func (p *closeCountingPlugin) Descriptor() model.Metadata {
	return p.metadata
}

func (p *closeCountingPlugin) Run(context.Context, map[string]any) (map[string]any, error) {
	return map[string]any{"id": p.metadata.ID}, nil
}

func (p *closeCountingPlugin) Close() error {
	*p.closes++
	return nil
}

func TestRegistryReplaceReusesUnchangedPluginAndClosesChangedPlugin(t *testing.T) {
	registry := NewRegistry()
	manifest := testManifest("demo", "1.0.0")
	manifest.Checksum = "same"

	firstCloses := 0
	first := &closeCountingPlugin{metadata: manifest.Metadata(), closes: &firstCloses}
	registry.Replace([]model.LoadedPlugin{{Manifest: manifest, Plugin: first}})

	unusedReplacementCloses := 0
	unusedReplacement := &closeCountingPlugin{metadata: manifest.Metadata(), closes: &unusedReplacementCloses}
	registry.Replace([]model.LoadedPlugin{{Manifest: manifest, Plugin: unusedReplacement}})

	plugins := registry.EnabledPlugins()
	if len(plugins) != 1 {
		t.Fatalf("enabled plugin count = %d, want 1", len(plugins))
	}
	if plugins[0] != first {
		t.Fatalf("registry did not keep unchanged plugin instance")
	}
	if firstCloses != 0 {
		t.Fatalf("first plugin closes = %d, want 0", firstCloses)
	}

	changed := manifest
	changed.Checksum = "changed"
	secondCloses := 0
	second := &closeCountingPlugin{metadata: changed.Metadata(), closes: &secondCloses}
	registry.Replace([]model.LoadedPlugin{{Manifest: changed, Plugin: second}})

	if firstCloses != 1 {
		t.Fatalf("first plugin closes after changed replace = %d, want 1", firstCloses)
	}
	plugins = registry.EnabledPlugins()
	if len(plugins) != 1 {
		t.Fatalf("enabled plugin count after changed replace = %d, want 1", len(plugins))
	}
	if plugins[0] != second {
		t.Fatalf("registry did not install changed plugin instance")
	}
}

func TestRegistryDependencyValidation(t *testing.T) {
	registry := NewRegistry()
	base := testManifest("base", "1.0.0")
	dependent := testManifest("dependent", "1.0.0")
	dependent.DependsOn = []model.Dependency{{ID: "base", Version: ">=1.0.0"}}

	registry.Replace([]model.LoadedPlugin{
		{Manifest: base, Plugin: stubFor(base)},
		{Manifest: dependent, Plugin: stubFor(dependent)},
	})

	requireStatus(t, registry, "base", model.StatusEnabled)
	requireStatus(t, registry, "dependent", model.StatusEnabled)

	if err := registry.SetEnabled("base", false); err != nil {
		t.Fatalf("disable base: %v", err)
	}

	requireStatus(t, registry, "base", model.StatusDisabled)
	requireStatus(t, registry, "dependent", model.StatusMissingDependency)
}

func TestRegistryPropagatesTransitiveDependencyFailure(t *testing.T) {
	registry := NewRegistry()
	middle := testManifest("middle", "1.0.0")
	middle.DependsOn = []model.Dependency{{ID: "base"}}
	top := testManifest("top", "1.0.0")
	top.DependsOn = []model.Dependency{{ID: "middle"}}

	registry.Replace([]model.LoadedPlugin{
		{Manifest: top, Plugin: stubFor(top)},
		{Manifest: middle, Plugin: stubFor(middle)},
	})

	requireStatus(t, registry, "middle", model.StatusMissingDependency)
	requireStatus(t, registry, "top", model.StatusMissingDependency)
}

func TestRegistryVersionConstraintValidation(t *testing.T) {
	registry := NewRegistry()
	base := testManifest("base", "0.9.0")
	dependent := testManifest("dependent", "1.0.0")
	dependent.DependsOn = []model.Dependency{{ID: "base", Version: ">=1.0.0"}}

	registry.Replace([]model.LoadedPlugin{
		{Manifest: base, Plugin: stubFor(base)},
		{Manifest: dependent, Plugin: stubFor(dependent)},
	})

	requireStatus(t, registry, "base", model.StatusEnabled)
	requireStatus(t, registry, "dependent", model.StatusMissingDependency)
}

func TestRegistryDetectsDependencyCycle(t *testing.T) {
	registry := NewRegistry()
	first := testManifest("first", "1.0.0")
	second := testManifest("second", "1.0.0")
	first.DependsOn = []model.Dependency{{ID: "second"}}
	second.DependsOn = []model.Dependency{{ID: "first"}}

	registry.Replace([]model.LoadedPlugin{
		{Manifest: first, Plugin: stubFor(first)},
		{Manifest: second, Plugin: stubFor(second)},
	})

	requireStatus(t, registry, "first", model.StatusError)
	requireStatus(t, registry, "second", model.StatusError)
}

func TestRegistryDoesNotTreatOptionalBackEdgeAsRequiredCycle(t *testing.T) {
	registry := NewRegistry()
	first := testManifest("first", "1.0.0")
	second := testManifest("second", "1.0.0")
	first.DependsOn = []model.Dependency{{ID: "second", Optional: true}}
	second.DependsOn = []model.Dependency{{ID: "first"}}

	registry.Replace([]model.LoadedPlugin{
		{Manifest: first, Plugin: stubFor(first)},
		{Manifest: second, Plugin: stubFor(second)},
	})

	requireStatus(t, registry, "first", model.StatusEnabled)
	requireStatus(t, registry, "second", model.StatusEnabled)
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

func stubFor(manifest model.Manifest) stubPlugin {
	return stubPlugin{
		metadata: manifest.Metadata(),
		result:   map[string]any{"id": manifest.ID},
	}
}

func requireStatus(t *testing.T, registry *Registry, id string, want model.Status) {
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
