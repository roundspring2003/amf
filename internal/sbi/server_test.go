package sbi

import (
	"io"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	amf_context "github.com/free5gc/amf/internal/context"
	"github.com/free5gc/amf/internal/logger"
	"github.com/free5gc/amf/internal/sbi/consumer"
	"github.com/free5gc/amf/internal/sbi/processor"
	"github.com/free5gc/amf/pkg/app"
	"github.com/free5gc/amf/pkg/factory"
)

// ==========================================
// 1. Logger Initialization
// ==========================================
func init() {
	// 1. Mute Free5GC internal loggers
	silentLogger := logrus.New()
	silentLogger.SetOutput(io.Discard)
	silentLogger.SetLevel(logrus.PanicLevel)

	mute := func(l *logrus.Entry, name string) *logrus.Entry {
		if l == nil {
			return silentLogger.WithField("component", name)
		}
		l.Logger.SetOutput(io.Discard)
		l.Logger.SetLevel(logrus.PanicLevel)
		return l
	}

	logger.MtLog = mute(logger.MtLog, "test_mt")
	logger.SBILog = mute(logger.SBILog, "test_sbi")
	logger.CallbackLog = mute(logger.CallbackLog, "test_callback")
	logger.ProducerLog = mute(logger.ProducerLog, "test_producer")
	logger.GmmLog = mute(logger.GmmLog, "test_gmm")
	logger.InitLog = mute(logger.InitLog, "test_init")
	logger.CfgLog = mute(logger.CfgLog, "test_cfg")
	logger.NgapLog = mute(logger.NgapLog, "test_ngap")

	// Prevent logger.Fatal from exiting the program
	if logger.CallbackLog.Logger != nil {
		logger.CallbackLog.Logger.ExitFunc = func(int) {}
	}

	// 2. Mute Gin Framework logs
	gin.SetMode(gin.TestMode)
	gin.DefaultWriter = io.Discard
	gin.DefaultErrorWriter = io.Discard
}

// ==========================================
// 2. Mock Definition
// ==========================================

// MockAmf serves as a dependency for both Server and Processor.
type MockAmf struct {
	app.App // Embeds interface to avoid implementing unused methods

	fakeConfig  *factory.Config
	fakeContext *amf_context.AMFContext

	// Stores the real Processor instance to resolve circular dependency
	proc *processor.Processor
}

func (m *MockAmf) Config() *factory.Config {
	return m.fakeConfig
}

func (m *MockAmf) Context() *amf_context.AMFContext {
	return m.fakeContext
}

func (m *MockAmf) Consumer() *consumer.Consumer {
	return nil
}

func (m *MockAmf) Processor() *processor.Processor {
	// Returns the stored instance instead of creating a new one
	return m.proc
}

// ==========================================
// 3. Test Helper
// ==========================================

// NewTestServer assembles MockAmf, Processor, and Server for testing.
func NewTestServer(t *testing.T) (*Server, *amf_context.AMFContext) {
	// 1. Prepare base configuration
	cfg := &factory.Config{
		Configuration: &factory.Configuration{
			Sbi: &factory.Sbi{
				BindingIPv4: "127.0.0.1",
				Port:        8000,
			},
			// Define required services for route registration
			ServiceNameList: []string{
				"namf-mt",
				"namf-oam",
				"namf-comm",
			},
		},
	}

	// Initialize global config (required by server.go's newRouter)
	factory.AmfConfig = cfg

	ctx := &amf_context.AMFContext{}

	// 2. Create MockAmf "Shell"
	mockAmf := &MockAmf{
		fakeConfig:  cfg,
		fakeContext: ctx,
	}

	// 3. Initialize real Processor "Brain"
	realProc, err := processor.NewProcessor(mockAmf)
	if err != nil {
		t.Fatalf("Failed to create real processor: %v", err)
	}

	// 4. Inject Processor back into MockAmf
	mockAmf.proc = realProc

	// 5. Start Server
	s, err := NewServer(mockAmf, "")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	return s, ctx
}