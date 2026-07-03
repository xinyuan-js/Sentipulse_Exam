package sdk

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"sentipulse/plugin-execution/pkg/protocol"
)

type Handler func(data map[string]any) (map[string]any, error)

func Serve(handler Handler) {
	if err := Run(os.Stdin, os.Stdout, handler); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func Run(in io.Reader, out io.Writer, handler Handler) error {
	var req protocol.PluginRequest
	if err := json.NewDecoder(in).Decode(&req); err != nil {
		return fmt.Errorf("decode request: %w", err)
	}
	if req.Data == nil {
		req.Data = map[string]any{}
	}

	result, err := handler(req.Data)
	resp := protocol.PluginResponse{Result: result}
	if err != nil {
		resp.Error = err.Error()
	}
	if resp.Result == nil {
		resp.Result = map[string]any{}
	}

	if err := json.NewEncoder(out).Encode(resp); err != nil {
		return fmt.Errorf("encode response: %w", err)
	}
	return nil
}
