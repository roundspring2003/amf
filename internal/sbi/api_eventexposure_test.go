package sbi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
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

//
// -------------------------------------------------------------------
// Helper Functions
// -------------------------------------------------------------------
//

type mockEventExposureAmf struct {
	ctx *amf_context.AMFContext
}

func (m *mockEventExposureAmf) Start()                           {}
func (m *mockEventExposureAmf) Terminate()                       {}
func (m *mockEventExposureAmf) SetLogEnable(bool)                {}
func (m *mockEventExposureAmf) SetLogLevel(string)               {}
func (m *mockEventExposureAmf) SetReportCaller(bool)             {}
func (m *mockEventExposureAmf) Context() *amf_context.AMFContext { return m.ctx }
func (m *mockEventExposureAmf) Config() *factory.Config          { return nil }
func (m *mockEventExposureAmf) Consumer() *consumer.Consumer     { return nil }

func (m *mockEventExposureAmf) Processor() *processor.Processor {
	proc, _ := processor.NewProcessor(m)
	return proc
}

type badReaderEE struct{}

func (b badReaderEE) Read(p []byte) (int, error) { return 0, errors.New("read error") }

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

func newMockEventExposureAmf() *mockEventExposureAmf {
	ctx := amf_context.GetSelf()
	ctx.Name = "TestAMF"
	ctx.ServedGuamiList = []models.Guami{
		{
			PlmnId: &models.PlmnIdNid{Mcc: "208", Mnc: "93"},
			AmfId:  "cafe00",
		},
	}
	return &mockEventExposureAmf{ctx: ctx}
}

func createEventExposureTestUE(ctx *amf_context.AMFContext, supi string) *amf_context.AmfUe {
	ue := ctx.NewAmfUe(supi)
	ue.EventSubscriptionsInfo = make(map[string]*amf_context.AmfUeEventSubscription)
	ue.Location = models.UserLocation{
		NrLocation: &models.NrLocation{
			Tai: &models.Tai{
				PlmnId: &models.PlmnId{Mcc: "208", Mnc: "93"},
				Tac:    "000001",
			},
		},
	}
	return ue
}

//
// -------------------------------------------------------------------
// Tests
// -------------------------------------------------------------------
//

func TestEventExposure_RouteDefinitions(t *testing.T) {
	s := &Server{}
	routes := s.getEventexposureRoutes()

	if len(routes) != 4 {
		t.Fatalf("expected 4 routes, got %d", len(routes))
	}
}

func TestEventExposure_BasicEndpoints(t *testing.T) {
	testCases := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "health check endpoint",
			method:         http.MethodGet,
			path:           "/",
			expectedStatus: http.StatusOK,
			expectedBody:   "Hello World!",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := &Server{}
			router := setupTestEventExposureRouter(s)
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Fatalf("expected %d, got %d", tc.expectedStatus, w.Code)
			}

			if tc.expectedBody != "" {
				if w.Body.String() != tc.expectedBody {
					t.Fatalf("expected %q, got %q", tc.expectedBody, w.Body.String())
				}
			}
		})
	}
}

func TestEventExposure_ModifySubscription_ErrorCases(t *testing.T) {
	testCases := []struct {
		name           string
		setupServer    func() *Server
		requestBody    io.Reader
		expectedStatus int
		expectedCause  string
	}{
		{
			name: "request body read error",
			setupServer: func() *Server {
				return &Server{}
			},
			requestBody:    badReaderEE{},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "invalid JSON format",
			setupServer: func() *Server {
				return &Server{}
			},
			requestBody:    bytes.NewBufferString("{bad json"),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "subscription not found",
			setupServer: func() *Server {
				return &Server{ServerAmf: newMockEventExposureAmf()}
			},
			requestBody:    bytes.NewBufferString(`{"optionItem":[{"op":"replace","path":"/expiry","value":"2025-12-31T23:59:59Z"}]}`),
			expectedStatus: http.StatusNotFound,
			expectedCause:  "SUBSCRIPTION_NOT_FOUND",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := tc.setupServer()
			router := setupTestEventExposureRouter(s)
			req := httptest.NewRequest(http.MethodPatch, "/subscriptions/test-sub", tc.requestBody)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Fatalf("expected status %d, got %d\nBody: %s",
					tc.expectedStatus, w.Code, w.Body.String())
			}

			if tc.expectedCause != "" {
				body := assertEventExposureJSON(t, w)
				if cause, ok := body["cause"].(string); !ok || cause != tc.expectedCause {
					t.Errorf("expected cause=%s, got %v", tc.expectedCause, body["cause"])
				}
			}
		})
	}
}

