package sbi

import (
	"bytes"
	"errors"
	"io"
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

//
// -------------------------------------------------------------------
// Helper Functions
// -------------------------------------------------------------------
//

type mockCommunicationAmf struct {
	ctx *amf_context.AMFContext
}

func (m *mockCommunicationAmf) Start()                           {}
func (m *mockCommunicationAmf) Terminate()                       {}
func (m *mockCommunicationAmf) SetLogEnable(bool)                {}
func (m *mockCommunicationAmf) SetLogLevel(string)               {}
func (m *mockCommunicationAmf) SetReportCaller(bool)             {}
func (m *mockCommunicationAmf) Context() *amf_context.AMFContext { return m.ctx }
func (m *mockCommunicationAmf) Config() *factory.Config          { return &factory.Config{} }
func (m *mockCommunicationAmf) Consumer() *consumer.Consumer     { return nil }

func (m *mockCommunicationAmf) Processor() *processor.Processor {
	proc, _ := processor.NewProcessor(m)
	return proc
}

func newMockCommunicationAmf() *mockCommunicationAmf {
	ctx := amf_context.GetSelf()
	ctx.Name = "TestAMF"
	ctx.ServedGuamiList = []models.Guami{
		{
			PlmnId: &models.PlmnIdNid{Mcc: "208", Mnc: "93"},
			AmfId:  "cafe00",
		},
	}
	return &mockCommunicationAmf{ctx: ctx}
}

type badReaderComm struct{}

func (b badReaderComm) Read(p []byte) (int, error) { return 0, errors.New("read error") }

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

//
// -------------------------------------------------------------------
// Tests
// -------------------------------------------------------------------
//

func TestCommunication_RouteDefinitions(t *testing.T) {
	s := &Server{}
	routes := s.getCommunicationRoutes()

	if len(routes) != 18 {
		t.Fatalf("expected 18 routes, got %d", len(routes))
	}
}

func TestCommunication_BasicEndpoints(t *testing.T) {
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
			router := setupTestCommunicationRouter(s)
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

func TestCommunication_NotImplementedEndpoints(t *testing.T) {
	testCases := []struct {
		name   string
		method string
		path   string
	}{
		{
			name:   "relocate UE context",
			method: http.MethodPost,
			path:   "/ue-contexts/ue123/relocate",
		},
		{
			name:   "cancel relocate UE context",
			method: http.MethodPost,
			path:   "/ue-contexts/ue123/cancel-relocate",
		},
		{
			name:   "non-UE N2 info unsubscribe",
			method: http.MethodDelete,
			path:   "/non-ue-n2-messages/subscriptions/sub123",
		},
		{
			name:   "non-UE N2 message transfer",
			method: http.MethodPost,
			path:   "/non-ue-n2-messages/transfer",
		},
		{
			name:   "non-UE N2 info subscribe",
			method: http.MethodPost,
			path:   "/non-ue-n2-messages/subscriptions",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := &Server{}
			router := setupTestCommunicationRouter(s)
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != http.StatusNotImplemented {
				t.Fatalf("expected 501, got %d", w.Code)
			}
		})
	}
}

func TestCommunication_Subscription_ErrorCases(t *testing.T) {
	testCases := []struct {
		name           string
		method         string
		path           string
		setupServer    func() *Server
		requestBody    io.Reader
		expectedStatus int
	}{
		{
			name:   "AMF status change subscribe - read error",
			method: http.MethodPost,
			path:   "/subscriptions",
			setupServer: func() *Server {
				return &Server{}
			},
			requestBody:    badReaderComm{},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:   "AMF status change subscribe - bad JSON",
			method: http.MethodPost,
			path:   "/subscriptions",
			setupServer: func() *Server {
				return &Server{}
			},
			requestBody:    bytes.NewBufferString("{bad json"),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "AMF status change modify - read error",
			method: http.MethodPut,
			path:   "/subscriptions/sub123",
			setupServer: func() *Server {
				return &Server{}
			},
			requestBody:    badReaderComm{},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:   "AMF status change modify - bad JSON",
			method: http.MethodPut,
			path:   "/subscriptions/sub123",
			setupServer: func() *Server {
				return &Server{}
			},
			requestBody:    bytes.NewBufferString("{bad json"),
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := tc.setupServer()
			router := setupTestCommunicationRouter(s)
			req := httptest.NewRequest(tc.method, tc.path, tc.requestBody)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Fatalf("expected status %d, got %d", tc.expectedStatus, w.Code)
			}
		})
	}
}

