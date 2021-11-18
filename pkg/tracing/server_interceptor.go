package tracing

import (
	"context"
	"strings"
)

import (
	"github.com/dubbogo/grpc-go"
	"github.com/dubbogo/grpc-go/codes"
	"github.com/dubbogo/grpc-go/metadata"
	"github.com/dubbogo/grpc-go/status"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/opentracing/opentracing-go/log"
)

// OpenTracingServerInterceptor returns a grpc.UnaryServerInterceptor suitable
// for use in a grpc.NewServer call.
//
// For example:
//
//     s := grpc.NewServer(
//         ...,  // (existing ServerOptions)
//         grpc.UnaryInterceptor(otgrpc.OpenTracingServerInterceptor(tracer)))
//
// All gRPC server spans will look for an OpenTracing SpanContext in the gRPC
// metadata; if found, the server span will act as the ChildOf that RPC
// SpanContext.
//
// Root or not, the server Span will be embedded in the context.Context for the
// application-specific gRPC handler(s) to access.
func OpenTracingServerInterceptor(tracer opentracing.Tracer, optFuncs ...Option) grpc.UnaryServerInterceptor {
	otgrpcOpts := newOptions()
	otgrpcOpts.apply(optFuncs...)
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp interface{}, err error) {
		spanContext, err := extractSpanContext(ctx, tracer)
		//if err != nil && err != opentracing.ErrSpanContextNotFound {
		// TODO: establish some sort of error reporting mechanism here. We
		// don't know where to put such an error and must rely on Tracer
		// implementations to do something appropriate for the time being.
		//}
		if otgrpcOpts.inclusionFunc != nil &&
			!otgrpcOpts.inclusionFunc(spanContext, info.FullMethod, req, nil) {
			return handler(ctx, req)
		}
		serverSpan := tracer.StartSpan(
			info.FullMethod,
			ext.RPCServerOption(spanContext),
			gRPCComponentTag,
		)
		defer serverSpan.Finish()

		ctx = opentracing.ContextWithSpan(ctx, serverSpan)
		if otgrpcOpts.logPayloads {
			serverSpan.LogFields(log.Object("gRPC request", req))
		}
		resp, err = handler(ctx, req)
		if err == nil {
			if otgrpcOpts.logPayloads {
				serverSpan.LogFields(log.Object("gRPC response", resp))
			}
		} else {
			SetSpanTags(serverSpan, err, false)
			serverSpan.LogFields(log.String("event", "error"), log.String("message", err.Error()))
		}
		if otgrpcOpts.decorator != nil {
			otgrpcOpts.decorator(ctx, serverSpan, info.FullMethod, req, resp, err)
		}
		return resp, err
	}
}

// OpenTracingStreamServerInterceptor returns a grpc.StreamServerInterceptor suitable
// for use in a grpc.NewServer call. The interceptor instruments streaming RPCs by
// creating a single span to correspond to the lifetime of the RPC's stream.
//
// For example:
//
//     s := grpc.NewServer(
//         ...,  // (existing ServerOptions)
//         grpc.StreamInterceptor(otgrpc.OpenTracingStreamServerInterceptor(tracer)))
//
// All gRPC server spans will look for an OpenTracing SpanContext in the gRPC
// metadata; if found, the server span will act as the ChildOf that RPC
// SpanContext.
//
// Root or not, the server Span will be embedded in the context.Context for the
// application-specific gRPC handler(s) to access.
func OpenTracingStreamServerInterceptor(tracer opentracing.Tracer, optFuncs ...Option) grpc.StreamServerInterceptor {
	otgrpcOpts := newOptions()
	otgrpcOpts.apply(optFuncs...)
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		spanContext, _ := extractSpanContext(ss.Context(), tracer)
		//if e != nil && e != opentracing.ErrSpanContextNotFound {
		// TODO: establish some sort of error reporting mechanism here. We
		// don't know where to put such an error and must rely on Tracer
		// implementations to do something appropriate for the time being.
		//}
		if otgrpcOpts.inclusionFunc != nil &&
			!otgrpcOpts.inclusionFunc(spanContext, info.FullMethod, nil, nil) {
			return handler(srv, ss)
		}

		serverSpan := tracer.StartSpan(
			info.FullMethod,
			ext.RPCServerOption(spanContext),
			gRPCComponentTag,
		)
		defer serverSpan.Finish()
		ss = &openTracingServerStream{
			ServerStream: ss,
			ctx:          opentracing.ContextWithSpan(ss.Context(), serverSpan),
		}
		e := handler(srv, ss)
		if e != nil {
			SetSpanTags(serverSpan, e, false)
			serverSpan.LogFields(log.String("event", "error"), log.String("message", e.Error()))
		}
		if otgrpcOpts.decorator != nil {
			otgrpcOpts.decorator(ss.Context(), serverSpan, info.FullMethod, nil, nil, e)
		}
		return e
	}
}

type openTracingServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (ss *openTracingServerStream) Context() context.Context {
	return ss.ctx
}

