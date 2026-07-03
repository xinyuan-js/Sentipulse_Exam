package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"sentipulse/plugin-execution/internal/model"
	"sentipulse/plugin-execution/pkg/protocol"
)

var (
	ErrPluginTimeout  = errors.New("plugin execution timeout")
	ErrPluginProtocol = errors.New("plugin protocol error")
	ErrPluginExited   = errors.New("plugin process exited with error")
)

type ProcessPlugin struct {
	manifest model.Manifest
}

func NewProcessPlugin(manifest model.Manifest) *ProcessPlugin {
	return &ProcessPlugin{manifest: manifest}
}

func (p *ProcessPlugin) Descriptor() model.Metadata {
	return p.manifest.Metadata()
}

func (p *ProcessPlugin) Run(ctx context.Context, data map[string]any) (map[string]any, error) {
	runCtx, cancel := context.WithTimeout(ctx, p.manifest.Timeout)
	defer cancel()

	payload, err := json.Marshal(protocol.PluginRequest{Data: data})
	if err != nil {
		return nil, fmt.Errorf("%w: encode request: %w", ErrPluginProtocol, err)
	}

	cmd := exec.CommandContext(runCtx, p.manifest.Command, p.manifest.Args...)
	cmd.Dir = p.manifest.WorkingDir
	cmd.Env = mergeEnv(os.Environ(), p.manifest.Env)
	cmd.Stdin = bytes.NewReader(payload)
	cmd.WaitDelay = 500 * time.Millisecond
	configureCommand(cmd)
	cmd.Cancel = func() error {
		return terminateCommand(cmd)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if runCtx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("%w after %s", ErrPluginTimeout, p.manifest.Timeout)
	}
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("%w: %s", ErrPluginExited, trimForError(message))
	}

	output := bytes.TrimSpace(stdout.Bytes())
	if len(output) == 0 {
		return map[string]any{}, nil
	}

	var response protocol.PluginResponse
	decoder := json.NewDecoder(bytes.NewReader(output))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&response); err != nil {
		return nil, fmt.Errorf("%w: decode response: %w; output=%q", ErrPluginProtocol, err, trimForError(string(output)))
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("%w: response contains multiple JSON values; output=%q", ErrPluginProtocol, trimForError(string(output)))
		}
		return nil, fmt.Errorf("%w: response has trailing data: %w; output=%q", ErrPluginProtocol, err, trimForError(string(output)))
	}
	if strings.TrimSpace(response.Error) != "" {
		return response.Result, fmt.Errorf("%w: %s", ErrPluginExited, response.Error)
	}
	if response.Result == nil {
		response.Result = map[string]any{}
	}
	return response.Result, nil
}

func (p *ProcessPlugin) Close() error {
	return nil
}

func mergeEnv(base []string, extra map[string]string) []string {
	if len(extra) == 0 {
		return base
	}
	out := append([]string(nil), base...)
	keys := make([]string, 0, len(extra))
	for key := range extra {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		out = append(out, key+"="+extra[key])
	}
	return out
}

func trimForError(value string) string {
	const limit = 2048
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "...(truncated)"
}
