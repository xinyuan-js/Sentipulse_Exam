package core

import (
	"context"
	"time"

	"sentipulse/plugin-execution/internal/loader"
	"sentipulse/plugin-execution/internal/registry"
)

type ManagerOptions struct {
	ScanInterval time.Duration
}

type Manager struct {
	loader   *loader.Loader
	registry *registry.Registry
	options  ManagerOptions
}

func NewManager(pluginDir string, options ManagerOptions) *Manager {
	if options.ScanInterval <= 0 {
		options.ScanInterval = 2 * time.Second
	}
	return &Manager{
		loader:   loader.NewLoader(pluginDir),
		registry: registry.NewRegistry(),
		options:  options,
	}
}

func (m *Manager) Reload(ctx context.Context) error {
	loaded, err := m.loader.Load(ctx)
	if err != nil {
		return err
	}
	m.registry.Replace(loaded)
	return nil
}

func (m *Manager) Watch(ctx context.Context) error {
	ticker := time.NewTicker(m.options.ScanInterval)
	defer ticker.Stop()

	if err := m.Reload(ctx); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := m.Reload(ctx); err != nil {
				return err
			}
		}
	}
}

func (m *Manager) Registry() *registry.Registry {
	return m.registry
}

func (m *Manager) Close() error {
	return m.registry.Close()
}
