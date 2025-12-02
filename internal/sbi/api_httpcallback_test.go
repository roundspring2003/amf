package sbi

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	amf_context "github.com/free5gc/amf/internal/context"
	"github.com/free5gc/amf/internal/logger"
	"github.com/free5gc/openapi/models"
	"github.com/free5gc/util/fsm"
)

// ==========================================
// 1. Initialization
// ==========================================

func init() {
	// Prevent logger.Fatal from exiting test process (Exit status 1)
	if logger.CallbackLog.Logger != nil {
		logger.CallbackLog.Logger.ExitFunc = func(int) {}
	}
}

// Helper: Initialize Gin engine with Callback routes.
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

// Verify Callback route definitions.
func TestCallbackRoutes_Definitions(t *testing.T) {
	s, _ := NewTestServer(t)
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

	assert.Equal(t, len(expected), len(routes), "Route count mismatch")

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

// Test JSON validation logic for Policy Update.
func TestHTTPAmPolicyControlUpdateNotifyUpdate_Validation(t *testing.T) {
	s, _ := NewTestServer(t)
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

		// 400 Bad Request is only expected for Malformed JSON.
		// If received here, ensure it is NOT "Malformed request syntax".
		if w.Code == http.StatusBadRequest {
			assert.NotContains(t, w.Body.String(), "Malformed request syntax")
		}
	})
}

// Test full logic for Policy Update with global state injection.
func TestHTTPAmPolicyControlUpdateNotifyUpdate_Procedure(t *testing.T) {
	// 1. Setup Environment
	s, _ := NewTestServer(t)
	router := setupTestRouterCallback(s)
	self := amf_context.GetSelf()

	// 2. Case: Context Not Found (404)
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

	// 3. Case: Success Update (204)
	t.Run("Success Update and Verify Context", func(t *testing.T) {
		// A. Inject Fake UE
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

		// Ensure cleanup
		t.Cleanup(func() {
			self.UePool.Delete(targetSupi)
		})

		// B. Prepare Request Data (Update RFSP and Triggers)
		jsonBody := `{
			"rfsp": 10,
			"triggers": ["LOC_CH"]
		}`
		url := "/am-policy/" + targetPolAssoId + "/update"
		req := httptest.NewRequest(http.MethodPost, url, bytes.NewBufferString(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// C. Execute
		router.ServeHTTP(w, req)

		// D. Verify Response
		assert.Equal(t, http.StatusNoContent, w.Code)

		// E. Verify Side Effects (Global state update)
		assert.Equal(t, int32(10), fakeUe.AmPolicyAssociation.Rfsp, "UE RFSP should be updated to 10")
		// Note: Verify specific boolean field if needed (e.g., RequestTriggerLocationChange)
	})
}

// Test JSON validation logic for Policy Terminate.
func TestHTTPAmPolicyControlUpdateNotifyTerminate_Validation(t *testing.T) {
	s, _ := NewTestServer(t)
	router := setupTestRouterCallback(s)
	url := "/am-policy/test-pol-id/terminate"

	// Case 1: Malformed JSON
	t.Run("Malformed JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, url, bytes.NewBufferString(`{ "cause": `))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Malformed request syntax")
	})

	// Case 2: Valid JSON structure
	t.Run("Valid JSON structure", func(t *testing.T) {
		validJSON := `{ "cause": "UNSPECIFIED" }`
		req := httptest.NewRequest(http.MethodPost, url, bytes.NewBufferString(validJSON))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code == http.StatusBadRequest {
			assert.Fail(t, "Should not return 400 for valid JSON")
		}
	})
}

