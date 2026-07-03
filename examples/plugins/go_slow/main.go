package main

import (
	"time"

	"sentipulse/plugin-execution/pkg/sdk"
)

func main() {
	sdk.Serve(func(data map[string]any) (map[string]any, error) {
		time.Sleep(3 * time.Second)
		return map[string]any{"done": true}, nil
	})
}
