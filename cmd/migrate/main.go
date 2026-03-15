package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
)

func main() {
	direction := flag.String("direction", "up", "migration direction: up or down")
	flag.Parse()

	slog.Info("migrate tool", "direction", *direction)

	// Phase 0: placeholder — actual migration logic added in Phase 1
	fmt.Fprintf(os.Stderr, "migrate %s: no migrations configured yet\n", *direction)
}
