package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	accounting "example.com/service/internal/accounting/application"
	"example.com/service/internal/httpapi"
	"example.com/service/internal/infrastructure/sqlite"
)

func main() {
	filename := os.Getenv("QUOTA_DB")
	if filename == "" {
		filename = "quota.db"
	}
	store, err := sqlite.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	service := accounting.New(store)
	if _, err = service.Usage(context.Background(), "demo"); err != nil {
		if !errors.Is(err, accounting.ErrNotFound) {
			log.Fatal(err)
		}
		if err = service.SetLimit(context.Background(), "demo", 100); err != nil {
			log.Fatal(err)
		}
	}
	server := &http.Server{Addr: "127.0.0.1:8080", Handler: httpapi.NewQuota(service), ReadHeaderTimeout: 5 * time.Second}
	log.Print("quota API: http://127.0.0.1:8080/api/quotas/demo")
	log.Fatal(server.ListenAndServe())
}
