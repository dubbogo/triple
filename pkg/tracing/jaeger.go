package tracing

import (
	"github.com/opentracing/opentracing-go"

	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/transport"
)

import (
	"github.com/dubbogo/triple/pkg/common/logger"
)

type jaegerLogger struct {
	logger.Logger
}

func (j *jaegerLogger) Error(msg string) {
	j.Logger.Error(msg)
}

func NewJaegerTracerDirect(service, httpEndpoint string, logger logger.Logger) opentracing.Tracer {
	sender := transport.NewHTTPTransport(
		httpEndpoint,
	)
	tracer, _ := jaeger.NewTracer(service,
		jaeger.NewConstSampler(true),
		jaeger.NewRemoteReporter(sender, jaeger.ReporterOptions.Logger(&jaegerLogger{
			Logger: logger,
		})),
	)
	return tracer
}

func NewJaegerTracerAgent(service, agentAddress string, logger logger.Logger) opentracing.Tracer {
	sender, _ := jaeger.NewUDPTransport(agentAddress, 0)
	tracer, _ := jaeger.NewTracer(service,
		jaeger.NewConstSampler(true),
		jaeger.NewRemoteReporter(sender, jaeger.ReporterOptions.Logger(&jaegerLogger{
			Logger: logger,
		})),
	)
	return tracer
}
