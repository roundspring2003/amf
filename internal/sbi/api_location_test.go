package sbi

import (
	"bytes"
	"encoding/json"
	"errors"
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

type mockLocationAmf struct{}

func (m *mockLocationAmf) Start()               {}
func (m *mockLocationAmf) Terminate()           {}
func (m *mockLocationAmf) SetLogEnable(bool)    {}
func (m *mockLocationAmf) SetLogLevel(string)   {}
func (m *mockLocationAmf) SetReportCaller(bool) {}

func (m *mockLocationAmf) Context() *amf_context.AMFContext {
	return nil
}

func (m *mockLocationAmf) Config() *factory.Config {
	return nil
}

func (m *mockLocationAmf) Consumer() *consumer.Consumer {
	return nil
}

func (m *mockLocationAmf) Processor() *processor.Processor {
	proc, _ := processor.NewProcessor(m)
	return proc
}

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

//
// -------------------------------------------------------------------
// Tests
// -------------------------------------------------------------------
//

func TestLocation_RouteDefinitions(t *testing.T) {
	s := &Server{}
	routes := s.getLocationRoutes()

	if len(routes) != 4 {
		t.Fatalf("expected 4 routes, got %d", len(routes))
	}
}

func TestLocation_HelloWorld(t *testing.T) {
	s := &Server{}
	router := setupTestLocationRouter(s)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// --- Body read error → 500
type badReader struct{}

func (b badReader) Read(p []byte) (int, error) { return 0, errors.New("read error") }

func TestLocation_ProvideLocationInfo_ReadError(t *testing.T) {
	s := &Server{}
	router := setupTestLocationRouter(s)

	req := httptest.NewRequest(http.MethodPost, "/ue/provide-loc-info", badReader{})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// --- Bad JSON → 400
func TestLocation_ProvideLocationInfo_BadJSON(t *testing.T) {
	s := &Server{}
	router := setupTestLocationRouter(s)

	req := httptest.NewRequest("POST", "/ue/provide-loc-info",
		bytes.NewBufferString("{bad json"))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- SUCCESS CASE — Processor called
func TestLocation_ProvideLocationInfo_Success(t *testing.T) {
	amfContext := amf_context.GetSelf()
	amfContext.Name = "TestAMF"

	mockAmf := &mockLocationAmf{}

	s := &Server{
		ServerAmf: mockAmf,
	}

	router := setupTestLocationRouter(s)

	jsonBody := `{"supportedGADShapes":["POINT"]}`
	req := httptest.NewRequest("POST", "/ue123/provide-loc-info",
		bytes.NewBufferString(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 (UE not found), got %d\nBody: %s", w.Code, w.Body.String())
	}

	var problemDetail map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &problemDetail); err != nil {
		t.Fatalf("failed to parse response body: %v", err)
	}

	expectedCause := "CONTEXT_NOT_FOUND"
	if cause, ok := problemDetail["cause"].(string); !ok || cause != expectedCause {
		t.Errorf("expected cause=%s, got %v", expectedCause, problemDetail["cause"])
	}
}

func TestLocation_ProvideLocationInfo_WithUE(t *testing.T) {
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

	supi := "imsi-208930000000001"
	ue := amfContext.NewAmfUe(supi)
	anType := models.AccessType__3_GPP_ACCESS

	ue.RanUe = make(map[models.AccessType]*amf_context.RanUe)
	ue.RanUe[anType] = &amf_context.RanUe{
		SupportedFeatures: "0123456789abcdef",
	}

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

	ue.RatType = models.RatType_NR
	ue.TimeZone = "+08:00"
	mockAmf := &mockLocationAmf{}

	s := &Server{
		ServerAmf: mockAmf,
	}

	router := setupTestLocationRouter(s)

	jsonBody := `{"req5gsLoc":true,"reqCurrentLoc":true,"reqRatType":true,"reqTimeZone":true}`
	req := httptest.NewRequest("POST", "/"+supi+"/provide-loc-info",
		bytes.NewBufferString(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d\nBody: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response body: %v", err)
	}

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
}

// --- 501 handlers
func TestLocation_ProvidePosInfo_501(t *testing.T) {
	s := &Server{}
	router := setupTestLocationRouter(s)

	req := httptest.NewRequest("POST", "/ue/provide-pos-info", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d", w.Code)
	}
}

func TestLocation_CancelLocation_501(t *testing.T) {
	s := &Server{}
	router := setupTestLocationRouter(s)

	req := httptest.NewRequest("POST", "/ue/cancel-loc-info", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d", w.Code)
	}
}
