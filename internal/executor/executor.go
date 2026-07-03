package executor

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"sync"
	"time"

	"sentipulse/plugin-execution/internal/model"
	"sentipulse/plugin-execution/internal/registry"
)

type Options struct {
	Parallelism int
}

type Executor struct {
	registry *registry.Registry
	options  Options
}

type ExecutionSummary struct {
	StartedAt   time.Time         `json:"started_at"`
	FinishedAt  time.Time         `json:"finished_at"`
	PluginCount int               `json:"plugin_count"`
	Successful  int               `json:"successful"`
	Failed      int               `json:"failed"`
	Degraded    int               `json:"degraded"`
	Results     []ExecutionResult `json:"results"`
}

type ExecutionResult struct {
	Plugin   model.Metadata `json:"plugin"`
	Duration string         `json:"duration"`
	Result   map[string]any `json:"result,omitempty"`
	Error    string         `json:"error,omitempty"`
	Degraded bool           `json:"degraded"`
}

func NewExecutor(registry *registry.Registry, options Options) *Executor {
	if options.Parallelism <= 0 {
		options.Parallelism = runtime.NumCPU()
	}
	return &Executor{registry: registry, options: options}
}

func (e *Executor) Execute(ctx context.Context, input map[string]any) (ExecutionSummary, error) {
	startedAt := time.Now().UTC()
	plugins := e.registry.EnabledPlugins()
	summary := ExecutionSummary{
		StartedAt:   startedAt,
		PluginCount: len(plugins),
		Results:     make([]ExecutionResult, 0, len(plugins)),
	}
	if len(plugins) == 0 {
		summary.FinishedAt = time.Now().UTC()
		return summary, nil
	}

	parallelism := e.options.Parallelism
	if parallelism > len(plugins) {
		parallelism = len(plugins)
	}

	sem := make(chan struct{}, parallelism)
	results := make(chan ExecutionResult, len(plugins))
	var wg sync.WaitGroup

	for _, candidate := range plugins {
		plugin := candidate
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results <- e.executeOne(ctx, plugin, input)
		}()
	}

	wg.Wait()
	close(results)

	for result := range results {
		if result.Error == "" {
			summary.Successful++
		} else {
			summary.Failed++
		}
		if result.Degraded {
			summary.Degraded++
		}
		summary.Results = append(summary.Results, result)
	}
	sort.Slice(summary.Results, func(i, j int) bool {
		return summary.Results[i].Plugin.ID < summary.Results[j].Plugin.ID
	})
	summary.FinishedAt = time.Now().UTC()
	return summary, nil
}

func (e *Executor) executeOne(ctx context.Context, candidate model.Plugin, input map[string]any) (result ExecutionResult) {
	metadata := candidate.Descriptor()
	startedAt := time.Now()
	result = ExecutionResult{
		Plugin: metadata,
	}

	defer func() {
		result.Duration = time.Since(startedAt).String()
	}()

	output, err := runSafely(ctx, candidate, model.CloneAnyMap(input))
	if err != nil {
		e.registry.MarkExecution(metadata.ID, err)
		result.Error = err.Error()
		if fallback := e.registry.Fallback(metadata.ID); len(fallback) > 0 {
			result.Result = fallback
			result.Degraded = true
		}
		return result
	}

	e.registry.MarkExecution(metadata.ID, nil)
	result.Result = output
	return result
}

func runSafely(ctx context.Context, candidate model.Plugin, input map[string]any) (output map[string]any, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("plugin panic recovered: %v", recovered)
		}
	}()
	return candidate.Run(ctx, input)
}
