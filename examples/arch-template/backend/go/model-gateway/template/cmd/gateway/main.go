package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"example.com/service/internal/gateway"
	"example.com/service/internal/httpapi"
	"example.com/service/internal/infrastructure/upstream"
	"example.com/service/internal/routing"
)

func main() {
	url := os.Getenv("UPSTREAM_URL")
	if url == "" {
		log.Fatal("UPSTREAM_URL must name an authorized chat-completions endpoint")
	}
	pool := routing.New([]routing.Target{{ID: "primary", Model: "chat", URL: url, Version: 1}})
	client := &http.Client{Timeout: 2 * time.Minute}
	server := &http.Server{Addr: "127.0.0.1:8081", ReadHeaderTimeout: 5 * time.Second, Handler: httpapi.NewRelay(gateway.New(pool, upstream.New(client)))}
	log.Print("gateway: http://127.0.0.1:8081/v1/chat/completions")
	log.Fatal(server.ListenAndServe())
}
