package main

import (
	"os"

	"sentipulse/plugin-execution/internal/app"
)

func main() {
	os.Exit(app.Run(os.Args[1:], os.Stdout, os.Stderr))
}
