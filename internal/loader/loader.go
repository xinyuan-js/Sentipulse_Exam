package loader

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"sentipulse/plugin-execution/internal/executor"
	"sentipulse/plugin-execution/internal/model"
)

type Loader struct {
	root string
}

func NewLoader(root string) *Loader {
	return &Loader{root: root}
}

func (l *Loader) Load(ctx context.Context) ([]model.LoadedPlugin, error) {
	manifestPaths, err := l.findManifests()
	if err != nil {
		return nil, err
	}

	loaded := make([]model.LoadedPlugin, 0, len(manifestPaths))
	seen := make(map[string]string, len(manifestPaths))
	for _, path := range manifestPaths {
		if err := ctx.Err(); err != nil {
			return loaded, err
		}

		manifest, err := LoadManifest(path)
		if err != nil {
			loaded = append(loaded, model.LoadedPlugin{
				Manifest:  manifest,
				LoadError: err,
			})
			continue
		}

		if previous, exists := seen[manifest.ID]; exists {
			loaded = append(loaded, model.LoadedPlugin{
				Manifest:  manifest,
				LoadError: fmt.Errorf("duplicate plugin id %q already declared in %s", manifest.ID, previous),
			})
			continue
		}
		seen[manifest.ID] = path

		loaded = append(loaded, model.LoadedPlugin{
			Manifest: manifest,
			Plugin:   executor.NewProcessPlugin(manifest),
		})
	}
	return loaded, nil
}

func (l *Loader) findManifests() ([]string, error) {
	if _, err := os.Stat(l.root); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var paths []string
	err := filepath.WalkDir(l.root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			name := entry.Name()
			if name == ".git" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Name() == "plugin.json" {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}
