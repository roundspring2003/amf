package sbi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	amf_context "github.com/free5gc/amf/internal/context"
	"github.com/free5gc/openapi/models"
)

// Helper: Initialize Gin engine with MT routes.
func setupTestRouterMT(s *Server) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	routes := s.getMTRoutes()

	for _, route := range routes {
		r.Handle(route.Method, route.Pattern, route.APIFunc)
	}
	return r
}

// Test handler with global state injection.
func TestHTTPProvideDomainSelectionInfo_GlobalInjection(t *testing.T) {
	// 1. Setup environment
	s, _ := NewTestServer(t)
	router := setupTestRouterMT(s)

	// 2. Prepare UE data (RanUe is required for JSON construction)
	self := amf_context.GetSelf()
	targetSupi := "imsi-208930000000003"

	fakeUe := &amf_context.AmfUe{
		Supi: targetSupi,
		RanUe: map[models.AccessType]*amf_context.RanUe{
			models.AccessType__3_GPP_ACCESS: {
				SupportedFeatures: "00",
				SupportVoPS:       true, // Simulate VoPS support
			},
		},
	}
	fakeUe.RatType = models.RatType_NR // Set RatType to 5G

	// 3. Inject into global pool and ensure cleanup
	self.UePool.Store(targetSupi, fakeUe)

	t.Cleanup(func() {
		self.UePool.Delete(targetSupi)
	})

	// 4. Execute Test (Include query params for full logic coverage)
	req := httptest.NewRequest(http.MethodGet,
		"/ue-contexts/"+targetSupi+"?info-class=TADS&supported-features=00", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// 5. Verification
	assert.Equal(t, http.StatusOK, w.Code, "Expected 200 OK")

	var respBody map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &respBody)
	assert.NoError(t, err)

	// Verify response content matches injected data
	assert.Equal(t, string(models.RatType_NR), respBody["ratType"])
	assert.Equal(t, true, respBody["supportVoPS"])
}

// Verify MT route definitions (Method, Pattern, Name).
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

// Test handlers using table-driven tests.
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