// Test full logic for Policy Terminate (404 and 204).
func TestHTTPAmPolicyControlUpdateNotifyTerminate_Procedure(t *testing.T) {
	// 1. Setup
	s, _ := NewTestServer(t)
	router := setupTestRouterCallback(s)
	self := amf_context.GetSelf()

	// 2. Case: Context Not Found (404)
	t.Run("Context Not Found", func(t *testing.T) {
		url := "/am-policy/unknown-pol-id/terminate"
		body := `{ "cause": "UNSPECIFIED" }`
		req := httptest.NewRequest(http.MethodPost, url, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), "CONTEXT_NOT_FOUND")
	})

	// 3. Case: Success Termination (204)
	t.Run("Success Termination", func(t *testing.T) {
		// A. Inject Fake UE
		targetPolAssoId := "pol-99999"
		targetSupi := "imsi-208930000000099"

		fakeUe := &amf_context.AmfUe{
			Supi:                targetSupi,
			PolicyAssociationId: targetPolAssoId,
			AmPolicyAssociation: &models.PcfAmPolicyControlPolicyAssociation{},
		}
		self.UePool.Store(targetSupi, fakeUe)

		t.Cleanup(func() {
			self.UePool.Delete(targetSupi)
		})

		// B. Prepare Request
		url := "/am-policy/" + targetPolAssoId + "/terminate"
		body := `{ "cause": "UE_SUBSCRIPTION_DELETED" }`
		req := httptest.NewRequest(http.MethodPost, url, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// C. Execute
		router.ServeHTTP(w, req)

		// D. Verify
		assert.Equal(t, http.StatusNoContent, w.Code)
	})
}

// Test JSON validation logic for SM Context Status Notify.
func TestHTTPSmContextStatusNotify_Validation(t *testing.T) {
	s, _ := NewTestServer(t)
	router := setupTestRouterCallback(s)
	url := "/smContextStatus/imsi-12345/10"

	// Case 1: Malformed JSON
	t.Run("Malformed JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, url, bytes.NewBufferString(`{ "invalid": `))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Malformed request syntax")
	})
}

// Test full logic for SM Context Status Notify (404, 400, 204).
func TestHTTPSmContextStatusNotify_Procedure(t *testing.T) {
	// 1. Setup
	s, _ := NewTestServer(t)
	router := setupTestRouterCallback(s)
	self := amf_context.GetSelf()

	// 2. Case: UE Not Found (404)
	t.Run("UE Not Found", func(t *testing.T) {
		url := "/smContextStatus/imsi-99999/10"
		body := `{
			"statusInfo": {
				"resourceStatus": "RELEASED"
			}
		}`
		req := httptest.NewRequest(http.MethodPost, url, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), "CONTEXT_NOT_FOUND")
	})

	// 3. Case: SM Context Not Found (404)
	t.Run("SM Context Not Found", func(t *testing.T) {
		targetSupi := "imsi-208930000000001"
		fakeUe := &amf_context.AmfUe{
			Supi:        targetSupi,
			ProducerLog: logger.ProducerLog, // Required to prevent nil pointer panic
		}
		self.UePool.Store(targetSupi, fakeUe)
		t.Cleanup(func() { self.UePool.Delete(targetSupi) })

		url := "/smContextStatus/" + targetSupi + "/99"
		body := `{
			"statusInfo": {
				"resourceStatus": "RELEASED"
			}
		}`
		req := httptest.NewRequest(http.MethodPost, url, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), "PDUSessionID[99] Not Found")
	})

	// 4. Case: Invalid Resource Status (400)
	t.Run("Invalid Resource Status", func(t *testing.T) {
		targetSupi := "imsi-208930000000002"
		pduSessionID := int32(5)
		fakeUe := &amf_context.AmfUe{Supi: targetSupi}

		// Inject SM Context
		smContext := amf_context.NewSmContext(pduSessionID)
		fakeUe.StoreSmContext(pduSessionID, smContext)

		self.UePool.Store(targetSupi, fakeUe)
		t.Cleanup(func() { self.UePool.Delete(targetSupi) })

		url := "/smContextStatus/" + targetSupi + "/5"
		body := `{
			"statusInfo": {
				"resourceStatus": "UPDATED" 
			}
		}`
		req := httptest.NewRequest(http.MethodPost, url, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "INVALID_MSG_FORMAT")
	})

	// 5. Case: Success Release (204)
	t.Run("Success Release", func(t *testing.T) {
		targetSupi := "imsi-208930000000003"
		pduSessionID := int32(10)
		fakeUe := &amf_context.AmfUe{
			Supi:        targetSupi,
			ProducerLog: logger.ProducerLog, // Required to prevent nil pointer panic
		}

		smContext := amf_context.NewSmContext(pduSessionID)
		smContext.SetAccessType(models.AccessType__3_GPP_ACCESS) // Required by DeleteSmContext
		fakeUe.StoreSmContext(pduSessionID, smContext)

		self.UePool.Store(targetSupi, fakeUe)
		t.Cleanup(func() { self.UePool.Delete(targetSupi) })

		// Pre-check
		_, existsBefore := fakeUe.SmContextFindByPDUSessionID(pduSessionID)
		assert.True(t, existsBefore, "SmContext should exist before test")

		url := "/smContextStatus/" + targetSupi + "/10"
		body := `{
			"statusInfo": {
				"resourceStatus": "RELEASED",
				"cause": "REL_DUE_TO_DUPLICATE_SESSION_ID"
			}
		}`
		req := httptest.NewRequest(http.MethodPost, url, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Verify Response
		assert.Equal(t, http.StatusNoContent, w.Code)

		// Verify Side Effect (Removal)
		_, existsAfter := fakeUe.SmContextFindByPDUSessionID(pduSessionID)
		assert.False(t, existsAfter, "SmContext should be removed after RELEASED notification")
	})
}

