package main

import (
	"log/slog"
	"os"

	"github.com/cloudwego/eino/schema"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)

	// Eino dependency anchor; replace with compose/flow graphs in internal/advisor.
	slog.Info("advisor-worker skeleton", "eino_schema_role", schema.Assistant)
}
