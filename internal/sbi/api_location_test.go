package sbi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	amf_context "github.com/free5gc/amf/internal/context"
	"github.com/free5gc/amf/internal/sbi/consumer"
	"github.com/free5gc/amf/internal/sbi/processor"
	"github.com/free5gc/amf/pkg/factory"
	"github.com/free5gc/openapi/models"
	"github.com/gin-gonic/gin"
)

// Mock and helper functions
type mockLocationAmf struct {
	ctx *amf_context.AMFContext
}

func (m *mockLocationAmf) Start()                           {}
func (m *mockLocationAmf) Terminate()                       {}
func (m *mockLocationAmf) SetLogEnable(bool)                {}
func (m *mockLocationAmf) SetLogLevel(string)               {}
func (m *mockLocationAmf) SetReportCaller(bool)             {}
func (m *mockLocationAmf) Context() *amf_context.AMFContext { return m.ctx }
func (m *mockLocationAmf) Config() *factory.Config          { return nil }
func (m *mockLocationAmf) Consumer() *consumer.Consumer     { return nil }

func (m *mockLocationAmf) Processor() *processor.Processor {
	proc, _ := processor.NewProcessor(m)
	return proc
}

type badReader struct{}

func (b badReader) Read(p []byte) (int, error) { return 0, errors.New("read error") }

func setupTestLocationRouter(s *Server) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	routes := s.getLocationRoutes()

	for _, route := range routes {
		switch route.Method {
		case http.MethodGet:
			r.GET(route.Pattern, route.APIFunc)
		case http.MethodPost:
			r.POST(route.Pattern, route.APIFunc)
		default:
			panic("unsupported method")
		}
	}
	return r
}

func newMockLocationAmf() *mockLocationAmf {
	ctx := amf_context.GetSelf()
	ctx.Name = "TestAMF"
	ctx.ServedGuamiList = []models.Guami{
		{
			PlmnId: &models.PlmnIdNid{Mcc: "208", Mnc: "93"},
			AmfId:  "cafe00",
		},
	}
	return &mockLocationAmf{ctx: ctx}
}

func createTestUE(ctx *amf_context.AMFContext, supi string) *amf_context.AmfUe {
	ue := ctx.NewAmfUe(supi)
	anType := models.AccessType__3_GPP_ACCESS

	ue.RanUe = make(map[models.AccessType]*amf_context.RanUe)
	ue.RanUe[anType] = &amf_context.RanUe{
		SupportedFeatures: "0123456789abcdef",
	}

	ue.Location = models.UserLocation{
		NrLocation: &models.NrLocation{
			Tai: &models.Tai{
				PlmnId: &models.PlmnId{Mcc: "208", Mnc: "93"},
				Tac:    "000001",
			},
		},
	}
	ue.RatType = models.RatType_NR
	ue.TimeZone = "+08:00"

	return ue
}

//
// -------------------------------------------------------------------
// Tests - 3A Style
// -------------------------------------------------------------------
//

func TestLocation_RouteDefinitions(t *testing.T) {
	s := &Server{}
	routes := s.getLocationRoutes()

	if len(routes) != 4 {
		t.Fatalf("expected 4 routes, got %d", len(routes))
	}
}

func TestLocation_BasicEndpoints(t *testing.T) {
	testCases := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
	}{
		{
			name:           "health check endpoint",
			method:         http.MethodGet,
			path:           "/",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "provide positioning info - not implemented",
			method:         http.MethodPost,
			path:           "/ue123/provide-pos-info",
			expectedStatus: http.StatusNotImplemented,
		},
		{
			name:           "cancel location - not implemented",
			method:         http.MethodPost,
			path:           "/ue123/cancel-loc-info",
			expectedStatus: http.StatusNotImplemented,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := &Server{}
			router := setupTestLocationRouter(s)
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Fatalf("expected %d, got %d", tc.expectedStatus, w.Code)
			}
		})
	}
}

func TestLocation_ProvideLocationInfo_ErrorCases(t *testing.T) {
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
			requestBody:    badReader{},
			expectedStatus: http.StatusInternalServerError,
			expectedCause:  "",
		},
		{
			name: "invalid JSON format",
			setupServer: func() *Server {
				return &Server{}
			},
			requestBody:    bytes.NewBufferString("{bad json"),
			expectedStatus: http.StatusBadRequest,
			expectedCause:  "",
		},
		{
			name: "UE context not found",
			setupServer: func() *Server {
				return &Server{ServerAmf: newMockLocationAmf()}
			},
			requestBody:    bytes.NewBufferString(`{"supportedGADShapes":["POINT"]}`),
			expectedStatus: http.StatusNotFound,
			expectedCause:  "CONTEXT_NOT_FOUND",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := tc.setupServer()
			router := setupTestLocationRouter(s)
			req := httptest.NewRequest(http.MethodPost, "/ue123/provide-loc-info", tc.requestBody)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Fatalf("expected status %d, got %d\nBody: %s",
					tc.expectedStatus, w.Code, w.Body.String())
			}

			// Additional assertion for specific error causes
			if tc.expectedCause != "" {
				var problemDetail map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &problemDetail); err != nil {
					t.Fatalf("failed to parse response body: %v", err)
				}

				if cause, ok := problemDetail["cause"].(string); !ok || cause != tc.expectedCause {
					t.Errorf("expected cause=%s, got %v", tc.expectedCause, problemDetail["cause"])
				}
			}
		})
	}
}

func TestLocation_ProvideLocationInfo_SuccessCases(t *testing.T) {
	testCases := []struct {
		name             string
		setupTest        func() (supi string, mock *mockLocationAmf)
		requestBody      string
		expectedStatus   int
		validateResponse func(t *testing.T, response map[string]interface{})
	}{
		{
			name: "successfully retrieve UE location",
			setupTest: func() (string, *mockLocationAmf) {
				mock := newMockLocationAmf()
				supi := "imsi-208930000000001"
				createTestUE(mock.ctx, supi)
				return supi, mock
			},
			requestBody:    `{"req5gsLoc":true,"reqCurrentLoc":true,"reqRatType":true,"reqTimeZone":true}`,
			expectedStatus: http.StatusOK,
			validateResponse: func(t *testing.T, response map[string]interface{}) {
				if currentLoc, ok := response["currentLoc"].(bool); !ok || !currentLoc {
					t.Errorf("expected currentLoc=true, got %v", response["currentLoc"])
				}

				if ratType, ok := response["ratType"].(string); !ok || ratType != string(models.RatType_NR) {
					t.Errorf("expected ratType=NR, got %v", response["ratType"])
				}

				if timezone, ok := response["timezone"].(string); !ok || timezone != "+08:00" {
					t.Errorf("expected timezone=+08:00, got %v", response["timezone"])
				}

				if location := response["location"]; location == nil {
					t.Error("expected location to be present")
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			supi, mock := tc.setupTest()
			s := &Server{ServerAmf: mock}
			router := setupTestLocationRouter(s)
			req := httptest.NewRequest(http.MethodPost, "/"+supi+"/provide-loc-info",
				bytes.NewBufferString(tc.requestBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Fatalf("expected status %d, got %d\nBody: %s",
					tc.expectedStatus, w.Code, w.Body.String())
			}

			var response map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatalf("failed to parse response body: %v", err)
			}

			if tc.validateResponse != nil {
				tc.validateResponse(t, response)
			}
		})
	}
}