// Test successful deregistration flow.
func TestHTTPHandleDeregistrationNotification_Success(t *testing.T) {
	// 1. Setup
	s, _ := NewTestServer(t)
	router := setupTestRouterCallback(s)
	self := amf_context.GetSelf()

	targetSupi := "imsi-callback-test-01"

	// 2. Prepare Data
	fakeUe := &amf_context.AmfUe{
		Supi: targetSupi,
	}
	self.UePool.Store(targetSupi, fakeUe)

	t.Cleanup(func() {
		self.UePool.Delete(targetSupi)
	})

	// 3. Prepare Request
	deregData := models.DeregistrationData{
		DeregReason: models.DeregistrationReason_UE_INITIAL_REGISTRATION,
		AccessType:  models.AccessType__3_GPP_ACCESS,
	}
	bodyBytes, _ := json.Marshal(deregData)

	req := httptest.NewRequest(http.MethodPost, "/deregistration/"+targetSupi, bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// 4. Execute
	router.ServeHTTP(w, req)

	// 5. Verify
	assert.Equal(t, http.StatusNoContent, w.Code)

	// Verify removal from pool
	_, exists := self.UePool.Load(targetSupi)
	assert.False(t, exists, "UE should be removed from the pool")
}

// Test Multipart format validation.
func TestHTTPN1MessageNotify_Validation(t *testing.T) {
	s, _ := NewTestServer(t)
	router := setupTestRouterCallback(s)
	url := "/n1-message-notify"

	// Case 1: Malformed Body (Non-Multipart/Non-JSON)
	t.Run("Malformed Body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, url, bytes.NewBufferString(`{ invalid-json-structure `))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Malformed request syntax")
	})
}

// Test Multipart parsing and procedure execution.
func TestHTTPN1MessageNotify_Procedure(t *testing.T) {
	s, _ := NewTestServer(t)
	router := setupTestRouterCallback(s)

	t.Run("Valid Multipart Request", func(t *testing.T) {
		// 1. Build Multipart Body
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		// Part 1: JSON Data
		h := make(textproto.MIMEHeader)
		h.Set("Content-Type", "application/json")
		h.Set("Content-ID", "jsonData")
		part, err := writer.CreatePart(h)
		assert.NoError(t, err)

		notification := models.N1MessageNotification{
			N1NotifySubscriptionId: "sub-123",
			N1MessageContainer: &models.N1MessageContainer{
				N1MessageClass: models.N1MessageClass_SM,
				N1MessageContent: &models.RefToBinaryData{
					ContentId: "n1Msg",
				},
			},
		}
		jsonBytes, _ := json.Marshal(notification)
		part.Write(jsonBytes)

		// Part 2: Binary Data
		h2 := make(textproto.MIMEHeader)
		h2.Set("Content-Type", "application/vnd.3gpp.5gnas")
		h2.Set("Content-ID", "n1Msg")
		part2, err := writer.CreatePart(h2)
		assert.NoError(t, err)
		part2.Write([]byte("fake-n1-message-content"))

		writer.Close()

		// 2. Create Request
		url := "/n1-message-notify"
		req := httptest.NewRequest(http.MethodPost, url, body)
		req.Header.Set("Content-Type", "multipart/related; boundary="+writer.Boundary())
		w := httptest.NewRecorder()

		// 3. Execute
		router.ServeHTTP(w, req)

		// 4. Verify
		// If 400 is returned, ensure it's NOT from binding failure ("Malformed request syntax").
		if w.Code == http.StatusBadRequest {
			assert.NotContains(t, w.Body.String(), "Malformed request syntax",
				"Should pass binding validation. 400 is acceptable if from Processor logic.")
		}
	})
}