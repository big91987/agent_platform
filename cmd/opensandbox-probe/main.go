package main

import (
	"agentplatform/internal/platform"
	"context"
	"flag"
	"fmt"
	"os"
	"time"
)

func main() {
	config := flag.String("service-config", "", "private OpenSandbox sandbox.toml")
	report := flag.String("report", "", "new private JSONL report path")
	image := flag.String("image", "python:3.12-slim", "probe container image")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	if err := platform.ProbeOpenSandbox(ctx, *config, *image, *report); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("OPENSANDBOX_PROBE_PASS: lifecycle, Python, file round-trip and remote cancellation; not product acceptance")
}
