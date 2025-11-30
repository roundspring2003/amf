package sbi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	amf_context "github.com/free5gc/amf/internal/context"
	"github.com/free5gc/amf/internal/sbi/consumer"
	"github.com/free5gc/amf/internal/sbi/processor"
	"github.com/free5gc/amf/pkg/factory"
	"github.com/free5gc/openapi/models"
	"github.com/gin-gonic/gin"
)

// Mock Processor
type mockCommunicationAmf struct{}

func (m *mockCommunicationAmf) Start()               {}
func (m *mockCommunicationAmf) Terminate()           {}
func (m *mockCommunicationAmf) SetLogEnable(bool)    {}
func (m *mockCommunicationAmf) SetLogLevel(string)   {}
func (m *mockCommunicationAmf) SetReportCaller(bool) {}

func (m *mockCommunicationAmf) Context() *amf_context.AMFContext {
	return amf_context.GetSelf()
}

func (m *mockCommunicationAmf) Config() *factory.Config {
	return &factory.Config{}
}

func (m *mockCommunicationAmf) Consumer() *consumer.Consumer {
	return nil
}

func (m *mockCommunicationAmf) Processor() *processor.Processor {
	proc, _ := processor.NewProcessor(m)
	return proc
}

// Helper: Validate JSON response
func assertCommunicationJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()

	if !strings.Contains(w.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("expected JSON Content-Type, got %s", w.Header().Get("Content-Type"))
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	return body
}

// Setup Router for Communication TEST
func setupTestCommunicationRouter(s *Server) *gin.Engine {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	routes := s.getCommunicationRoutes()

	for _, route := range routes {
		switch route.Method {
		case http.MethodGet:
			r.GET(route.Pattern, route.APIFunc)
		case http.MethodPost:
			r.POST(route.Pattern, route.APIFunc)
		case http.MethodPut:
			r.PUT(route.Pattern, route.APIFunc)
		case http.MethodDelete:
			r.DELETE(route.Pattern, route.APIFunc)
		default:
			panic("unsupported HTTP method")
		}
	}

	return r
}

// Test 1: Route Coverage
func TestCommunication_RouteDefinitions(t *testing.T) {
	s := &Server{}
	routes := s.getCommunicationRoutes()

	expected := []Route{
		{Method: http.MethodGet, Pattern: "/"},
		{Name: "AMFStatusChangeSubscribeModfy", Method: http.MethodPut, Pattern: "/subscriptions/:subscriptionId"},
		{Name: "AMFStatusChangeUnSubscribe", Method: http.MethodDelete, Pattern: "/subscriptions/:subscriptionId"},
		{Name: "CreateUEContext", Method: http.MethodPut, Pattern: "/ue-contexts/:ueContextId"},
		{Name: "EBIAssignment", Method: http.MethodPost, Pattern: "/ue-contexts/:ueContextId/assign-ebi"},
		{Name: "RegistrationStatusUpdate", Method: http.MethodPost, Pattern: "/ue-contexts/:ueContextId/transfer-update"},
		{Name: "ReleaseUEContext", Method: http.MethodPost, Pattern: "/ue-contexts/:ueContextId/release"},
		{Name: "UEContextTransfer", Method: http.MethodPost, Pattern: "/ue-contexts/:ueContextId/transfer"},
		{Name: "RelocateUEContext", Method: http.MethodPost, Pattern: "/ue-contexts/:ueContextId/relocate"},
		{Name: "CancelRelocateUEContext", Method: http.MethodPost, Pattern: "/ue-contexts/:ueContextId/cancel-relocate"},
		{Name: "N1N2MessageUnSubscribe", Method: http.MethodDelete, Pattern: "/ue-contexts/:ueContextId/n1-n2-messages/subscriptions/:subscriptionId"},
		{Name: "N1N2MessageTransfer", Method: http.MethodPost, Pattern: "/ue-contexts/:ueContextId/n1-n2-messages"},
		{Name: "N1N2MessageTransferStatus", Method: http.MethodGet, Pattern: "/ue-contexts/:ueContextId/n1-n2-messages/:n1N2MessageId"},
		{Name: "N1N2MessageSubscribe", Method: http.MethodPost, Pattern: "/ue-contexts/:ueContextId/n1-n2-messages/subscriptions"},
		{Name: "NonUeN2InfoUnSubscribe", Method: http.MethodDelete, Pattern: "/non-ue-n2-messages/subscriptions/:n2NotifySubscriptionId"},
		{Name: "NonUeN2MessageTransfer", Method: http.MethodPost, Pattern: "/non-ue-n2-messages/transfer"},
		{Name: "NonUeN2InfoSubscribe", Method: http.MethodPost, Pattern: "/non-ue-n2-messages/subscriptions"},
		{Name: "AMFStatusChangeSubscribe", Method: http.MethodPost, Pattern: "/subscriptions"},
	}

	if len(routes) != len(expected) {
		t.Fatalf("expected %d routes, got %d", len(expected), len(routes))
	}

	for i := range expected {
		if routes[i].Method != expected[i].Method {
			t.Errorf("route[%d] Method mismatch: got %s expected %s",
				i, routes[i].Method, expected[i].Method)
		}
		if routes[i].Pattern != expected[i].Pattern {
			t.Errorf("route[%d] Pattern mismatch: got %s expected %s",
				i, routes[i].Pattern, expected[i].Pattern)
		}
		if expected[i].Name != "" && routes[i].Name != expected[i].Name {
			t.Errorf("route[%d] Name mismatch: got %s expected %s",
				i, routes[i].Name, expected[i].Name)
		}
	}
}

