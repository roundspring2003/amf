package sbi

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	amf_context "github.com/free5gc/amf/internal/context"
	"github.com/free5gc/amf/internal/sbi/consumer"
	"github.com/free5gc/amf/internal/sbi/processor"
	"github.com/free5gc/amf/pkg/factory"
	"github.com/free5gc/openapi/models"
	"github.com/gin-gonic/gin"
)

// Mock Processor
type mockEventExposureAmf struct{}

func (m *mockEventExposureAmf) Start()               {}
func (m *mockEventExposureAmf) Terminate()           {}
func (m *mockEventExposureAmf) SetLogEnable(bool)    {}
func (m *mockEventExposureAmf) SetLogLevel(string)   {}
func (m *mockEventExposureAmf) SetReportCaller(bool) {}

func (m *mockEventExposureAmf) Context() *amf_context.AMFContext {
	return nil
}

func (m *mockEventExposureAmf) Config() *factory.Config {
	return nil
}

func (m *mockEventExposureAmf) Consumer() *consumer.Consumer {
	return nil
}

func (m *mockEventExposureAmf) Processor() *processor.Processor {
	proc, _ := processor.NewProcessor(m)
	return proc
}

// Validate JSON response
func assertEventExposureJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
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

// Setup Router
func setupTestEventExposureRouter(s *Server) *gin.Engine {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	routes := s.getEventexposureRoutes()

	for _, route := range routes {
		switch route.Method {
		case http.MethodGet:
			r.GET(route.Pattern, route.APIFunc)
		case http.MethodPost:
			r.POST(route.Pattern, route.APIFunc)
		case http.MethodPatch:
			r.PATCH(route.Pattern, route.APIFunc)
		case http.MethodDelete:
			r.DELETE(route.Pattern, route.APIFunc)
		default:
			panic("unsupported HTTP method")
		}
	}

	return r
}

