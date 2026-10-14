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
	runtime := flag.Bool("runtime", false, "also verify TypeScript and credential-free native service transport")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	probe := platform.ProbeOpenSandbox
	if *runtime {
		probe = platform.ProbeOpenSandboxRuntime
	}
	if err := probe(ctx, *config, *image, *report); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("OPENSANDBOX_PROBE_PASS: lifecycle, Python, file round-trip and remote cancellation; not product acceptance")
	if *runtime {
		fmt.Println("OPENSANDBOX_RUNTIME_PASS: TypeScript, authenticated native initialization, Skill scope, external-sandbox command and owned process stop; native login/model turn Not Run")
	}
}
