package main

import (
	"log/slog"
	"os"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)

	slog.Info("obs-mcp skeleton: implement MCP server (stdio or HTTP transport)")
}
