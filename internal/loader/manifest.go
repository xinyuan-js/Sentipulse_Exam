package loader

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"sentipulse/plugin-execution/internal/model"
	"sentipulse/plugin-execution/internal/version"
)

const defaultPluginTimeout = 3 * time.Second

type rawManifest struct {
	ID         string             `json:"id"`
	Name       string             `json:"name"`
	Version    string             `json:"version"`
	Type       string             `json:"type"`
	Enabled    *bool              `json:"enabled,omitempty"`
	Command    string             `json:"command"`
	Args       []string           `json:"args,omitempty"`
	WorkingDir string             `json:"working_dir,omitempty"`
	Timeout    string             `json:"timeout,omitempty"`
	DependsOn  []model.Dependency `json:"depends_on,omitempty"`
	Env        map[string]string  `json:"env,omitempty"`
	Fallback   map[string]any     `json:"fallback,omitempty"`
}

func LoadManifest(path string) (model.Manifest, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return model.Manifest{}, err
	}
	return parseManifest(path, content)
}

func parseManifest(path string, content []byte) (model.Manifest, error) {
	var raw rawManifest
	decoder := json.NewDecoder(strings.NewReader(string(content)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		return invalidManifest(path), fmt.Errorf("decode manifest: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return invalidManifest(path), fmt.Errorf("decode manifest: multiple JSON values")
	} else if err != io.EOF {
		return invalidManifest(path), fmt.Errorf("decode manifest: trailing data: %w", err)
	}

	enabled := true
	if raw.Enabled != nil {
		enabled = *raw.Enabled
	}

	timeout := defaultPluginTimeout
	if raw.Timeout != "" {
		parsed, err := time.ParseDuration(raw.Timeout)
		if err != nil {
			return invalidManifest(path), fmt.Errorf("invalid timeout %q: %w", raw.Timeout, err)
		}
		timeout = parsed
	}

	sourceDir := filepath.Dir(path)
	workingDir := raw.WorkingDir
	if workingDir == "" {
		workingDir = "."
	}
	if !filepath.IsAbs(workingDir) {
		workingDir = filepath.Clean(filepath.Join(sourceDir, workingDir))
	}

	manifest := model.Manifest{
		ID:         strings.TrimSpace(raw.ID),
		Name:       strings.TrimSpace(raw.Name),
		Version:    strings.TrimSpace(raw.Version),
		Type:       strings.TrimSpace(raw.Type),
		Enabled:    enabled,
		Command:    strings.TrimSpace(raw.Command),
		Args:       append([]string(nil), raw.Args...),
		WorkingDir: workingDir,
		Timeout:    timeout,
		DependsOn:  append([]model.Dependency(nil), raw.DependsOn...),
		Env:        model.CloneStringMap(raw.Env),
		Fallback:   model.CloneAnyMap(raw.Fallback),
		SourcePath: path,
		SourceDir:  sourceDir,
		Checksum:   checksum(content),
	}

	if err := validateManifest(manifest); err != nil {
		return manifest, err
	}
	return manifest, nil
}

func validateManifest(m model.Manifest) error {
	var missing []string
	if m.ID == "" {
		missing = append(missing, "id")
	}
	if m.Name == "" {
		missing = append(missing, "name")
	}
	if m.Version == "" {
		missing = append(missing, "version")
	}
	if m.Type == "" {
		missing = append(missing, "type")
	}
	if m.Command == "" {
		missing = append(missing, "command")
	}
	if len(missing) > 0 {
		return fmt.Errorf("manifest missing required fields: %s", strings.Join(missing, ", "))
	}
	if m.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	for _, dep := range m.DependsOn {
		if strings.TrimSpace(dep.ID) == "" {
			return fmt.Errorf("dependency id must not be empty")
		}
		if dep.Version != "" {
			if err := version.ValidateConstraint(dep.Version); err != nil {
				return fmt.Errorf("dependency %s has invalid version constraint %q: %w", dep.ID, dep.Version, err)
			}
		}
	}
	return nil
}

func invalidManifest(path string) model.Manifest {
	id := filepath.Base(filepath.Dir(path))
	if id == "." || id == string(filepath.Separator) {
		id = "invalid"
	}
	return model.Manifest{
		ID:         id,
		Name:       id,
		Version:    "0.0.0",
		Type:       "invalid",
		Enabled:    false,
		Timeout:    defaultPluginTimeout,
		SourcePath: path,
		SourceDir:  filepath.Dir(path),
	}
}

func checksum(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}
