package sbi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

//
// -------------------------------------------------------------------
// Helper Functions
// -------------------------------------------------------------------
//

func setupTestMbsRouter(s *Server) *gin.Engine {
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

//
// -------------------------------------------------------------------
// Tests
// -------------------------------------------------------------------
//

func TestMbsBroadcast_RouteDefinitions(t *testing.T) {
	s := &Server{}
	routes := s.getMbsBroadcastRoutes()

	if len(routes) != 4 {
		t.Fatalf("expected 4 routes, got %d", len(routes))
	}
}

func TestMbsBroadcast_Endpoints(t *testing.T) {
	testCases := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
		expectedBody   string
		checkJSON      bool
	}{
		{
			name:           "health check endpoint",
			method:         http.MethodGet,
			path:           "/",
			expectedStatus: http.StatusOK,
			expectedBody:   "Hello World!",
			checkJSON:      false,
		},
		{
			name:           "context create - not implemented",
			method:         http.MethodPost,
			path:           "/mbs-contexts",
			expectedStatus: http.StatusNotImplemented,
			checkJSON:      true,
		},
		{
			name:           "context update - not implemented",
			method:         http.MethodPost,
			path:           "/mbs-contexts/abc/update",
			expectedStatus: http.StatusNotImplemented,
			checkJSON:      true,
		},
		{
			name:           "context release - not implemented",
			method:         http.MethodDelete,
			path:           "/mbs-contexts/abc",
			expectedStatus: http.StatusNotImplemented,
			checkJSON:      true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := &Server{}
			router := setupTestMbsRouter(s)
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Fatalf("expected status %d, got %d", tc.expectedStatus, w.Code)
			}

			if tc.expectedBody != "" {
				if w.Body.String() != tc.expectedBody {
					t.Fatalf("expected body %q, got %q", tc.expectedBody, w.Body.String())
				}
			}

			if tc.checkJSON {
				assertJSONResponse(t, w)
			}
		})
	}
}