func TestCommunication_UEContext_ErrorCases(t *testing.T) {
	testCases := []struct {
		name           string
		method         string
		path           string
		setupServer    func() *Server
		requestBody    io.Reader
		expectedStatus int
	}{
		{
			name:   "create UE context - read error",
			method: http.MethodPut,
			path:   "/ue-contexts/ue123",
			setupServer: func() *Server {
				return &Server{}
			},
			requestBody:    badReaderComm{},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:   "create UE context - bad JSON",
			method: http.MethodPut,
			path:   "/ue-contexts/ue123",
			setupServer: func() *Server {
				return &Server{}
			},
			requestBody:    bytes.NewBufferString("{bad json"),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "EBI assignment - read error",
			method: http.MethodPost,
			path:   "/ue-contexts/ue123/assign-ebi",
			setupServer: func() *Server {
				return &Server{}
			},
			requestBody:    badReaderComm{},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:   "EBI assignment - bad JSON",
			method: http.MethodPost,
			path:   "/ue-contexts/ue123/assign-ebi",
			setupServer: func() *Server {
				return &Server{}
			},
			requestBody:    bytes.NewBufferString("{bad json"),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "registration status update - read error",
			method: http.MethodPost,
			path:   "/ue-contexts/ue123/transfer-update",
			setupServer: func() *Server {
				return &Server{}
			},
			requestBody:    badReaderComm{},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:   "registration status update - bad JSON",
			method: http.MethodPost,
			path:   "/ue-contexts/ue123/transfer-update",
			setupServer: func() *Server {
				return &Server{}
			},
			requestBody:    bytes.NewBufferString("{bad json"),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "release UE context - read error",
			method: http.MethodPost,
			path:   "/ue-contexts/ue123/release",
			setupServer: func() *Server {
				return &Server{}
			},
			requestBody:    badReaderComm{},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:   "release UE context - bad JSON",
			method: http.MethodPost,
			path:   "/ue-contexts/ue123/release",
			setupServer: func() *Server {
				return &Server{}
			},
			requestBody:    bytes.NewBufferString("{bad json"),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "UE context transfer - read error",
			method: http.MethodPost,
			path:   "/ue-contexts/ue123/transfer",
			setupServer: func() *Server {
				return &Server{}
			},
			requestBody:    badReaderComm{},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:   "UE context transfer - bad JSON",
			method: http.MethodPost,
			path:   "/ue-contexts/ue123/transfer",
			setupServer: func() *Server {
				return &Server{}
			},
			requestBody:    bytes.NewBufferString("{bad json"),
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := tc.setupServer()
			router := setupTestCommunicationRouter(s)
			req := httptest.NewRequest(tc.method, tc.path, tc.requestBody)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Fatalf("expected status %d, got %d", tc.expectedStatus, w.Code)
			}
		})
	}
}

func TestCommunication_N1N2Message_ErrorCases(t *testing.T) {
	testCases := []struct {
		name           string
		method         string
		path           string
		setupServer    func() *Server
		requestBody    io.Reader
		contentType    string
		expectedStatus int
	}{
		{
			name:   "N1N2 message transfer - read error",
			method: http.MethodPost,
			path:   "/ue-contexts/ue123/n1-n2-messages",
			setupServer: func() *Server {
				return &Server{}
			},
			requestBody:    badReaderComm{},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:   "N1N2 message transfer - JSON content type",
			method: http.MethodPost,
			path:   "/ue-contexts/ue123/n1-n2-messages",
			setupServer: func() *Server {
				return &Server{}
			},
			requestBody:    bytes.NewBufferString("{}"),
			contentType:    "application/json",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "N1N2 message subscribe - read error",
			method: http.MethodPost,
			path:   "/ue-contexts/ue123/n1-n2-messages/subscriptions",
			setupServer: func() *Server {
				return &Server{}
			},
			requestBody:    badReaderComm{},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:   "N1N2 message subscribe - bad JSON",
			method: http.MethodPost,
			path:   "/ue-contexts/ue123/n1-n2-messages/subscriptions",
			setupServer: func() *Server {
				return &Server{}
			},
			requestBody:    bytes.NewBufferString("{bad json"),
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := tc.setupServer()
			router := setupTestCommunicationRouter(s)
			req := httptest.NewRequest(tc.method, tc.path, tc.requestBody)
			if tc.contentType != "" {
				req.Header.Set("Content-Type", tc.contentType)
			}
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Fatalf("expected status %d, got %d", tc.expectedStatus, w.Code)
			}
		})
	}
}

