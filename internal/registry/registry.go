package registry

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"sentipulse/plugin-execution/internal/model"
	"sentipulse/plugin-execution/internal/version"
)

type Registry struct {
	mu      sync.RWMutex
	entries map[string]*registryEntry
}

type registryEntry struct {
	manifest model.Manifest
	plugin   model.Plugin
	state    model.PluginState
}

func NewRegistry() *Registry {
	return &Registry{entries: make(map[string]*registryEntry)}
}

func (r *Registry) Replace(loaded []model.LoadedPlugin) {
	now := time.Now().UTC()

	r.mu.Lock()
	defer r.mu.Unlock()

	next := make(map[string]*registryEntry, len(loaded))
	for _, candidate := range loaded {
		existing, exists := r.entries[candidate.Manifest.ID]
		plugin := candidate.Plugin
		if exists && existing.manifest.Checksum == candidate.Manifest.Checksum {
			plugin = existing.plugin
		}
		if exists && existing.plugin != nil && existing.manifest.Checksum != candidate.Manifest.Checksum {
			_ = existing.plugin.Close()
		}

		loadedAt := now
		if exists && existing.manifest.Checksum == candidate.Manifest.Checksum {
			loadedAt = existing.state.LoadedAt
		}

		state := model.PluginState{
			Metadata:     candidate.Manifest.Metadata(),
			Enabled:      candidate.Manifest.Enabled,
			Status:       model.StatusEnabled,
			Dependencies: append([]model.Dependency(nil), candidate.Manifest.DependsOn...),
			LoadedAt:     loadedAt,
			UpdatedAt:    now,
		}
		if !candidate.Manifest.Enabled {
			state.Status = model.StatusDisabled
		}
		if candidate.LoadError != nil {
			state.Status = model.StatusError
			state.Enabled = candidate.Manifest.Enabled
			state.LastError = candidate.LoadError.Error()
		}

		next[candidate.Manifest.ID] = &registryEntry{
			manifest: candidate.Manifest,
			plugin:   plugin,
			state:    state,
		}
	}

	for id, existing := range r.entries {
		if _, stillPresent := next[id]; !stillPresent && existing.plugin != nil {
			_ = existing.plugin.Close()
		}
	}

	r.entries = next
	r.validateLocked()
}

func (r *Registry) States() []model.PluginState {
	r.mu.RLock()
	defer r.mu.RUnlock()

	states := make([]model.PluginState, 0, len(r.entries))
	for _, entry := range r.entries {
		states = append(states, entry.state)
	}
	sort.Slice(states, func(i, j int) bool {
		return states[i].Metadata.ID < states[j].Metadata.ID
	})
	return states
}

func (r *Registry) EnabledPlugins() []model.Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugins := make([]model.Plugin, 0, len(r.entries))
	for _, entry := range r.entries {
		if entry.plugin != nil && entry.state.Status == model.StatusEnabled && entry.state.Enabled {
			plugins = append(plugins, entry.plugin)
		}
	}
	sort.Slice(plugins, func(i, j int) bool {
		return plugins[i].Descriptor().ID < plugins[j].Descriptor().ID
	})
	return plugins
}

func (r *Registry) SetEnabled(id string, enabled bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.entries[id]
	if !ok {
		return fmt.Errorf("plugin %q not found", id)
	}

	entry.manifest.Enabled = enabled
	entry.state.Enabled = enabled
	entry.state.UpdatedAt = time.Now().UTC()
	entry.state.LastError = ""
	if enabled {
		entry.state.Status = model.StatusEnabled
	} else {
		entry.state.Status = model.StatusDisabled
	}
	r.validateLocked()
	return nil
}

func (r *Registry) Fallback(id string) map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, ok := r.entries[id]
	if !ok {
		return nil
	}
	return model.CloneAnyMap(entry.manifest.Fallback)
}

func (r *Registry) MarkExecution(id string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.entries[id]
	if !ok {
		return
	}
	entry.state.UpdatedAt = time.Now().UTC()
	if err != nil {
		entry.state.Status = model.StatusError
		entry.state.LastError = err.Error()
		return
	}
	if entry.manifest.Enabled {
		entry.state.Status = model.StatusEnabled
		entry.state.LastError = ""
	}
}