func TestEventExposure_ModifySubscription_SuccessCases(t *testing.T) {
	testCases := []struct {
		name             string
		setupTest        func() (subscriptionID string, mock *mockEventExposureAmf)
		validateResponse func(t *testing.T, response map[string]interface{})
	}{
		{
			name: "modify subscription expiry",
			setupTest: func() (string, *mockEventExposureAmf) {
				mock := newMockEventExposureAmf()

				supi := "imsi-208930000000001"
				ue := createEventExposureTestUE(mock.ctx, supi)

				subscriptionID := "sub-001"
				expiry := time.Now().Add(1 * time.Hour)
				contextSub := &amf_context.AMFContextEventSubscription{
					EventSubscription: models.AmfEventSubscription{
						EventList: []models.AmfEvent{
							{Type: models.AmfEventType_LOCATION_REPORT},
						},
						EventNotifyUri: "http://callback.example.com",
						Supi:           supi,
					},
					UeSupiList: []string{supi},
					Expiry:     &expiry,
				}
				mock.ctx.NewEventSubscription(subscriptionID, contextSub)

				ue.EventSubscriptionsInfo[subscriptionID] = &amf_context.AmfUeEventSubscription{
					EventSubscription: &models.ExtAmfEventSubscription{
						EventList:      contextSub.EventSubscription.EventList,
						EventNotifyUri: contextSub.EventSubscription.EventNotifyUri,
						Supi:           supi,
					},
					Timestamp: time.Now().UTC(),
				}

				return subscriptionID, mock
			},
			validateResponse: func(t *testing.T, response map[string]interface{}) {
				if subscription, ok := response["subscription"].(map[string]interface{}); !ok || subscription == nil {
					t.Error("expected subscription in response")
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			subscriptionID, mock := tc.setupTest()
			s := &Server{ServerAmf: mock}
			router := setupTestEventExposureRouter(s)

			newExpiry := time.Now().Add(2 * time.Hour).Format(time.RFC3339)
			jsonBody := `{"optionItem":[{"op":"replace","path":"/expiry","value":"` + newExpiry + `"}]}`
			req := httptest.NewRequest(http.MethodPatch, "/subscriptions/"+subscriptionID,
				bytes.NewBufferString(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d\nBody: %s", w.Code, w.Body.String())
			}

			var response map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatalf("failed to parse response body: %v", err)
			}

			if tc.validateResponse != nil {
				tc.validateResponse(t, response)
			}

			ctx := amf_context.GetSelf()
			ctx.DeleteEventSubscription(subscriptionID)
		})
	}
}

func TestEventExposure_CreateSubscription_ErrorCases(t *testing.T) {
	testCases := []struct {
		name           string
		setupServer    func() *Server
		requestBody    io.Reader
		expectedStatus int
		expectedCause  string
	}{
		{
			name: "request body read error",
			setupServer: func() *Server {
				return &Server{}
			},
			requestBody:    badReaderEE{},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "invalid JSON format",
			setupServer: func() *Server {
				return &Server{}
			},
			requestBody:    bytes.NewBufferString("{bad json"),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "empty subscription",
			setupServer: func() *Server {
				return &Server{ServerAmf: newMockEventExposureAmf()}
			},
			requestBody:    bytes.NewBufferString(`{}`),
			expectedStatus: http.StatusBadRequest,
			expectedCause:  "SUBSCRIPTION_EMPTY",
		},
		{
			name: "UE not found",
			setupServer: func() *Server {
				return &Server{ServerAmf: newMockEventExposureAmf()}
			},
			requestBody:    bytes.NewBufferString(`{"subscription":{"eventList":[{"type":"LOCATION_REPORT"}],"eventNotifyUri":"http://callback.example.com","supi":"imsi-999999999999999"}}`),
			expectedStatus: http.StatusForbidden,
			expectedCause:  "UE_NOT_SERVED_BY_AMF",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := tc.setupServer()
			router := setupTestEventExposureRouter(s)
			req := httptest.NewRequest(http.MethodPost, "/subscriptions", tc.requestBody)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Fatalf("expected status %d, got %d\nBody: %s",
					tc.expectedStatus, w.Code, w.Body.String())
			}

			if tc.expectedCause != "" {
				body := assertEventExposureJSON(t, w)
				if cause, ok := body["cause"].(string); !ok || cause != tc.expectedCause {
					t.Errorf("expected cause=%s, got %v", tc.expectedCause, body["cause"])
				}
			}
		})
	}
}

