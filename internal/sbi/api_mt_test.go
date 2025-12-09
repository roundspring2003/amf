package sbi

import (
	"encoding/json"
	"net/http"
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
	// Arrange: Setup environment
	s, _ := NewTestServer(t)
	router := setupTestRouterMT(s)

	// Arrange: Prepare UE data (RanUe is required for JSON construction)
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

	ManageTestUE(t, fakeUe) // Use Helper

	// Act: Execute Test (Include query params for full logic coverage)
	url := "/ue-contexts/" + targetSupi + "?info-class=TADS&supported-features=00"
	w := PerformJSONRequest(router, http.MethodGet, url, "")

	// Assert: Verification
	assert.Equal(t, http.StatusOK, w.Code, "Expected 200 OK")

	var respBody map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &respBody)
	assert.NoError(t, err)

	// Assert: Verify response content matches injected data
	assert.Equal(t, string(models.RatType_NR), respBody["ratType"])
	assert.Equal(t, true, respBody["supportVoPS"])
}

// Verify MT route definitions (Method, Pattern, Name).
func TestMTRoutes_Definitions(t *testing.T) {
	// Arrange
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

	// Assert
	assert.Equal(t, len(expected), len(routes))

	for _, r := range routes {
		exp, exists := expected[r.Pattern]
		if !exists {
			t.Errorf("Unexpected route pattern: %s", r.Pattern)
			continue
		}
		assert.Equal(t, exp.Method, r.Method, "Method mismatch for %s", r.Pattern)
		if exp.Name != "" {
			assert.Equal(t, exp.Name, r.Name, "Name mismatch for %s", r.Pattern)
		}
	}
}

// Test handlers using table-driven tests.
func TestMTRoutes_Handlers(t *testing.T) {
	// Arrange
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
			// Act
			w := PerformJSONRequest(router, tt.method, tt.url, "")

			// Assert
			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}
