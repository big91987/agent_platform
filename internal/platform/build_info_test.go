package platform

import (
	"encoding/json"
	"testing"
)

func TestHealthReportsActualBinaryRevision(t *testing.T) {
	server := NewServer(testStore(t), nil, nil, "password", "http://localhost")
	response := requestJSON(t, server, "GET", "/api/health", "", nil)
	var health map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &health); err != nil {
		t.Fatal(err)
	}
	if response.Code != 200 || health["revision"] == nil || health["revision"] == "" {
		t.Fatal("deployed revision cannot be verified", response.Code, health)
	}
}