func TestEventExposure_CreateSubscription_SuccessCases(t *testing.T) {
	testCases := []struct {
		name             string
		setupTest        func() (supi string, mock *mockEventExposureAmf)
		validateResponse func(t *testing.T, supi string, response map[string]interface{})
	}{
		{
			name: "create subscription with UE",
			setupTest: func() (string, *mockEventExposureAmf) {
				mock := newMockEventExposureAmf()
				supi := "imsi-208930000000002"
				createEventExposureTestUE(mock.ctx, supi)
				return supi, mock
			},
			validateResponse: func(t *testing.T, supi string, response map[string]interface{}) {
				subscriptionID, ok := response["subscriptionId"].(string)
				if !ok || subscriptionID == "" {
					t.Error("expected subscriptionId in response")
				}

				ctx := amf_context.GetSelf()
				if sub, exists := ctx.FindEventSubscription(subscriptionID); !exists {
					t.Error("subscription should be created in context")
				} else {
					if len(sub.UeSupiList) != 1 || sub.UeSupiList[0] != supi {
						t.Errorf("expected UE %s in subscription, got %v", supi, sub.UeSupiList)
					}
				}

				if subscriptionID != "" {
					ctx.DeleteEventSubscription(subscriptionID)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			supi, mock := tc.setupTest()
			s := &Server{ServerAmf: mock}
			router := setupTestEventExposureRouter(s)

			jsonBody := `{"subscription":{"eventList":[{"type":"LOCATION_REPORT","immediateFlag":true}],"eventNotifyUri":"http://callback.example.com","supi":"` + supi + `"}}`
			req := httptest.NewRequest(http.MethodPost, "/subscriptions",
				bytes.NewBufferString(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != http.StatusCreated {
				t.Fatalf("expected status 201, got %d\nBody: %s", w.Code, w.Body.String())
			}

			var response map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatalf("failed to parse response body: %v", err)
			}

			if tc.validateResponse != nil {
				tc.validateResponse(t, supi, response)
			}
		})
	}
}

func TestEventExposure_DeleteSubscription_ErrorCases(t *testing.T) {
	testCases := []struct {
		name           string
		setupServer    func() *Server
		subscriptionID string
		expectedStatus int
		expectedCause  string
	}{
		{
			name: "subscription not found",
			setupServer: func() *Server {
				return &Server{ServerAmf: newMockEventExposureAmf()}
			},
			subscriptionID: "non-existent",
			expectedStatus: http.StatusNotFound,
			expectedCause:  "SUBSCRIPTION_NOT_FOUND",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := tc.setupServer()
			router := setupTestEventExposureRouter(s)
			req := httptest.NewRequest(http.MethodDelete, "/subscriptions/"+tc.subscriptionID, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Fatalf("expected status %d, got %d\nBody: %s",
					tc.expectedStatus, w.Code, w.Body.String())
			}

			if tc.expectedCause != "" {
				body := assertEventExposureJSON(t, w)
				if cause, ok := body["cause"].(string); !ok || cause != tc.expectedCause {
					t.Errorf("expected cause=%s, got %v", tc.expectedCause, body["cause"])
				}
			}
		})
	}
}

func TestEventExposure_DeleteSubscription_SuccessCases(t *testing.T) {
	testCases := []struct {
		name             string
		setupTest        func() (subscriptionID string, mock *mockEventExposureAmf)
		validateResponse func(t *testing.T, subscriptionID string)
	}{
		{
			name: "delete existing subscription",
			setupTest: func() (string, *mockEventExposureAmf) {
				mock := newMockEventExposureAmf()

				supi := "imsi-208930000000003"
				ue := createEventExposureTestUE(mock.ctx, supi)

				subscriptionID := "sub-002"
				contextSub := &amf_context.AMFContextEventSubscription{
					EventSubscription: models.AmfEventSubscription{
						EventList: []models.AmfEvent{
							{Type: models.AmfEventType_LOCATION_REPORT},
						},
						EventNotifyUri: "http://callback.example.com",
						Supi:           supi,
					},
					UeSupiList: []string{supi},
				}
				mock.ctx.NewEventSubscription(subscriptionID, contextSub)

				ue.EventSubscriptionsInfo[subscriptionID] = &amf_context.AmfUeEventSubscription{
					EventSubscription: &models.ExtAmfEventSubscription{
						EventList:      contextSub.EventSubscription.EventList,
						EventNotifyUri: contextSub.EventSubscription.EventNotifyUri,
						Supi:           supi,
					},
					Timestamp: time.Now().UTC(),
				}

				return subscriptionID, mock
			},
			validateResponse: func(t *testing.T, subscriptionID string) {
				ctx := amf_context.GetSelf()
				if _, exists := ctx.FindEventSubscription(subscriptionID); exists {
					t.Error("subscription should be deleted from context")
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			subscriptionID, mock := tc.setupTest()
			s := &Server{ServerAmf: mock}
			router := setupTestEventExposureRouter(s)

			req := httptest.NewRequest(http.MethodDelete, "/subscriptions/"+subscriptionID, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d\nBody: %s", w.Code, w.Body.String())
			}

			if tc.validateResponse != nil {
				tc.validateResponse(t, subscriptionID)
			}
		})
	}
}
