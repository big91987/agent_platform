package platform

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

func TestPageScriptsAreServed(t *testing.T) {
	h := &Server{}
	page := httptest.NewRecorder()
	h.static(page, httptest.NewRequest(http.MethodGet, "/", nil))
	for _, match := range regexp.MustCompile(`<script src="([^"]+)"`).FindAllStringSubmatch(page.Body.String(), -1) {
		response := httptest.NewRecorder()
		h.static(response, httptest.NewRequest(http.MethodGet, match[1], nil))
		if response.Code != http.StatusOK || !strings.Contains(response.Header().Get("Content-Type"), "javascript") || response.Body.Len() == 0 {
			t.Fatalf("page script %s is unavailable: %d %s", match[1], response.Code, response.Header().Get("Content-Type"))
		}
	}
}
