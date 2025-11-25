package sbi

import (
	"io"
	"github.com/sirupsen/logrus"

	amf_context "github.com/free5gc/amf/internal/context"
	"github.com/free5gc/amf/internal/logger"
	"github.com/free5gc/amf/internal/sbi/consumer"
	"github.com/free5gc/amf/internal/sbi/processor"
	"github.com/free5gc/amf/pkg/app"
	"github.com/free5gc/amf/pkg/factory"
)

// ==========================================
// 1. Common Initialization
// ==========================================

func init() {
	// Create a silent logger that discards all output
	silentLogger := logrus.New()
	silentLogger.SetOutput(io.Discard)
	silentLogger.SetLevel(logrus.PanicLevel)

	// Helper function to mute a logger entry
	mute := func(l *logrus.Entry, name string) *logrus.Entry {
		if l == nil {
			return silentLogger.WithField("component", name)
		}
		// If logger exists, redirect its output to discard
		l.Logger.SetOutput(io.Discard)
		l.Logger.SetLevel(logrus.PanicLevel)
		return l
	}

	// Mute all relevant loggers used in AMF
	// This prevents "[ERRO]...", "[WARN]...", "[INFO]..." logs from cluttering test output
	logger.MtLog = mute(logger.MtLog, "test_mt")
	logger.SBILog = mute(logger.SBILog, "test_sbi")
	logger.CallbackLog = mute(logger.CallbackLog, "test_callback")
	logger.ProducerLog = mute(logger.ProducerLog, "test_producer")
	logger.GmmLog = mute(logger.GmmLog, "test_gmm")
	logger.InitLog = mute(logger.InitLog, "test_init")
	logger.CfgLog = mute(logger.CfgLog, "test_cfg")
	logger.NgapLog = mute(logger.NgapLog, "test_ngap")
}

// ==========================================
// 2. Common Mock Definitions
// ==========================================

// MockProcessorAmf acts as the bottom layer dependency (Processor's dependency).
// It mocks the application context to prevent nil pointer panics during tests.
type MockProcessorAmf struct {
	app.App
	fakeContext *amf_context.AMFContext
}

func (m *MockProcessorAmf) Consumer() *consumer.Consumer {
	return &consumer.Consumer{}
}

func (m *MockProcessorAmf) Context() *amf_context.AMFContext {
	if m.fakeContext == nil {
		return &amf_context.AMFContext{}
	}
	return m.fakeContext
}

func (m *MockProcessorAmf) Config() *factory.Config {
	return &factory.Config{
		Configuration: &factory.Configuration{
			Sbi: &factory.Sbi{
				BindingIPv4: "127.0.0.1",
				Port:        8000,
			},
		},
	}
}

// MockServerAmf acts as the top layer dependency (Server's dependency).
// It injects the real processor (initialized with MockProcessorAmf) into the Server.
type MockServerAmf struct {
	app.App
	realProcessor *processor.Processor
}

func (m *MockServerAmf) Processor() *processor.Processor {
	return m.realProcessor
}

func (m *MockServerAmf) Consumer() *consumer.Consumer {
	return &consumer.Consumer{}
}

func (m *MockServerAmf) Config() *factory.Config {
	return &factory.Config{
		Configuration: &factory.Configuration{
			Sbi: &factory.Sbi{
				BindingIPv4: "127.0.0.1",
				Port:        8000,
			},
		},
	}
}