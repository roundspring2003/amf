package sbi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	amf_context "github.com/free5gc/amf/internal/context"
	"github.com/free5gc/amf/internal/logger"
	"github.com/free5gc/amf/internal/sbi/processor"
	"github.com/free5gc/openapi/models"
	"github.com/free5gc/util/fsm"
)

// ==========================================
// 1. Initialization
// ==========================================

func init() {
	// 防止 logger.Fatalf 導致測試程序直接退出 (Exit status 1)
	if logger.CallbackLog.Logger != nil {
		logger.CallbackLog.Logger.ExitFunc = func(int) {}
	}
}

// setupTestRouterCallback initializes a Gin engine with Callback routes for testing.
func setupTestRouterCallback(s *Server) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	routes := s.getHttpCallBackRoutes()

	for _, route := range routes {
		r.Handle(route.Method, route.Pattern, route.APIFunc)
	}
	return r
}

// ==========================================
// 2. Test Cases
// ==========================================

// TestCallbackRoutes_Definitions verifies correct route table definitions for Callbacks.
func TestCallbackRoutes_Definitions(t *testing.T) {
	s := &Server{}
	routes := s.getHttpCallBackRoutes()

	expected := map[string]struct {
		Method string
		Name   string
	}{
		"/": {
			Method: http.MethodGet,
		},
		"/am-policy/:polAssoId/update": {
			Method: http.MethodPost,
			Name:   "AmPolicyControlUpdateNotifyUpdate",
		},
		"/am-policy/:polAssoId/terminate": {
			Method: http.MethodPost,
			Name:   "AmPolicyControlUpdateNotifyTerminate",
		},
		"/smContextStatus/:supi/:pduSessionId": {
			Method: http.MethodPost,
			Name:   "SmContextStatusNotify",
		},
		"/n1-message-notify": {
			Method: http.MethodPost,
			Name:   "N1MessageNotify",
		},
		"/deregistration/:ueid": {
			Method: http.MethodPost,
			Name:   "HandleDeregistrationNotification",
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

// TestHTTPAmPolicyControlUpdateNotifyUpdate tests basic JSON validation logic.
func TestHTTPAmPolicyControlUpdateNotifyUpdate(t *testing.T) {
	dummyCtx := &amf_context.AMFContext{}
	mockProcAmf := &MockProcessorAmf{fakeContext: dummyCtx}
	realProc, _ := processor.NewProcessor(mockProcAmf)

	mockServerAmf := &MockServerAmf{realProcessor: realProc}
	s := &Server{ServerAmf: mockServerAmf}

	router := setupTestRouterCallback(s)
	url := "/am-policy/test-pol-id/update"

	// Case 1: Malformed JSON
	t.Run("Malformed JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, url, bytes.NewBufferString(`{ "invalid": `))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Malformed request syntax")
	})

	// Case 2: Valid JSON structure
	t.Run("Valid JSON structure", func(t *testing.T) {
		validJSON := `{}`
		req := httptest.NewRequest(http.MethodPost, url, bytes.NewBufferString(validJSON))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code == http.StatusBadRequest {
			assert.NotContains(t, w.Body.String(), "Malformed request syntax")
		} else {
			// If not 400 Syntax Error, it means Handler successfully parsed JSON
			// and passed it to Processor (which likely returns 404 or 500 here)
			t.Logf("Valid JSON passed Handler. Processor returned: %d", w.Code)
		}
	})
}

// TestHTTPAmPolicyControlUpdateNotifyUpdate_Procedure tests the full logic using Global State Injection.
// It verifies that the AMF correctly updates the UE context based on the Policy Update.
func TestHTTPAmPolicyControlUpdateNotifyUpdate_Procedure(t *testing.T) {
	// --- 1. Setup Environment ---
	self := amf_context.GetSelf()

	dummyCtx := &amf_context.AMFContext{}
	mockProcAmf := &MockProcessorAmf{fakeContext: dummyCtx}
	realProc, _ := processor.NewProcessor(mockProcAmf)
	mockServerAmf := &MockServerAmf{realProcessor: realProc}
	s := &Server{ServerAmf: mockServerAmf}
	router := setupTestRouterCallback(s)

	// --- 2. Test Case: Context Not Found (404) ---
	t.Run("Context Not Found", func(t *testing.T) {
		url := "/am-policy/unknown-policy-id/update"
		body := `{}`
		req := httptest.NewRequest(http.MethodPost, url, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), "CONTEXT_NOT_FOUND")
	})

	// --- 3. Test Case: Success Update (204) ---
	t.Run("Success Update and Verify Context", func(t *testing.T) {
		// A. Inject Fake UE into Global Pool
		targetPolAssoId := "pol-12345"
		targetSupi := "imsi-208930000000001"

		fakeUe := &amf_context.AmfUe{
			Supi:                targetSupi,
			PolicyAssociationId: targetPolAssoId,
			AmPolicyAssociation: &models.PcfAmPolicyControlPolicyAssociation{
				Rfsp: 0,
			},
			// Initialize State and Tai to prevent panics in Processor logic
			State: map[models.AccessType]*fsm.State{
				models.AccessType__3_GPP_ACCESS: fsm.NewState(amf_context.Registered),
			},
			Tai: models.Tai{
				PlmnId: &models.PlmnId{Mcc: "466", Mnc: "92"},
				Tac:    "000001",
			},
		}
		self.UePool.Store(targetSupi, fakeUe)

		// Also map PolicyAssociationId to the UE (Processor often looks up by PolAssoId)
		// Assuming amf_context has a map for this or iterates UePool.
		// In free5gc, usually it iterates UePool to match PolicyAssociationId if not directly mapped.

		// B. Prepare Request Data
		// Simulate PCF requesting to update RFSP index and Triggers
		jsonBody := `{
            "rfsp": 10,
            "triggers": ["LOC_CH"]
        }`
		url := "/am-policy/" + targetPolAssoId + "/update"
		req := httptest.NewRequest(http.MethodPost, url, bytes.NewBufferString(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// C. Execute Request
		router.ServeHTTP(w, req)

		// D. Verify HTTP Response (204 No Content is expected for successful update)
		assert.Equal(t, http.StatusNoContent, w.Code)

		// E. Verify Side Effects (Check if global state is updated)
		assert.Equal(t, int32(10), fakeUe.AmPolicyAssociation.Rfsp, "UE RFSP should be updated to 10")
		assert.True(t, fakeUe.RequestTriggerLocationChange, "Location Change Trigger should be set to true")

		// F. Teardown
		self.UePool.Delete(targetSupi)
	})
}

// TestHTTPHandleDeregistrationNotification_Success tests the deregistration flow.
func TestHTTPHandleDeregistrationNotification_Success(t *testing.T) {
	self := amf_context.GetSelf()
	targetSupi := "imsi-callback-test-01"

	fakeUe := &amf_context.AmfUe{
		Supi: targetSupi,
	}
	self.UePool.Store(targetSupi, fakeUe)

	dummyCtx := &amf_context.AMFContext{}
	mockProcAmf := &MockProcessorAmf{fakeContext: dummyCtx}
	realProc, _ := processor.NewProcessor(mockProcAmf)
	mockServerAmf := &MockServerAmf{realProcessor: realProc}
	s := &Server{ServerAmf: mockServerAmf}

	router := setupTestRouterCallback(s)

	deregData := models.DeregistrationData{
		DeregReason: models.DeregistrationReason_UE_INITIAL_REGISTRATION,
		AccessType:  models.AccessType__3_GPP_ACCESS,
	}
	bodyBytes, _ := json.Marshal(deregData)

	req := httptest.NewRequest(http.MethodPost, "/deregistration/"+targetSupi, bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	_, exists := self.UePool.Load(targetSupi)
	assert.False(t, exists, "UE should be removed from the pool")

	self.UePool.Delete(targetSupi)
}