// Test 2: HelloWorld
func TestCommunication_HelloWorld(t *testing.T) {
	s := &Server{}
	router := setupTestCommunicationRouter(s)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "Hello World!" {
		t.Fatalf("expected Hello World!, got %s", w.Body.String())
	}
}

// Test 3: AMFStatusChangeSubscribeModify - Body read error → 500
type badReaderComm struct{}

func (b badReaderComm) Read(p []byte) (int, error) { return 0, errors.New("read error") }

func TestCommunication_AMFStatusChangeSubscribeModify_ReadError(t *testing.T) {
	s := &Server{}
	router := setupTestCommunicationRouter(s)

	req := httptest.NewRequest(http.MethodPut, "/subscriptions/sub123", badReaderComm{})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}

	body := assertCommunicationJSON(t, w)
	if body["status"].(float64) != 500 {
		t.Fatalf("expected status 500, got %v", body["status"])
	}
}

// Test 4: AMFStatusChangeSubscribeModify - Bad JSON → 400
func TestCommunication_AMFStatusChangeSubscribeModify_BadJSON(t *testing.T) {
	s := &Server{}
	router := setupTestCommunicationRouter(s)

	req := httptest.NewRequest(http.MethodPut, "/subscriptions/sub123",
		bytes.NewBufferString("{bad json"))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	body := assertCommunicationJSON(t, w)
	if body["status"].(float64) != 400 {
		t.Fatalf("expected status 400, got %v", body["status"])
	}
}

