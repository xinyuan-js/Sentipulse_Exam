package main

import (
	"strings"

	"sentipulse/plugin-execution/pkg/sdk"
)

func main() {
	sdk.Serve(func(data map[string]any) (map[string]any, error) {
		text, _ := data["text"].(string)
		words := strings.Fields(text)
		return map[string]any{
			"chars": len([]rune(text)),
			"words": len(words),
			"empty": strings.TrimSpace(text) == "",
		}, nil
	})
}