// Test 1: Route Coverage
func TestEventExposure_RouteDefinitions(t *testing.T) {
	s := &Server{}
	routes := s.getEventexposureRoutes()

	expected := []Route{
		{Method: http.MethodGet, Pattern: "/"},
		{Name: "DeleteSubscription", Method: http.MethodDelete, Pattern: "/subscriptions/:subscriptionId"},
		{Name: "ModifySubscription", Method: http.MethodPatch, Pattern: "/subscriptions/:subscriptionId"},
		{Name: "CreateSubscription", Method: http.MethodPost, Pattern: "/subscriptions"},
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
func TestEventExposure_HelloWorld(t *testing.T) {
	s := &Server{}
	router := setupTestEventExposureRouter(s)

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

// Test 3: ModifySubscription - Body read error → 500
type badReaderEE struct{}

func (b badReaderEE) Read(p []byte) (int, error) { return 0, errors.New("read error") }

func TestEventExposure_ModifySubscription_ReadError(t *testing.T) {
	s := &Server{}
	router := setupTestEventExposureRouter(s)

	req := httptest.NewRequest(http.MethodPatch, "/subscriptions/sub123", badReaderEE{})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}

	body := assertEventExposureJSON(t, w)
	if body["status"].(float64) != 500 {
		t.Fatalf("expected status 500, got %v", body["status"])
	}
}

// Test 4: ModifySubscription - Bad JSON → 400
func TestEventExposure_ModifySubscription_BadJSON(t *testing.T) {
	s := &Server{}
	router := setupTestEventExposureRouter(s)

	req := httptest.NewRequest(http.MethodPatch, "/subscriptions/sub123",
		bytes.NewBufferString("{bad json"))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	body := assertEventExposureJSON(t, w)
	if body["status"].(float64) != 400 {
		t.Fatalf("expected status 400, got %v", body["status"])
	}
}

// Test 5: ModifySubscription - Subscription not found → 404
func TestEventExposure_ModifySubscription_NotFound(t *testing.T) {
	amfContext := amf_context.GetSelf()
	amfContext.Name = "TestAMF"

	mockAmf := &mockEventExposureAmf{}
	s := &Server{
		ServerAmf: mockAmf,
	}
	router := setupTestEventExposureRouter(s)

	jsonBody := `{"optionItem":[{"op":"replace","path":"/expiry","value":"2025-12-31T23:59:59Z"}]}`
	req := httptest.NewRequest(http.MethodPatch, "/subscriptions/non-existent-sub",
		bytes.NewBufferString(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}

	body := assertEventExposureJSON(t, w)
	if cause, ok := body["cause"].(string); !ok || cause != "SUBSCRIPTION_NOT_FOUND" {
		t.Errorf("expected cause=SUBSCRIPTION_NOT_FOUND, got %v", body["cause"])
	}
}

// Test 6: ModifySubscription - With valid subscription
func TestEventExposure_ModifySubscription_WithSubscription(t *testing.T) {
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
	supi := "imsi-208930000000001"
	ue := amfContext.NewAmfUe(supi)
	ue.EventSubscriptionsInfo = make(map[string]*amf_context.AmfUeEventSubscription)

	// Create subscription
	subscriptionID := "sub-001"
	expiry := time.Now().Add(1 * time.Hour)
	contextSub := &amf_context.AMFContextEventSubscription{
		EventSubscription: models.AmfEventSubscription{
			EventList: []models.AmfEvent{
				{
					Type: models.AmfEventType_LOCATION_REPORT,
				},
			},
			EventNotifyUri: "http://callback.example.com",
			Supi:           supi,
		},
		UeSupiList: []string{supi},
		Expiry:     &expiry,
	}
	amfContext.NewEventSubscription(subscriptionID, contextSub)

	mockAmf := &mockEventExposureAmf{}
	s := &Server{
		ServerAmf: mockAmf,
	}
	router := setupTestEventExposureRouter(s)

	newExpiry := time.Now().Add(2 * time.Hour).Format(time.RFC3339)
	jsonBody := `{"optionItem":[{"op":"replace","path":"/expiry","value":"` + newExpiry + `"}]}`
	req := httptest.NewRequest(http.MethodPatch, "/subscriptions/"+subscriptionID,
		bytes.NewBufferString(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d\nBody: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if subscription, ok := response["subscription"].(map[string]interface{}); !ok || subscription == nil {
		t.Error("expected subscription in response")
	}

	// Cleanup
	amfContext.DeleteEventSubscription(subscriptionID)
}

// Test 6: CreateSubscription - Body read error → 500
func TestEventExposure_CreateSubscription_ReadError(t *testing.T) {
	s := &Server{}
	router := setupTestEventExposureRouter(s)

	req := httptest.NewRequest(http.MethodPost, "/subscriptions", badReaderEE{})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}

	body := assertEventExposureJSON(t, w)
	if body["status"].(float64) != 500 {
		t.Fatalf("expected status 500, got %v", body["status"])
	}
}

// Test 6b: CreateSubscription - Bad JSON → 400
func TestEventExposure_CreateSubscription_BadJSON(t *testing.T) {
	s := &Server{}
	router := setupTestEventExposureRouter(s)

	req := httptest.NewRequest(http.MethodPost, "/subscriptions",
		bytes.NewBufferString("{bad json"))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	body := assertEventExposureJSON(t, w)
	if body["status"].(float64) != 400 {
		t.Fatalf("expected status 400, got %v", body["status"])
	}
}

// Test 7: CreateSubscription - Missing subscription → 400
func TestEventExposure_CreateSubscription_EmptySubscription(t *testing.T) {
	amfContext := amf_context.GetSelf()
	amfContext.Name = "TestAMF"

	mockAmf := &mockEventExposureAmf{}
	s := &Server{
		ServerAmf: mockAmf,
	}
	router := setupTestEventExposureRouter(s)

	jsonBody := `{}`
	req := httptest.NewRequest(http.MethodPost, "/subscriptions",
		bytes.NewBufferString(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	body := assertEventExposureJSON(t, w)
	if cause, ok := body["cause"].(string); !ok || cause != "SUBSCRIPTION_EMPTY" {
		t.Errorf("expected cause=SUBSCRIPTION_EMPTY, got %v", body["cause"])
	}
}

// Test 8: CreateSubscription - UE not found → 403
func TestEventExposure_CreateSubscription_UENotFound(t *testing.T) {
	amfContext := amf_context.GetSelf()
	amfContext.Name = "TestAMF"

	mockAmf := &mockEventExposureAmf{}
	s := &Server{
		ServerAmf: mockAmf,
	}
	router := setupTestEventExposureRouter(s)

	jsonBody := `{"subscription":{"eventList":[{"type":"LOCATION_REPORT"}],"eventNotifyUri":"http://callback.example.com","supi":"imsi-999999999999999"}}`
	req := httptest.NewRequest(http.MethodPost, "/subscriptions",
		bytes.NewBufferString(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}

	body := assertEventExposureJSON(t, w)
	if cause, ok := body["cause"].(string); !ok || cause != "UE_NOT_SERVED_BY_AMF" {
		t.Errorf("expected cause=UE_NOT_SERVED_BY_AMF, got %v", body["cause"])
	}
}

// Test 9: CreateSubscription - With UE → Success
func TestEventExposure_CreateSubscription_WithUE(t *testing.T) {
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
	supi := "imsi-208930000000002"
	ue := amfContext.NewAmfUe(supi)
	ue.EventSubscriptionsInfo = make(map[string]*amf_context.AmfUeEventSubscription)
	ue.Location = models.UserLocation{
		NrLocation: &models.NrLocation{
			Tai: &models.Tai{
				PlmnId: &models.PlmnId{
					Mcc: "208",
					Mnc: "93",
				},
				Tac: "000001",
			},
		},
	}

	mockAmf := &mockEventExposureAmf{}
	s := &Server{
		ServerAmf: mockAmf,
	}
	router := setupTestEventExposureRouter(s)

	jsonBody := `{"subscription":{"eventList":[{"type":"LOCATION_REPORT","immediateFlag":true}],"eventNotifyUri":"http://callback.example.com","supi":"` + supi + `"}}`
	req := httptest.NewRequest(http.MethodPost, "/subscriptions",
		bytes.NewBufferString(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d\nBody: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	subscriptionID, ok := response["subscriptionId"].(string)
	if !ok || subscriptionID == "" {
		t.Error("expected subscriptionId in response")
	}

	// Verify subscription was created
	if sub, exists := amfContext.FindEventSubscription(subscriptionID); !exists {
		t.Error("subscription should be created in context")
	} else {
		if len(sub.UeSupiList) != 1 || sub.UeSupiList[0] != supi {
			t.Errorf("expected UE %s in subscription, got %v", supi, sub.UeSupiList)
		}
	}

	// Cleanup
	if subscriptionID != "" {
		amfContext.DeleteEventSubscription(subscriptionID)
	}
}

// Test 10: DeleteSubscription - Not found → 404
func TestEventExposure_DeleteSubscription_NotFound(t *testing.T) {
	amfContext := amf_context.GetSelf()
	amfContext.Name = "TestAMF"

	mockAmf := &mockEventExposureAmf{}
	s := &Server{
		ServerAmf: mockAmf,
	}
	router := setupTestEventExposureRouter(s)

	req := httptest.NewRequest(http.MethodDelete, "/subscriptions/non-existent", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}

	body := assertEventExposureJSON(t, w)
	if cause, ok := body["cause"].(string); !ok || cause != "SUBSCRIPTION_NOT_FOUND" {
		t.Errorf("expected cause=SUBSCRIPTION_NOT_FOUND, got %v", body["cause"])
	}
}

// Test 11: DeleteSubscription - With subscription → Success
func TestEventExposure_DeleteSubscription_Success(t *testing.T) {
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
	supi := "imsi-208930000000003"
	ue := amfContext.NewAmfUe(supi)
	ue.EventSubscriptionsInfo = make(map[string]*amf_context.AmfUeEventSubscription)

	// Create subscription
	subscriptionID := "sub-002"
	contextSub := &amf_context.AMFContextEventSubscription{
		EventSubscription: models.AmfEventSubscription{
			EventList: []models.AmfEvent{
				{
					Type: models.AmfEventType_LOCATION_REPORT,
				},
			},
			EventNotifyUri: "http://callback.example.com",
			Supi:           supi,
		},
		UeSupiList: []string{supi},
	}
	amfContext.NewEventSubscription(subscriptionID, contextSub)

	// Add subscription to UE
	ue.EventSubscriptionsInfo[subscriptionID] = &amf_context.AmfUeEventSubscription{
		EventSubscription: &models.ExtAmfEventSubscription{
			EventList:      contextSub.EventSubscription.EventList,
			EventNotifyUri: contextSub.EventSubscription.EventNotifyUri,
			Supi:           supi,
		},
		Timestamp: time.Now().UTC(),
	}

	mockAmf := &mockEventExposureAmf{}
	s := &Server{
		ServerAmf: mockAmf,
	}
	router := setupTestEventExposureRouter(s)

	req := httptest.NewRequest(http.MethodDelete, "/subscriptions/"+subscriptionID, nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d\nBody: %s", w.Code, w.Body.String())
	}

	// Verify subscription was deleted
	if _, exists := amfContext.FindEventSubscription(subscriptionID); exists {
		t.Error("subscription should be deleted from context")
	}

	// Verify subscription was removed from UE
	if _, exists := ue.EventSubscriptionsInfo[subscriptionID]; exists {
		t.Error("subscription should be removed from UE")
	}
}
