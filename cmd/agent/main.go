package main

import (
	"log/slog"
	"os"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)

	slog.Info("agent skeleton: optional bootstrap for Alloy/OTel config pull")
}
