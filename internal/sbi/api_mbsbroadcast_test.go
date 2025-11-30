package sbi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

/* Set up gin engine */
func setupTestRouter(s *Server) *gin.Engine {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	routes := s.getMbsBroadcastRoutes()

	for _, route := range routes {
		switch route.Method {
		case http.MethodGet:
			r.GET(route.Pattern, route.APIFunc)
		case http.MethodPost:
			r.POST(route.Pattern, route.APIFunc)
		case http.MethodDelete:
			r.DELETE(route.Pattern, route.APIFunc)
		default:
			panic("unsupported HTTP method in test")
		}
	}

	return r
}

/* Check JSON response */
func assertJSONResponse(t *testing.T, w *httptest.ResponseRecorder) {
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

/* Route coverage test */
func TestMbsBroadcast_RouteDefinitions(t *testing.T) {
	s := &Server{}
	routes := s.getMbsBroadcastRoutes()

	expected := []Route{
		{Method: http.MethodGet, Pattern: "/"},
		{Name: "ContextCreate", Method: http.MethodPost, Pattern: "/mbs-contexts"},
		{Name: "ContextUpdate", Method: http.MethodPost, Pattern: "/mbs-contexts/:mbsContextRef/update"},
		{Name: "ContextReleas", Method: http.MethodDelete, Pattern: "/mbs-contexts/:mbsContextRef"},
	}

	if len(routes) != len(expected) {
		t.Fatalf("expected %d routes, got %d", len(expected), len(routes))
	}

	for i := range routes {
		if routes[i].Method != expected[i].Method {
			t.Errorf("route[%d] Method mismatch: got %s, expected %s",
				i, routes[i].Method, expected[i].Method)
		}

		if routes[i].Pattern != expected[i].Pattern {
			t.Errorf("route[%d] Pattern mismatch: got %s, expected %s",
				i, routes[i].Pattern, expected[i].Pattern)
		}

		if expected[i].Name != "" && routes[i].Name != expected[i].Name {
			t.Errorf("route[%d] Name mismatch: got %s, expected %s",
				i, routes[i].Name, expected[i].Name)
		}
	}
}

/* Handler test */
func TestMbsBroadcast_HelloWorld(t *testing.T) {
	s := &Server{}
	router := setupTestRouter(s)

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

func TestMbsBroadcast_ContextCreate(t *testing.T) {
	s := &Server{}
	router := setupTestRouter(s)

	req := httptest.NewRequest(http.MethodPost, "/mbs-contexts", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Fatalf("expected status %d, got %d", http.StatusNotImplemented, w.Code)
	}

	assertJSONResponse(t, w)
}

func TestMbsBroadcast_ContextUpdate(t *testing.T) {
	s := &Server{}
	router := setupTestRouter(s)

	req := httptest.NewRequest(http.MethodPost, "/mbs-contexts/abc/update", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Fatalf("expected status %d, got %d", http.StatusNotImplemented, w.Code)
	}

	assertJSONResponse(t, w)
}

func TestMbsBroadcast_ContextRelease(t *testing.T) {
	s := &Server{}
	router := setupTestRouter(s)

	req := httptest.NewRequest(http.MethodDelete, "/mbs-contexts/abc", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Fatalf("expected status %d, got %d", http.StatusNotImplemented, w.Code)
	}

	assertJSONResponse(t, w)
}