func extractSpanContext(ctx context.Context, tracer opentracing.Tracer) (opentracing.SpanContext, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	}
	return tracer.Extract(opentracing.HTTPHeaders, metadataReaderWriter{md})
}

// Option instances may be used in OpenTracing(Server|Client)Interceptor
// initialization.
//
// See this post about the "functional options" pattern:
// http://dave.cheney.net/2014/10/17/functional-options-for-friendly-apis
type Option func(o *options)

// LogPayloads returns an Option that tells the OpenTracing instrumentation to
// try to log application payloads in both directions.
func LogPayloads() Option {
	return func(o *options) {
		o.logPayloads = true
	}
}

// SpanInclusionFunc provides an optional mechanism to decide whether or not
// to trace a given gRPC call. Return true to create a Span and initiate
// tracing, false to not create a Span and not trace.
//
// parentSpanCtx may be nil if no parent could be extraction from either the Go
// context.Context (on the client) or the RPC (on the server).
type SpanInclusionFunc func(
	parentSpanCtx opentracing.SpanContext,
	method string,
	req, resp interface{}) bool

// IncludingSpans binds a IncludeSpanFunc to the options
func IncludingSpans(inclusionFunc SpanInclusionFunc) Option {
	return func(o *options) {
		o.inclusionFunc = inclusionFunc
	}
}

// SpanDecoratorFunc provides an (optional) mechanism for otgrpc users to add
// arbitrary tags/logs/etc to the opentracing.Span associated with client
// and/or server RPCs.
type SpanDecoratorFunc func(
	ctx context.Context,
	span opentracing.Span,
	method string,
	req, resp interface{},
	grpcError error)

// SpanDecorator binds a function that decorates gRPC Spans.
func SpanDecorator(decorator SpanDecoratorFunc) Option {
	return func(o *options) {
		o.decorator = decorator
	}
}

// The internal-only options struct. Obviously overkill at the moment; but will
// scale well as production use dictates other configuration and tuning
// parameters.
type options struct {
	logPayloads bool
	decorator   SpanDecoratorFunc
	// May be nil.
	inclusionFunc SpanInclusionFunc
}

// newOptions returns the default options.
func newOptions() *options {
	return &options{
		logPayloads:   false,
		inclusionFunc: nil,
	}
}

func (o *options) apply(opts ...Option) {
	for _, opt := range opts {
		opt(o)
	}
}

var (
	// Morally a const:
	gRPCComponentTag = opentracing.Tag{Key: string(ext.Component), Value: "gRPC"}
)

// metadataReaderWriter satisfies both the opentracing.TextMapReader and
// opentracing.TextMapWriter interfaces.
type metadataReaderWriter struct {
	metadata.MD
}

func (w metadataReaderWriter) Set(key, val string) {
	// The GRPC HPACK implementation rejects any uppercase keys here.
	//
	// As such, since the HTTP_HEADERS format is case-insensitive anyway, we
	// blindly lowercase the key (which is guaranteed to work in the
	// Inject/Extract sense per the OpenTracing spec).
	key = strings.ToLower(key)
	w.MD[key] = append(w.MD[key], val)
}

func (w metadataReaderWriter) ForeachKey(handler func(key, val string) error) error {
	for k, vals := range w.MD {
		for _, v := range vals {
			if err := handler(k, v); err != nil {
				return err
			}
		}
	}

	return nil
}

// A Class is a set of types of outcomes (including errors) that will often
// be handled in the same way.
type Class string

const (
	Unknown Class = "0xx"
	// Success represents outcomes that achieved the desired results.
	Success Class = "2xx"
	// ClientError represents errors that were the client's fault.
	ClientError Class = "4xx"
	// ServerError represents errors that were the server's fault.
	ServerError Class = "5xx"
)

// ErrorClass returns the class of the given error
func ErrorClass(err error) Class {
	if s, ok := status.FromError(err); ok {
		switch s.Code() {
		// Success or "success"
		case codes.OK, codes.Canceled:
			return Success

		// Client errors
		case codes.InvalidArgument, codes.NotFound, codes.AlreadyExists,
			codes.PermissionDenied, codes.Unauthenticated, codes.FailedPrecondition,
			codes.OutOfRange:
			return ClientError

		// Server errors
		case codes.DeadlineExceeded, codes.ResourceExhausted, codes.Aborted,
			codes.Unimplemented, codes.Internal, codes.Unavailable, codes.DataLoss:
			return ServerError

		// Not sure
		case codes.Unknown:
			fallthrough
		default:
			return Unknown
		}
	}
	return Unknown
}

// SetSpanTags sets one or more tags on the given span according to the
// error.
func SetSpanTags(span opentracing.Span, err error, client bool) {
	c := ErrorClass(err)
	code := codes.Unknown
	if s, ok := status.FromError(err); ok {
		code = s.Code()
	}
	span.SetTag("response_code", code)
	span.SetTag("response_class", c)
	if err == nil {
		return
	}
	if c != Success && (client || c == ServerError) {
		ext.Error.Set(span, true)
	}
}
