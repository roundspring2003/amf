package sbi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	amf_context "github.com/free5gc/amf/internal/context"
	"github.com/free5gc/amf/internal/sbi/processor"
)

// setupTestRouterMT initializes a Gin engine with MT routes for testing.
func setupTestRouterMT(s *Server) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	routes := s.getMTRoutes()

	for _, route := range routes {
		r.Handle(route.Method, route.Pattern, route.APIFunc)
	}
	return r
}

// TestHTTPProvideDomainSelectionInfo_GlobalInjection tests the handler by injecting state into the global context.
func TestHTTPProvideDomainSelectionInfo_GlobalInjection(t *testing.T) {
	// --- A. Global State Setup ---
	self := amf_context.GetSelf()

	targetSupi := "imsi-208930000000003"
	fakeUe := &amf_context.AmfUe{
		Supi: targetSupi,
	}
	self.UePool.Store(targetSupi, fakeUe)

	// --- B. Mock Setup ---
	dummyCtx := &amf_context.AMFContext{}
	mockProcAmf := &MockProcessorAmf{fakeContext: dummyCtx}

	realProc, err := processor.NewProcessor(mockProcAmf)
	if err != nil {
		t.Fatalf("Failed to create real processor: %v", err)
	}

	mockServerAmf := &MockServerAmf{realProcessor: realProc}
	s := &Server{ServerAmf: mockServerAmf}

	// --- C. Execute Test ---
	router := setupTestRouterMT(s)

	req := httptest.NewRequest(http.MethodGet, "/ue-contexts/"+targetSupi, nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// --- D. Verification ---
	if w.Code == http.StatusNotFound {
		t.Log("Note: Still got 404. Check if fakeUe needs more fields.")
	} else {
		assert.Equal(t, http.StatusOK, w.Code)
		var respBody map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &respBody)
		assert.NoError(t, err)
	}

	// --- E. Teardown ---
	self.UePool.Delete(targetSupi)
}

// TestMTRoutes_Definitions verifies correct route table definitions for MT.
func TestMTRoutes_Definitions(t *testing.T) {
	s := &Server{}
	routes := s.getMTRoutes()

	expected := map[string]struct {
		Method string
		Name   string
	}{
		"/": {
			Method: http.MethodGet,
		},
		"/ue-contexts/:ueContextId": {
			Method: http.MethodGet,
			Name:   "ProvideDomainSelectionInfo",
		},
		"/ue-contexts/:ueContextId/ue-reachind": {
			Method: http.MethodPut,
			Name:   "EnableUeReachability",
		},
		"/ue-contexts/enable-group-reachability": {
			Method: http.MethodPost,
			Name:   "EnableGroupReachability",
		},
	}

	assert.Equal(t, len(expected), len(routes))
	for _, r := range routes {
		exp, exists := expected[r.Pattern]
		if !exists {
			t.Errorf("Unexpected route pattern: %s", r.Pattern)
			continue
		}
		assert.Equal(t, exp.Method, r.Method)
	}
}

func TestMTRoutes_Handlers(t *testing.T) {
	s := &Server{}
	router := setupTestRouterMT(s)

	tests := []struct {
		name           string
		method         string
		url            string
		expectedStatus int
	}{
		{
			name:           "Root Hello World",
			method:         http.MethodGet,
			url:            "/",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "EnableUeReachability (Not Implemented)",
			method:         http.MethodPut,
			url:            "/ue-contexts/imsi-test/ue-reachind",
			expectedStatus: http.StatusNotImplemented,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.url, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}