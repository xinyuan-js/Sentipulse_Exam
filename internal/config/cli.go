package config

import (
	"flag"
	"fmt"
	"io"
	"strings"
	"time"
)

const DefaultInput = `{"message":"hello plugin system","text":"hello plugin system","value":42}`

type CLI struct {
	PluginsDir       string
	InputRaw         string
	InputFile        string
	List             bool
	Watch            bool
	Interval         time.Duration
	ExecutionTimeout time.Duration
	EnableIDs        []string
	DisableIDs       []string
	Parallelism      int
}

func ParseCLI(args []string) (CLI, error) {
	var cfg CLI
	var enable string
	var disable string

	flags := flag.NewFlagSet("plugin-executor", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&cfg.PluginsDir, "plugins", "plugins", "directory containing plugin manifests")
	flags.StringVar(&cfg.InputRaw, "input", DefaultInput, "JSON input passed to enabled plugins")
	flags.StringVar(&cfg.InputFile, "input-file", "", "path to a JSON input file")
	flags.BoolVar(&cfg.List, "list", false, "list plugin states and exit")
	flags.BoolVar(&cfg.Watch, "watch", false, "watch the plugin directory and print state changes")
	flags.DurationVar(&cfg.Interval, "interval", 2*time.Second, "plugin watch scan interval")
	flags.DurationVar(&cfg.ExecutionTimeout, "execution-timeout", 30*time.Second, "overall execution timeout")
	flags.StringVar(&enable, "enable", "", "comma-separated plugin ids to enable for this run")
	flags.StringVar(&disable, "disable", "", "comma-separated plugin ids to disable for this run")
	flags.IntVar(&cfg.Parallelism, "parallelism", 8, "maximum number of plugins to run concurrently")

	if err := flags.Parse(args); err != nil {
		return CLI{}, err
	}
	if cfg.Interval <= 0 {
		return CLI{}, fmt.Errorf("interval must be positive")
	}
	if cfg.ExecutionTimeout <= 0 {
		return CLI{}, fmt.Errorf("execution-timeout must be positive")
	}
	cfg.EnableIDs = splitIDs(enable)
	cfg.DisableIDs = splitIDs(disable)
	return cfg, nil
}

func splitIDs(raw string) []string {
	var ids []string
	for _, id := range strings.Split(raw, ",") {
		id = strings.TrimSpace(id)
		if id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}