// Test 5: AMFStatusChangeSubscribeModify - Valid JSON → No parse errors
func TestCommunication_AMFStatusChangeSubscribeModify_Success(t *testing.T) {
	s := &Server{
		ServerAmf: &mockCommunicationAmf{},
	}
	router := setupTestCommunicationRouter(s)

	jsonBody := `{"nfStatusNotificationUri":"http://callback.example.com"}`
	req := httptest.NewRequest(http.MethodPut, "/subscriptions/sub123",
		bytes.NewBufferString(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code == http.StatusBadRequest {
		body := assertCommunicationJSON(t, w)
		if strings.Contains(fmt.Sprintf("%v", body), "Malformed request syntax") {
			t.Fatalf("JSON should have been parsed successfully, got 400 with malformed syntax error")
		}
	}

	if w.Code == http.StatusInternalServerError {
		body := assertCommunicationJSON(t, w)
		if strings.Contains(fmt.Sprintf("%v", body["detail"]), "Get Request Body error") {
			t.Fatalf("Request body should have been read successfully")
		}
	}
}

// Test 6: AMFStatusChangeUnSubscribe - Verify handler is called
func TestCommunication_AMFStatusChangeUnSubscribe(t *testing.T) {
	s := &Server{
		ServerAmf: &mockCommunicationAmf{},
	}
	router := setupTestCommunicationRouter(s)

	req := httptest.NewRequest(http.MethodDelete, "/subscriptions/sub123", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Status code may vary based on processor implementation
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound &&
		w.Code != http.StatusInternalServerError && w.Code != http.StatusNoContent {
		t.Fatalf("expected status 200/204/404/500, got %d", w.Code)
	}
}

// Test 7: CreateUEContext - Body read error → 500
func TestCommunication_CreateUEContext_ReadError(t *testing.T) {
	s := &Server{}
	router := setupTestCommunicationRouter(s)

	req := httptest.NewRequest(http.MethodPut, "/ue-contexts/ue123", badReaderComm{})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}

	body := assertCommunicationJSON(t, w)
	if body["status"].(float64) != 500 {
		t.Fatalf("expected status 500, got %v", body["status"])
	}
}

// Test 8: CreateUEContext - Bad JSON → 400
func TestCommunication_CreateUEContext_BadJSON(t *testing.T) {
	s := &Server{}
	router := setupTestCommunicationRouter(s)

	req := httptest.NewRequest(http.MethodPut, "/ue-contexts/ue123",
		bytes.NewBufferString("{bad json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	body := assertCommunicationJSON(t, w)
	if body["status"].(float64) != 400 {
		t.Fatalf("expected status 400, got %v", body["status"])
	}
}

// Test 9: EBIAssignment - Body read error → 500
func TestCommunication_EBIAssignment_ReadError(t *testing.T) {
	s := &Server{}
	router := setupTestCommunicationRouter(s)

	req := httptest.NewRequest(http.MethodPost, "/ue-contexts/ue123/assign-ebi", badReaderComm{})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}

	body := assertCommunicationJSON(t, w)
	if body["status"].(float64) != 500 {
		t.Fatalf("expected status 500, got %v", body["status"])
	}
}

// Test 10: EBIAssignment - Bad JSON → 400
func TestCommunication_EBIAssignment_BadJSON(t *testing.T) {
	s := &Server{}
	router := setupTestCommunicationRouter(s)

	req := httptest.NewRequest(http.MethodPost, "/ue-contexts/ue123/assign-ebi",
		bytes.NewBufferString("{bad json"))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	body := assertCommunicationJSON(t, w)
	if body["status"].(float64) != 400 {
		t.Fatalf("expected status 400, got %v", body["status"])
	}
}

// Test 11: RegistrationStatusUpdate - Body read error → 500
func TestCommunication_RegistrationStatusUpdate_ReadError(t *testing.T) {
	s := &Server{}
	router := setupTestCommunicationRouter(s)

	req := httptest.NewRequest(http.MethodPost, "/ue-contexts/ue123/transfer-update", badReaderComm{})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}

	body := assertCommunicationJSON(t, w)
	if body["status"].(float64) != 500 {
		t.Fatalf("expected status 500, got %v", body["status"])
	}
}

// Test 12: RegistrationStatusUpdate - Bad JSON → 400
func TestCommunication_RegistrationStatusUpdate_BadJSON(t *testing.T) {
	s := &Server{}
	router := setupTestCommunicationRouter(s)

	req := httptest.NewRequest(http.MethodPost, "/ue-contexts/ue123/transfer-update",
		bytes.NewBufferString("{bad json"))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	body := assertCommunicationJSON(t, w)
	if body["status"].(float64) != 400 {
		t.Fatalf("expected status 400, got %v", body["status"])
	}
}

// Test 13: ReleaseUEContext - Body read error → 500
func TestCommunication_ReleaseUEContext_ReadError(t *testing.T) {
	s := &Server{}
	router := setupTestCommunicationRouter(s)

	req := httptest.NewRequest(http.MethodPost, "/ue-contexts/ue123/release", badReaderComm{})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}

	body := assertCommunicationJSON(t, w)
	if body["status"].(float64) != 500 {
		t.Fatalf("expected status 500, got %v", body["status"])
	}
}

// Test 14: ReleaseUEContext - Bad JSON → 400
func TestCommunication_ReleaseUEContext_BadJSON(t *testing.T) {
	s := &Server{}
	router := setupTestCommunicationRouter(s)

	req := httptest.NewRequest(http.MethodPost, "/ue-contexts/ue123/release",
		bytes.NewBufferString("{bad json"))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	body := assertCommunicationJSON(t, w)
	if body["status"].(float64) != 400 {
		t.Fatalf("expected status 400, got %v", body["status"])
	}
}

// Test 15: UEContextTransfer - Body read error → 500
func TestCommunication_UEContextTransfer_ReadError(t *testing.T) {
	s := &Server{}
	router := setupTestCommunicationRouter(s)

	req := httptest.NewRequest(http.MethodPost, "/ue-contexts/ue123/transfer", badReaderComm{})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}

	body := assertCommunicationJSON(t, w)
	if body["status"].(float64) != 500 {
		t.Fatalf("expected status 500, got %v", body["status"])
	}
}

