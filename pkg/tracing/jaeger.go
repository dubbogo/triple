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

func NewJaegerTracerDirect(service, endpoint string, logger logger.Logger) opentracing.Tracer {
	sender := transport.NewHTTPTransport(
		endpoint,
	)
	tracer, _ := jaeger.NewTracer(service,
		jaeger.NewConstSampler(true),
		jaeger.NewRemoteReporter(sender, jaeger.ReporterOptions.Logger(jaeger.StdLogger)),
	)
	return tracer
}

func NewJaegerTracerAgent(service, endpoint string) opentracing.Tracer {
	sender, _ := jaeger.NewUDPTransport(endpoint, 0)
	tracer, _ := jaeger.NewTracer(service,
		jaeger.NewConstSampler(true),
		jaeger.NewRemoteReporter(sender, jaeger.ReporterOptions.Logger(jaeger.StdLogger)),
	)
	return tracer
}
