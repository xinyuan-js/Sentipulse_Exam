package main

import (
	"time"

	"sentipulse/plugin-execution/pkg/sdk"
)

func main() {
	sdk.Serve(func(data map[string]any) (map[string]any, error) {
		return map[string]any{
			"received":    data,
			"plugin":      "go.echo",
			"processedAt": time.Now().UTC().Format(time.RFC3339),
		}, nil
	})
}