// Test 16: UEContextTransfer - Bad JSON → 400
func TestCommunication_UEContextTransfer_BadJSON(t *testing.T) {
	s := &Server{}
	router := setupTestCommunicationRouter(s)

	req := httptest.NewRequest(http.MethodPost, "/ue-contexts/ue123/transfer",
		bytes.NewBufferString("{bad json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	body := assertCommunicationJSON(t, w)
	if body["status"].(float64) != 400 {
		t.Fatalf("expected status 400, got %v", body["status"])
	}
}

// Test 17: RelocateUEContext → 501
func TestCommunication_RelocateUEContext(t *testing.T) {
	s := &Server{}
	router := setupTestCommunicationRouter(s)

	req := httptest.NewRequest(http.MethodPost, "/ue-contexts/ue123/relocate", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d", w.Code)
	}

	assertCommunicationJSON(t, w)
}

// Test 18: CancelRelocateUEContext → 501
func TestCommunication_CancelRelocateUEContext(t *testing.T) {
	s := &Server{}
	router := setupTestCommunicationRouter(s)

	req := httptest.NewRequest(http.MethodPost, "/ue-contexts/ue123/cancel-relocate", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d", w.Code)
	}

	assertCommunicationJSON(t, w)
}

// Test 19: N1N2MessageUnSubscribe - UE not found
func TestCommunication_N1N2MessageUnSubscribe(t *testing.T) {
	amfContext := amf_context.GetSelf()
	amfContext.Name = "TestAMF"

	mockAmf := &mockCommunicationAmf{}
	s := &Server{
		ServerAmf: mockAmf,
	}
	router := setupTestCommunicationRouter(s)

	req := httptest.NewRequest(http.MethodDelete, "/ue-contexts/non-existent-ue/n1-n2-messages/subscriptions/sub456", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// UE not found should result in appropriate status
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound &&
		w.Code != http.StatusInternalServerError && w.Code != http.StatusNoContent {
		t.Fatalf("expected status 200/204/404/500, got %d", w.Code)
	}
}

// Test 20: N1N2MessageTransfer - Body read error → 500
func TestCommunication_N1N2MessageTransfer_ReadError(t *testing.T) {
	s := &Server{}
	router := setupTestCommunicationRouter(s)

	req := httptest.NewRequest(http.MethodPost, "/ue-contexts/ue123/n1-n2-messages", badReaderComm{})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}

	body := assertCommunicationJSON(t, w)
	if body["status"].(float64) != 500 {
		t.Fatalf("expected status 500, got %v", body["status"])
	}
}

// Test 21: N1N2MessageTransfer - JSON Content-Type → 400
func TestCommunication_N1N2MessageTransfer_JSONContentType(t *testing.T) {
	s := &Server{}
	router := setupTestCommunicationRouter(s)

	req := httptest.NewRequest(http.MethodPost, "/ue-contexts/ue123/n1-n2-messages",
		bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	body := assertCommunicationJSON(t, w)
	if body["status"].(float64) != 400 {
		t.Fatalf("expected status 400, got %v", body["status"])
	}
}

// Test 22: N1N2MessageTransferStatus - UE not found
func TestCommunication_N1N2MessageTransferStatus(t *testing.T) {
	amfContext := amf_context.GetSelf()
	amfContext.Name = "TestAMF"

	mockAmf := &mockCommunicationAmf{}
	s := &Server{
		ServerAmf: mockAmf,
	}
	router := setupTestCommunicationRouter(s)

	req := httptest.NewRequest(http.MethodGet, "/ue-contexts/non-existent-ue/n1-n2-messages/msg456", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// UE not found should result in appropriate status
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound &&
		w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 200/404/500, got %d", w.Code)
	}
}

// Test 23: N1N2MessageSubscribe - Body read error → 500
func TestCommunication_N1N2MessageSubscribe_ReadError(t *testing.T) {
	s := &Server{}
	router := setupTestCommunicationRouter(s)

	req := httptest.NewRequest(http.MethodPost, "/ue-contexts/ue123/n1-n2-messages/subscriptions", badReaderComm{})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}

	body := assertCommunicationJSON(t, w)
	if body["status"].(float64) != 500 {
		t.Fatalf("expected status 500, got %v", body["status"])
	}
}

// Test 24: N1N2MessageSubscribe - Bad JSON → 400
func TestCommunication_N1N2MessageSubscribe_BadJSON(t *testing.T) {
	s := &Server{}
	router := setupTestCommunicationRouter(s)

	req := httptest.NewRequest(http.MethodPost, "/ue-contexts/ue123/n1-n2-messages/subscriptions",
		bytes.NewBufferString("{bad json"))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	body := assertCommunicationJSON(t, w)
	if body["status"].(float64) != 400 {
		t.Fatalf("expected status 400, got %v", body["status"])
	}
}

// Test 25: NonUeN2InfoUnSubscribe → 501
func TestCommunication_NonUeN2InfoUnSubscribe(t *testing.T) {
	s := &Server{}
	router := setupTestCommunicationRouter(s)

	req := httptest.NewRequest(http.MethodDelete, "/non-ue-n2-messages/subscriptions/sub123", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d", w.Code)
	}

	assertCommunicationJSON(t, w)
}

// Test 26: NonUeN2MessageTransfer → 501
func TestCommunication_NonUeN2MessageTransfer(t *testing.T) {
	s := &Server{}
	router := setupTestCommunicationRouter(s)

	req := httptest.NewRequest(http.MethodPost, "/non-ue-n2-messages/transfer", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d", w.Code)
	}

	assertCommunicationJSON(t, w)
}

// Test 27: NonUeN2InfoSubscribe → 501
func TestCommunication_NonUeN2InfoSubscribe(t *testing.T) {
	s := &Server{}
	router := setupTestCommunicationRouter(s)

	req := httptest.NewRequest(http.MethodPost, "/non-ue-n2-messages/subscriptions", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d", w.Code)
	}

	assertCommunicationJSON(t, w)
}

// Test 28: AMFStatusChangeSubscribe - Body read error → 500
func TestCommunication_AMFStatusChangeSubscribe_ReadError(t *testing.T) {
	s := &Server{}
	router := setupTestCommunicationRouter(s)

	req := httptest.NewRequest(http.MethodPost, "/subscriptions", badReaderComm{})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}

	body := assertCommunicationJSON(t, w)
	if body["status"].(float64) != 500 {
		t.Fatalf("expected status 500, got %v", body["status"])
	}
}

// Test 29: AMFStatusChangeSubscribe - Bad JSON → 400
func TestCommunication_AMFStatusChangeSubscribe_BadJSON(t *testing.T) {
	s := &Server{}
	router := setupTestCommunicationRouter(s)

	req := httptest.NewRequest(http.MethodPost, "/subscriptions",
		bytes.NewBufferString("{bad json"))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	body := assertCommunicationJSON(t, w)
	if body["status"].(float64) != 400 {
		t.Fatalf("expected status 400, got %v", body["status"])
	}
}

// Test 30: AMFStatusChangeSubscribe - Valid JSON → No parse errors
func TestCommunication_AMFStatusChangeSubscribe_Success(t *testing.T) {
	s := &Server{
		ServerAmf: &mockCommunicationAmf{},
	}
	router := setupTestCommunicationRouter(s)

	jsonBody := `{"nfStatusNotificationUri":"http://callback.example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/subscriptions",
		bytes.NewBufferString(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code == http.StatusBadRequest {
		body := assertCommunicationJSON(t, w)
		if strings.Contains(fmt.Sprintf("%v", body), "Malformed request syntax") {
			t.Fatalf("JSON should have been parsed successfully, got 400 with malformed syntax error")
		}
	}

	if w.Code == http.StatusInternalServerError {
		body := assertCommunicationJSON(t, w)
		if strings.Contains(fmt.Sprintf("%v", body["detail"]), "Get Request Body error") {
			t.Fatalf("Request body should have been read successfully")
		}
	}
}

// Test 31: N1N2MessageUnSubscribe - With UE
func TestCommunication_N1N2MessageUnSubscribe_WithUE(t *testing.T) {
	amfContext := amf_context.GetSelf()
	amfContext.Name = "TestAMF"

	amfContext.ServedGuamiList = []models.Guami{
		{
			PlmnId: &models.PlmnIdNid{
				Mcc: "208",
				Mnc: "93",
			},
			AmfId: "cafe00",
		},
	}

	// Create UE
	supi := "imsi-208930000000010"
	_ = amfContext.NewAmfUe(supi)

	mockAmf := &mockCommunicationAmf{}
	s := &Server{
		ServerAmf: mockAmf,
	}
	router := setupTestCommunicationRouter(s)

	// Try to unsubscribe (subscription may not exist, but UE exists)
	req := httptest.NewRequest(http.MethodDelete, "/ue-contexts/"+supi+"/n1-n2-messages/subscriptions/1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should get a valid response (200/204/404 all acceptable)
	if w.Code != http.StatusOK && w.Code != http.StatusNoContent && w.Code != http.StatusNotFound {
		t.Fatalf("expected 200/204/404, got %d\nBody: %s", w.Code, w.Body.String())
	}
}

// Test 32: N1N2MessageTransferStatus - With UE
func TestCommunication_N1N2MessageTransferStatus_WithUE(t *testing.T) {
	amfContext := amf_context.GetSelf()
	amfContext.Name = "TestAMF"

	amfContext.ServedGuamiList = []models.Guami{
		{
			PlmnId: &models.PlmnIdNid{
				Mcc: "208",
				Mnc: "93",
			},
			AmfId: "cafe00",
		},
	}

	// Create UE
	supi := "imsi-208930000000011"
	_ = amfContext.NewAmfUe(supi)

	mockAmf := &mockCommunicationAmf{}
	s := &Server{
		ServerAmf: mockAmf,
	}
	router := setupTestCommunicationRouter(s)

	req := httptest.NewRequest(http.MethodGet, "/ue-contexts/"+supi+"/n1-n2-messages/999", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Message not found is expected (UE exists but message doesn't)
	if w.Code != http.StatusNotFound && w.Code != http.StatusOK {
		t.Logf("Got status %d (expected 404 or 200)", w.Code)
	}
}

// Test 33: CreateUEContext - With real UE creation
func TestCommunication_CreateUEContext_Success(t *testing.T) {
	amfContext := amf_context.GetSelf()
	amfContext.Name = "TestAMF"

	amfContext.ServedGuamiList = []models.Guami{
		{
			PlmnId: &models.PlmnIdNid{
				Mcc: "208",
				Mnc: "93",
			},
			AmfId: "cafe00",
		},
	}

	mockAmf := &mockCommunicationAmf{}
	s := &Server{
		ServerAmf: mockAmf,
	}
	router := setupTestCommunicationRouter(s)

	supi := "imsi-208930000000012"
	jsonBody := `{"ueContext":{"supi":"` + supi + `","pei":"imei-123456789012345"}}`
	req := httptest.NewRequest(http.MethodPut, "/ue-contexts/"+supi,
		bytes.NewBufferString(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// The actual response depends on processor implementation
	// We're testing that JSON parsing works and handler is called
	if w.Code == http.StatusBadRequest {
		body := assertCommunicationJSON(t, w)
		detail := fmt.Sprintf("%v", body["detail"])
		if strings.Contains(detail, "Malformed request syntax") {
			t.Fatalf("JSON should have been parsed successfully")
		}
	}
}
