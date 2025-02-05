package triple

import (
	"context"
	"time"
)

import (
	"github.com/dubbogo/grpc-go"
	"github.com/dubbogo/grpc-go/backoff"
	"github.com/dubbogo/grpc-go/keepalive"
)

const (
	// DialTimeout the timeout of create connection
	DialTimeout = 5 * time.Second

	// BackoffMaxDelay provided maximum delay when backing off after failed connection attempts.
	BackoffMaxDelay = 3 * time.Second

	// KeepAliveTime is the duration of time after which if the client doesn't see
	// any activity it pings the server to see if the transport is still alive.
	KeepAliveTime = time.Duration(10) * time.Second

	// KeepAliveTimeout is the duration of time for which the client waits after having
	// pinged for keepalive check and if no activity is seen even after that the connection
	// is closed.
	KeepAliveTimeout = time.Duration(3) * time.Second

	// InitialWindowSize we set it 1GB is to provide system's throughput.
	InitialWindowSize = 1 << 30

	// InitialConnWindowSize we set it 1GB is to provide system's throughput.
	InitialConnWindowSize = 1 << 30

	// MaxSendMsgSize set max gRPC request message size sent to server.
	MaxSendMsgSize = 4 << 30

	// MaxRecvMsgSize set max gRPC receive message size received from server.
	MaxRecvMsgSize = 4 << 30
)

// Options are params for creating grpc connect pool.
type Options struct {
	// Dial is an application supplied function for creating and configuring a connection.
	Dial func(address string, opts ...grpc.DialOption) (*grpc.ClientConn, error)

	// Maximum number of idle connections in the pool.
	MaxIdle int

	// Maximum number of connections allocated by the pool at a given time.
	// When zero, there is no limit on the number of connections in the pool.
	MaxActive int

	// IdleTimeout is the maximum duration for which a connection can be idle before being closed.
	IdleTimeout time.Duration
}

var DefaultOptions = Options{
	Dial:        Dial,
	MaxIdle:     8,
	MaxActive:   64,
	IdleTimeout: 5 * time.Minute,
}

func Dial(address string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), DialTimeout)
	defer cancel()

	defaultOptions := []grpc.DialOption{
		grpc.WithInsecure(),
		grpc.WithInitialWindowSize(InitialWindowSize),
		grpc.WithInitialConnWindowSize(InitialConnWindowSize),
		grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(MaxSendMsgSize)),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(MaxRecvMsgSize)),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                KeepAliveTime,
			Timeout:             KeepAliveTimeout,
			PermitWithoutStream: true,
		}),
		grpc.WithConnectParams(grpc.ConnectParams{
			Backoff: backoff.Config{
				MaxDelay: BackoffMaxDelay,
			},
		}),
	}

	allOptions := append(defaultOptions, opts...)

	return grpc.DialContext(ctx, address, allOptions...)
}