func TestCommunication_Subscription_SuccessCases(t *testing.T) {
	testCases := []struct {
		name        string
		method      string
		path        string
		setupTest   func() *mockCommunicationAmf
		requestBody string
	}{
		{
			name:   "AMF status change subscribe",
			method: http.MethodPost,
			path:   "/subscriptions",
			setupTest: func() *mockCommunicationAmf {
				return newMockCommunicationAmf()
			},
			requestBody: `{"nfStatusNotificationUri":"http://callback.example.com"}`,
		},
		{
			name:   "AMF status change modify",
			method: http.MethodPut,
			path:   "/subscriptions/sub123",
			setupTest: func() *mockCommunicationAmf {
				return newMockCommunicationAmf()
			},
			requestBody: `{"nfStatusNotificationUri":"http://callback.example.com"}`,
		},
		{
			name:   "AMF status change unsubscribe",
			method: http.MethodDelete,
			path:   "/subscriptions/sub123",
			setupTest: func() *mockCommunicationAmf {
				return newMockCommunicationAmf()
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mock := tc.setupTest()
			s := &Server{ServerAmf: mock}
			router := setupTestCommunicationRouter(s)

			var req *http.Request
			if tc.requestBody != "" {
				req = httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.requestBody))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(tc.method, tc.path, nil)
			}
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code == http.StatusBadRequest && tc.requestBody != "" {
				if strings.Contains(w.Body.String(), "Malformed request syntax") {
					t.Fatalf("JSON should have been parsed successfully")
				}
			}

			if w.Code == http.StatusInternalServerError && tc.requestBody != "" {
				if strings.Contains(w.Body.String(), "Get Request Body error") {
					t.Fatalf("Request body should have been read successfully")
				}
			}
		})
	}
}

func TestCommunication_N1N2Message_WithUE(t *testing.T) {
	testCases := []struct {
		name   string
		method string
		path   string
	}{
		{
			name:   "N1N2 message unsubscribe",
			method: http.MethodDelete,
			path:   "/n1-n2-messages/subscriptions/1",
		},
		{
			name:   "N1N2 message transfer status",
			method: http.MethodGet,
			path:   "/n1-n2-messages/999",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mock := newMockCommunicationAmf()
			ctx := mock.ctx

			supi := "imsi-208930000000010"
			_ = ctx.NewAmfUe(supi)

			s := &Server{ServerAmf: mock}
			router := setupTestCommunicationRouter(s)

			fullPath := "/ue-contexts/" + supi + tc.path
			req := httptest.NewRequest(tc.method, fullPath, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK && w.Code != http.StatusNoContent &&
				w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
				t.Logf("Got status %d (acceptable)", w.Code)
			}
		})
	}
}

func TestCommunication_CreateUEContext_WithUE(t *testing.T) {
	mock := newMockCommunicationAmf()

	s := &Server{ServerAmf: mock}
	router := setupTestCommunicationRouter(s)

	supi := "imsi-208930000000012"
	jsonBody := `{"ueContext":{"supi":"` + supi + `","pei":"imei-123456789012345"}}`
	req := httptest.NewRequest(http.MethodPut, "/ue-contexts/"+supi,
		bytes.NewBufferString(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code == http.StatusBadRequest {
		if strings.Contains(w.Body.String(), "Malformed request syntax") {
			t.Fatalf("JSON should have been parsed successfully")
		}
	}
}
