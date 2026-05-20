package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDeprecationMiddlewareMatches(t *testing.T) {
	cases := []struct {
		path       string
		wantHeader bool
	}{
		{"/api/knowledge/list", true},
		{"/api/knowledge/unit/list", true},
		{"/api/knowledge/segments/create", true},
		{"/api/bot/knowledge/update", true},
		{"/api/bot/prompt/optimize", true},
		{"/api/bot/explore/list", true},
		{"/api/user/login", false}, // 非 deprecated
		{"/api/bot/prompt/update", false},
		{"/api/msg/push", false},
	}

	m := NewDeprecationMiddleware()
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, c.path, nil)
			handler := m.Handle(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			handler(rr, req)

			got := rr.Header().Get("Deprecation") == "true"
			if got != c.wantHeader {
				t.Errorf("path=%s Deprecation header=%v, want=%v", c.path, got, c.wantHeader)
			}
			if c.wantHeader {
				if sunset := rr.Header().Get("Sunset"); sunset != "v1.2" {
					t.Errorf("path=%s Sunset=%q, want v1.2", c.path, sunset)
				}
				if rr.Header().Get("Link") == "" {
					t.Errorf("path=%s missing Link header", c.path)
				}
			}
		})
	}
}
