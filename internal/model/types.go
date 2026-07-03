package model

import (
	"context"
	"time"
)

type Status string

const (
	StatusEnabled           Status = "enabled"
	StatusDisabled          Status = "disabled"
	StatusError             Status = "error"
	StatusMissingDependency Status = "missing_dependency"
)

type Metadata struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Type    string `json:"type"`
}

type Plugin interface {
	Descriptor() Metadata
	Run(ctx context.Context, data map[string]any) (map[string]any, error)
	Close() error
}

type PluginState struct {
	Metadata     Metadata     `json:"metadata"`
	Enabled      bool         `json:"enabled"`
	Status       Status       `json:"status"`
	LastError    string       `json:"last_error,omitempty"`
	Dependencies []Dependency `json:"dependencies,omitempty"`
	LoadedAt     time.Time    `json:"loaded_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

type Dependency struct {
	ID       string `json:"id"`
	Version  string `json:"version,omitempty"`
	Optional bool   `json:"optional,omitempty"`
}

type Manifest struct {
	ID         string
	Name       string
	Version    string
	Type       string
	Enabled    bool
	Command    string
	Args       []string
	WorkingDir string
	Timeout    time.Duration
	DependsOn  []Dependency
	Env        map[string]string
	Fallback   map[string]any
	SourcePath string
	SourceDir  string
	Checksum   string
}

func (m Manifest) Metadata() Metadata {
	return Metadata{
		ID:      m.ID,
		Name:    m.Name,
		Version: m.Version,
		Type:    m.Type,
	}
}

type LoadedPlugin struct {
	Manifest  Manifest
	Plugin    Plugin
	LoadError error
}

func CloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func CloneAnyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
