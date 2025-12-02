package sbi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	amf_context "github.com/free5gc/amf/internal/context"
	"github.com/free5gc/openapi/models"
	"github.com/free5gc/util/fsm"
)

// Helper: Initialize Gin engine with OAM routes.
func setupTestRouterOAM(s *Server) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	routes := s.getOAMRoutes()

	for _, route := range routes {
		r.Handle(route.Method, route.Pattern, route.APIFunc)
	}
	return r
}

// Verify OAM route definitions (Method, Pattern, Name).
func TestOAMRoutes_Definitions(t *testing.T) {
	s := &Server{}
	routes := s.getOAMRoutes()

	expected := map[string]struct {
		Method string
		Name   string
	}{
		"/": {
			Method: http.MethodGet,
		},
		"/registered-ue-context": {
			Method: http.MethodGet,
			Name:   "RegisteredUEContext",
		},
		"/registered-ue-context/:supi": {
			Method: http.MethodGet,
			Name:   "RegisteredUEContext",
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

// Test OAM handler using global state injection (CORS and Data verification).
func TestHTTPRegisteredUEContext_GlobalInjection(t *testing.T) {
	// 1. Setup environment
	s, _ := NewTestServer(t)
	router := setupTestRouterOAM(s)

	// 2. Prepare UE data (State and TAI are required to prevent panics)
	targetSupi := "imsi-208930000000004"
	fakeUe := &amf_context.AmfUe{
		Supi: targetSupi,
		State: map[models.AccessType]*fsm.State{
			models.AccessType__3_GPP_ACCESS:    fsm.NewState(amf_context.Registered),
			models.AccessType_NON_3_GPP_ACCESS: fsm.NewState(amf_context.Deregistered),
		},
		Tai: models.Tai{
			PlmnId: &models.PlmnId{
				Mcc: "466",
				Mnc: "92",
			},
			Tac: "000001",
		},
	}

	// 3. Inject into global pool and ensure cleanup
	self := amf_context.GetSelf()
	self.UePool.Store(targetSupi, fakeUe)

	t.Cleanup(func() {
		self.UePool.Delete(targetSupi)
	})

	// 4. Test: Get Specific UE
	t.Run("Get Specific UE Context", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/registered-ue-context/"+targetSupi, nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Verify CORS headers
		assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
		assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "GET")

		// Verify Data
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), targetSupi)
		assert.Contains(t, w.Body.String(), "466")
	})

	// 5. Test: List All UEs
	t.Run("List All UE Contexts", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/registered-ue-context", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Verify CORS and Data
		assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), targetSupi)
	})
}