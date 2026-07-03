package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sentipulse/plugin-execution/internal/config"
	"sentipulse/plugin-execution/internal/core"
	"sentipulse/plugin-execution/internal/executor"
	"sentipulse/plugin-execution/internal/registry"
)

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	cfg, err := config.ParseCLI(args)
	if err != nil {
		fmt.Fprintf(stderr, "parse flags: %v\n", err)
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	manager := core.NewManager(cfg.PluginsDir, core.ManagerOptions{ScanInterval: cfg.Interval})
	defer func() {
		if err := manager.Close(); err != nil {
			fmt.Fprintf(stderr, "close manager: %v\n", err)
		}
	}()

	if err := manager.Reload(ctx); err != nil {
		fmt.Fprintf(stderr, "load plugins: %v\n", err)
		return 1
	}

	applyRuntimeToggles(stderr, manager.Registry(), cfg.EnableIDs, true)
	applyRuntimeToggles(stderr, manager.Registry(), cfg.DisableIDs, false)

	if cfg.List {
		printJSON(stderr, stdout, manager.Registry().States())
		return 0
	}

	if cfg.Watch {
		if err := watchStates(ctx, stderr, stdout, manager, cfg.Interval, cfg.EnableIDs, cfg.DisableIDs); err != nil && ctx.Err() == nil {
			fmt.Fprintf(stderr, "watch plugins: %v\n", err)
			return 1
		}
		return 0
	}

	input, err := readInput(cfg.InputRaw, cfg.InputFile)
	if err != nil {
		fmt.Fprintf(stderr, "read input: %v\n", err)
		return 1
	}

	runCtx, cancel := context.WithTimeout(ctx, cfg.ExecutionTimeout)
	defer cancel()

	runner := executor.NewExecutor(manager.Registry(), executor.Options{Parallelism: cfg.Parallelism})
	summary, err := runner.Execute(runCtx, input)
	if err != nil {
		fmt.Fprintf(stderr, "execute plugins: %v\n", err)
		return 1
	}
	printJSON(stderr, stdout, summary)
	return 0
}

func applyRuntimeToggles(stderr io.Writer, registry *registry.Registry, ids []string, enabled bool) {
	for _, id := range ids {
		if err := registry.SetEnabled(id, enabled); err != nil {
			fmt.Fprintf(stderr, "set plugin %s enabled=%v: %v\n", id, enabled, err)
		}
	}
}

func readInput(raw string, file string) (map[string]any, error) {
	if file != "" {
		content, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		raw = string(content)
	}

	var input map[string]any
	if err := json.Unmarshal([]byte(raw), &input); err != nil {
		return nil, err
	}
	if input == nil {
		input = map[string]any{}
	}
	return input, nil
}

func watchStates(ctx context.Context, stderr io.Writer, stdout io.Writer, manager *core.Manager, interval time.Duration, enableIDs []string, disableIDs []string) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		if err := manager.Reload(ctx); err != nil {
			return err
		}
		applyRuntimeToggles(stderr, manager.Registry(), enableIDs, true)
		applyRuntimeToggles(stderr, manager.Registry(), disableIDs, false)
		printJSON(stderr, stdout, manager.Registry().States())

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func printJSON(stderr io.Writer, stdout io.Writer, value any) {
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		fmt.Fprintf(stderr, "encode output: %v\n", err)
	}
}
