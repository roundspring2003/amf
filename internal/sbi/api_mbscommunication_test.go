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

// 測試路由
func TestMbsCommunication_RouteDefinitions(t *testing.T) {
	s := &Server{}
	routes := s.getMbsCommunicationRoutes()
	expected := []Route{
		{Method: http.MethodGet, Pattern: "/"},
		{Name: "N2MessageTransfer", Method: http.MethodPost, Pattern: "/n2-messages/transfer"},
	}
	if len(routes) != len(expected) {
		t.Fatalf("expected %d routes, got %d", len(expected), len(routes))
	}
	for i := range routes {
		if routes[i].Method != expected[i].Method {
			t.Errorf("route[%d] Method mismatch: got %s, expected %s", i, routes[i].Method, expected[i].Method)
		}
		if routes[i].Pattern != expected[i].Pattern {
			t.Errorf("route[%d] Pattern mismatch: got %s, expected %s", i, routes[i].Pattern, expected[i].Pattern)
		}
		if expected[i].Name != "" && routes[i].Name != expected[i].Name {
			t.Errorf("route[%d] Name mismatch: got %s, expected %s", i, routes[i].Name, expected[i].Name)
		}
	}
}

func TestMbsCommunication_HelloWorld(t *testing.T) {
	s := &Server{}
	router := setupTestRouterMbsCommunication(s)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

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

	req := httptest.NewRequest(http.MethodPost, "/n2-messages/transfer", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Fatalf("expected status %d, got %d", http.StatusNotImplemented, w.Code)
	}
	assertJSONResponseMbsCommunication(t, w)
}