func (r *Registry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var errs []string
	for _, entry := range r.entries {
		if entry.plugin == nil {
			continue
		}
		if err := entry.plugin.Close(); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("close plugins: %s", strings.Join(errs, "; "))
	}
	return nil
}

func (r *Registry) validateLocked() {
	now := time.Now().UTC()
	for _, entry := range r.entries {
		if entry.state.Status == model.StatusError && entry.plugin == nil {
			continue
		}
		entry.state.Enabled = entry.manifest.Enabled
		entry.state.UpdatedAt = now
		if !entry.manifest.Enabled {
			entry.state.Status = model.StatusDisabled
			entry.state.LastError = ""
			continue
		}
		if entry.plugin == nil {
			entry.state.Status = model.StatusError
			if entry.state.LastError == "" {
				entry.state.LastError = "plugin implementation is not available"
			}
			continue
		}
		entry.state.Status = model.StatusEnabled
		entry.state.LastError = ""
	}

	cycled := r.detectCyclesLocked()
	for id := range cycled {
		if entry, ok := r.entries[id]; ok && entry.state.Status == model.StatusEnabled {
			entry.state.Status = model.StatusError
			entry.state.LastError = "dependency cycle detected"
		}
	}

	r.validateDependenciesLocked()
}

func (r *Registry) validateDependenciesLocked() {
	visiting := make(map[string]bool, len(r.entries))
	visited := make(map[string]bool, len(r.entries))

	var validate func(id string) model.Status
	validate = func(id string) model.Status {
		entry, ok := r.entries[id]
		if !ok {
			return model.StatusMissingDependency
		}
		if entry.state.Status != model.StatusEnabled {
			return entry.state.Status
		}
		if visiting[id] {
			return entry.state.Status
		}
		if visited[id] {
			return entry.state.Status
		}

		visiting[id] = true
		defer func() {
			visiting[id] = false
			visited[id] = true
		}()

		for _, dep := range entry.manifest.DependsOn {
			depEntry, ok := r.entries[dep.ID]
			if !ok {
				if dep.Optional {
					continue
				}
				entry.state.Status = model.StatusMissingDependency
				entry.state.LastError = fmt.Sprintf("required dependency %q is not loaded", dep.ID)
				return entry.state.Status
			}

			depStatus := validate(dep.ID)
			if depStatus != model.StatusEnabled {
				if dep.Optional {
					continue
				}
				entry.state.Status = model.StatusMissingDependency
				entry.state.LastError = fmt.Sprintf("required dependency %q is %s", dep.ID, depStatus)
				return entry.state.Status
			}

			ok, err := version.Satisfies(depEntry.manifest.Version, dep.Version)
			if err != nil {
				entry.state.Status = model.StatusError
				entry.state.LastError = fmt.Sprintf("invalid dependency constraint for %q: %v", dep.ID, err)
				return entry.state.Status
			}
			if !ok {
				entry.state.Status = model.StatusMissingDependency
				entry.state.LastError = fmt.Sprintf("dependency %q version %s does not satisfy %q", dep.ID, depEntry.manifest.Version, dep.Version)
				return entry.state.Status
			}
		}

		return entry.state.Status
	}

	for id := range r.entries {
		validate(id)
	}
}

func (r *Registry) detectCyclesLocked() map[string]struct{} {
	visiting := make(map[string]bool, len(r.entries))
	visited := make(map[string]bool, len(r.entries))
	cycled := make(map[string]struct{})

	var visit func(id string, path []string)
	visit = func(id string, path []string) {
		if visiting[id] {
			inCycle := false
			for _, pathID := range path {
				if pathID == id {
					inCycle = true
				}
				if inCycle {
					cycled[pathID] = struct{}{}
				}
			}
			cycled[id] = struct{}{}
			return
		}
		if visited[id] {
			return
		}
		entry, ok := r.entries[id]
		if !ok || entry.state.Status != model.StatusEnabled {
			return
		}

		visiting[id] = true
		for _, dep := range entry.manifest.DependsOn {
			if dep.Optional {
				continue
			}
			visit(dep.ID, append(path, id))
		}
		visiting[id] = false
		visited[id] = true
	}

	for id := range r.entries {
		visit(id, nil)
	}
	return cycled
}
