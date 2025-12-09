package sbi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupTestRouterMbsCommunication(s *Server) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	routes := s.getMbsCommunicationRoutes()
	for _, route := range routes {
		switch route.Method {
		case http.MethodGet:
			r.GET(route.Pattern, route.APIFunc)
		case http.MethodPost:
			r.POST(route.Pattern, route.APIFunc)
		default:
			panic("unsupported HTTP method in test")
		}
	}
	return r
}

func assertJSONResponseMbsCommunication(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if !strings.Contains(w.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("expected JSON Content-Type, got %s", w.Header().Get("Content-Type"))
	}
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	if len(body) != 0 {
		t.Fatalf("expected empty JSON {}, got %v", body)
	}
}

// 測試路由定義 (使用標準化 Map 檢查)
func TestMbsCommunication_RouteDefinitions(t *testing.T) {
	s := &Server{}
	routes := s.getMbsCommunicationRoutes()

	expected := map[string]struct {
		Method string
		Name   string
	}{
		"/": {
			Method: http.MethodGet,
		},
		"/n2-messages/transfer": {
			Method: http.MethodPost,
			Name:   "N2MessageTransfer",
		},
	}

	if len(routes) != len(expected) {
		t.Fatalf("expected %d routes, got %d", len(expected), len(routes))
	}

	for _, r := range routes {
		exp, exists := expected[r.Pattern]
		if !exists {
			t.Errorf("Unexpected route pattern: %s", r.Pattern)
			continue
		}
		if r.Method != exp.Method {
			t.Errorf("Pattern %s: Method mismatch. Got %s, Want %s", r.Pattern, r.Method, exp.Method)
		}
		if exp.Name != "" && r.Name != exp.Name {
			t.Errorf("Pattern %s: Name mismatch. Got %s, Want %s", r.Pattern, r.Name, exp.Name)
		}
	}
}

func TestMbsCommunication_HelloWorld(t *testing.T) {
	s := &Server{}
	router := setupTestRouterMbsCommunication(s)

	// 使用 Helper
	w := PerformJSONRequest(router, http.MethodGet, "/", "")

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}
	if w.Body.String() != "Hello World!" {
		t.Fatalf("expected body %q, got %q", "Hello World!", w.Body.String())
	}
}

// 測試 /n2-messages/transfer
func TestMbsCommunication_N2MessageTransfer(t *testing.T) {
	s := &Server{}
	router := setupTestRouterMbsCommunication(s)

	// 使用 Helper
	w := PerformJSONRequest(router, http.MethodPost, "/n2-messages/transfer", "")

	if w.Code != http.StatusNotImplemented {
		t.Fatalf("expected status %d, got %d", http.StatusNotImplemented, w.Code)
	}
	assertJSONResponseMbsCommunication(t, w)
}
