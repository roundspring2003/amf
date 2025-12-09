package sbi

import (
	"net/http"
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
	// Arrange
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

	// Assert
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
	// Arrange: Setup environment and Prepare Data
	s, _ := NewTestServer(t)
	router := setupTestRouterOAM(s)

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

	ManageTestUE(t, fakeUe)

	// 4. Test: Get Specific UE
	t.Run("Get Specific UE Context", func(t *testing.T) {
		// Act
		w := PerformJSONRequest(router, http.MethodGet, "/registered-ue-context/"+targetSupi, "")

		// Assert: Verify CORS headers
		assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
		assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "GET")

		// Assert: Verify Data
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), targetSupi)
		assert.Contains(t, w.Body.String(), "466")
	})

	// 5. Test: List All UEs
	t.Run("List All UE Contexts", func(t *testing.T) {
		// Act
		w := PerformJSONRequest(router, http.MethodGet, "/registered-ue-context", "")

		// Assert: Verify CORS and Data
		assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), targetSupi)
	})
}
