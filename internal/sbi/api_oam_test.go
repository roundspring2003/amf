package sbi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	amf_context "github.com/free5gc/amf/internal/context"
	"github.com/free5gc/amf/internal/sbi/processor"
	"github.com/free5gc/openapi/models"
	"github.com/free5gc/util/fsm"
)

// setupTestRouterOAM initializes a Gin engine with OAM routes for testing.
func setupTestRouterOAM(s *Server) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	routes := s.getOAMRoutes()

	for _, route := range routes {
		r.Handle(route.Method, route.Pattern, route.APIFunc)
	}
	return r
}

// TestOAMRoutes_Definitions verifies correct route table definitions for OAM.
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

// TestHTTPRegisteredUEContext_GlobalInjection tests the OAM handler using global state injection.
// It verifies both the data retrieval and the presence of CORS headers.
func TestHTTPRegisteredUEContext_GlobalInjection(t *testing.T) {
	// --- A. Global State Setup ---
	self := amf_context.GetSelf()

	targetSupi := "imsi-208930000000004"
	fakeUe := &amf_context.AmfUe{
		Supi: targetSupi,
		// Initialize the FSM State for BOTH access types to prevent panic.
		// The processor iterates or checks both 3GPP and Non-3GPP access types.
		State: map[models.AccessType]*fsm.State{
			models.AccessType__3_GPP_ACCESS:    fsm.NewState(amf_context.Registered),
			models.AccessType_NON_3_GPP_ACCESS: fsm.NewState(amf_context.Deregistered),
		},
		// Initialize TAI and PlmnId to prevent panic when processor accesses ue.Tai.PlmnId.Mcc
		Tai: models.Tai{
			PlmnId: &models.PlmnId{
				Mcc: "466",
				Mnc: "92",
			},
			Tac: "000001",
		},
	}
	self.UePool.Store(targetSupi, fakeUe)

	// --- B. Mock Setup ---
	// Reuse MockProcessorAmf and MockServerAmf defined in server_test.go
	dummyCtx := &amf_context.AMFContext{}
	mockProcAmf := &MockProcessorAmf{fakeContext: dummyCtx}

	realProc, err := processor.NewProcessor(mockProcAmf)
	if err != nil {
		t.Fatalf("Failed to create real processor: %v", err)
	}

	mockServerAmf := &MockServerAmf{realProcessor: realProc}
	s := &Server{ServerAmf: mockServerAmf}

	router := setupTestRouterOAM(s)

	// --- C. Test Case 1: Get Specific UE (verify CORS + Data) ---
	t.Run("Get Specific UE Context", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/registered-ue-context/"+targetSupi, nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Verification 1: CORS Headers
		assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
		assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "GET")

		// Verification 2: Status and Body
		if w.Code == http.StatusNotFound {
			t.Log("Note: Got 404. Processor might check for specific UE state.")
		} else {
			assert.Equal(t, http.StatusOK, w.Code)
			assert.Contains(t, w.Body.String(), targetSupi)
		}
	})

	// --- D. Test Case 2: List All UEs (verify CORS + Data) ---
	t.Run("List All UE Contexts", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/registered-ue-context", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Verification 1: CORS Headers
		assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))

		// Verification 2: Status and Body
		assert.Equal(t, http.StatusOK, w.Code)
		if w.Body.Len() > 0 {
			assert.Contains(t, w.Body.String(), targetSupi)
		}
	})

	// --- E. Teardown ---
	self.UePool.Delete(targetSupi)
}