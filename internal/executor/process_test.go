package executor

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"sentipulse/plugin-execution/internal/model"
	"sentipulse/plugin-execution/pkg/protocol"
)

func TestProcessPluginExecutesProtocol(t *testing.T) {
	manifest := helperManifest("echo", 2*time.Second)
	plugin := NewProcessPlugin(manifest)

	result, err := plugin.Run(context.Background(), map[string]any{"text": "hello"})
	if err != nil {
		t.Fatalf("run helper plugin: %v", err)
	}
	if result["text"] != "hello" {
		t.Fatalf("result text = %v, want hello", result["text"])
	}
}

func TestProcessPluginTimeout(t *testing.T) {
	manifest := helperManifest("sleep", 50*time.Millisecond)
	plugin := NewProcessPlugin(manifest)

	_, err := plugin.Run(context.Background(), map[string]any{"text": "hello"})
	if !errors.Is(err, ErrPluginTimeout) {
		t.Fatalf("error = %v, want ErrPluginTimeout", err)
	}
}

func TestProcessPluginRejectsTrailingOutput(t *testing.T) {
	manifest := helperManifest("extra", 2*time.Second)
	plugin := NewProcessPlugin(manifest)

	_, err := plugin.Run(context.Background(), map[string]any{"text": "hello"})
	if !errors.Is(err, ErrPluginProtocol) {
		t.Fatalf("error = %v, want ErrPluginProtocol", err)
	}
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("PLUGIN_TEST_HELPER") != "1" {
		return
	}

	mode := ""
	for i, arg := range os.Args {
		if arg == "--" && i+1 < len(os.Args) {
			mode = os.Args[i+1]
			break
		}
	}

	switch mode {
	case "echo":
		var req protocol.PluginRequest
		if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
			os.Exit(2)
		}
		text, _ := req.Data["text"].(string)
		_ = json.NewEncoder(os.Stdout).Encode(protocol.PluginResponse{Result: map[string]any{"text": text}})
		os.Exit(0)
	case "sleep":
		time.Sleep(2 * time.Second)
		os.Exit(0)
	case "extra":
		_, _ = os.Stdout.WriteString(`{"result":{"ok":true}}` + "\nplugin log on stdout")
		os.Exit(0)
	default:
		os.Exit(3)
	}
}

func helperManifest(mode string, timeout time.Duration) model.Manifest {
	return model.Manifest{
		ID:         "helper." + mode,
		Name:       "Helper " + mode,
		Version:    "1.0.0",
		Type:       "test-process",
		Enabled:    true,
		Command:    os.Args[0],
		Args:       []string{"-test.run=TestHelperProcess", "--", mode},
		WorkingDir: ".",
		Timeout:    timeout,
		Env: map[string]string{
			"PLUGIN_TEST_HELPER": "1",
		},
	}
}
