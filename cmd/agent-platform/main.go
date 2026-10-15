package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"agentplatform/internal/platform"
)

func main() {
	showVersion := flag.Bool("version", false, "print binary version and source revision without starting the service")
	address := flag.String("listen", "127.0.0.1:8788", "HTTP listening address")
	data := flag.String("data", ".data", "persistent platform data directory")
	base := flag.String("base-url", "", "public URL for conversation links")
	concurrency := flag.Int("concurrency", 2, "maximum simultaneous local executions")
	binary := flag.String("codex", "codex", "native Codex binary")
	auth := flag.String("auth-home", "", "native Codex authentication directory")
	sandboxConfig := flag.String("sandbox-config", "", "private OpenSandbox service configuration")
	sandboxImage := flag.String("sandbox-image", "agent-platform-runtime:0.1.0", "fixed image for new OpenSandbox Runs")
	sandboxTTL := flag.Int("sandbox-ttl", 3600, "OpenSandbox environment lifetime in seconds")
	sandboxMemory := flag.Int("sandbox-memory-mib", 2048, "memory limit for new OpenSandbox environments in MiB")
	flag.Parse()
	if *showVersion {
		json.NewEncoder(os.Stdout).Encode(map[string]string{"version": platform.Version, "revision": platform.BuildRevision})
		return
	}
	if *concurrency < 1 || *concurrency > 32 {
		log.Fatal("concurrency must be between 1 and 32")
	}
	s, e := platform.OpenStore(*data)
	if e != nil {
		log.Fatal(e)
	}
	defer s.Close()
	if *sandboxConfig != "" {
		if e = s.ConfigureSandbox(*sandboxConfig, *sandboxImage, *sandboxTTL, *sandboxMemory); e != nil {
			log.Fatal(e)
		}
	}
	password := os.Getenv("AGENT_PLATFORM_PASSWORD")
	if password == "" {
		password = "admin"
	}
	if password != "admin" && len(password) < 8 {
		log.Fatal("custom administrator password must contain at least 8 characters")
	}
	agents, e := s.Agents()
	if e != nil {
		log.Fatal(e)
	}
	if len(agents) == 0 {
		_, e = s.SaveAgent(platform.Agent{Name: "本机 Codex", Executor: "codex", Enabled: true, InheritEnv: true, Sandbox: "workspace-write", Skills: []string{}, Instructions: "用清楚的自然语言与用户交流。开始工作时简短说明要做什么；执行较长时说明有用的进展；结束时说明做成了什么。需要用户输入时，明确说明需要用户回答或操作什么。产物写在当前工作目录，便于用户查看和下载。"})
		if e != nil {
			log.Fatal(e)
		}
	}
	x := &platform.Codex{Root: s.Dir, Store: s, Binary: *binary, AuthHome: *auth, Approve: s.AwaitToolApproval}
	scheduler := platform.NewScheduler(s, x, *concurrency)
	scheduler.Start()
	defer scheduler.Close()
	listener, e := net.Listen("tcp", *address)
	if e != nil {
		log.Fatal(e)
	}
	if *base == "" {
		*base = "http://" + listener.Addr().String()
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	handler := platform.NewServer(s, scheduler, x, password, *base)
	workflowDone := make(chan struct{})
	go func() { defer close(workflowDone); handler.RunWorkflows(ctx) }()
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, BaseContext: func(net.Listener) context.Context { return ctx }}
	log.Printf("Agent Platform: %s", *base)
	log.Printf("Administrator account: admin; existing account passwords are preserved")
	log.Printf("Data: %s; local execution capacity: %d", s.Dir, *concurrency)
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	select {
	case <-ctx.Done():
	case e := <-done:
		if e != nil && e != http.ErrServerClosed {
			fmt.Fprintln(os.Stderr, e)
		}
		cancel()
	}
	cancel()
	<-workflowDone
	scheduler.Close()
	shutdown, c := context.WithTimeout(context.Background(), 10*time.Second)
	defer c()
	server.Shutdown(shutdown)
}
