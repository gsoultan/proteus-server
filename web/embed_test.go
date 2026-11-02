// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandler(t *testing.T) {
	h := Handler()
	for _, p := range []string{"/", "/t/abc", "/new", "/assets/missing.js"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if Built() {
			if rec.Code != http.StatusOK || (!strings.HasPrefix(p, "/assets/") && !strings.Contains(rec.Body.String(), "<div id=\"root\">")) {
				t.Fatalf("%s: %d %q", p, rec.Code, rec.Body.String()[:min(rec.Body.Len(), 80)])
			}
		} else if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "not built") {
			t.Fatalf("%s without a build: %d %q", p, rec.Code, rec.Body.String())
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("POST = %d, want 404", rec.Code)
	}
}
