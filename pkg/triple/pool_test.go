package triple

import (
	"testing"
	"time"
)

import (
	"github.com/dubbogo/grpc-go"
	"github.com/stretchr/testify/assert"
)

// TestConnPool_NewConnPool tests the initialization of the connection pool
func TestConnPool_NewConnPool(t *testing.T) {
	dialer := func(address string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
		if len(opts) == 0 {
			opts = append(opts, grpc.WithInsecure()) // 这里加上 WithInsecure 选项
		}
		conn, err := grpc.Dial(address, opts...)
		if err != nil {
			return nil, err
		}
		return conn, nil
	}

	pool := NewConnPool(2, "localhost:50051", dialer)

	assert.Equal(t, 2, len(pool.pool), "expected 2 connections in pool")

	assert.Equal(t, 2, pool.maxPoolSize, "expected maxPoolSize 2")

	assert.Equal(t, 2, pool.currentSize, "expected currentSize 2")

	assert.NotNil(t, pool.pool, "expected pool to be initialized")

	assert.NotNil(t, pool.closeChannel, "expected closeChannel to be initialized")

}

// TestConnPool_GetConnection tests getting a connection from the pool
func TestConnPool_GetConnection(t *testing.T) {
	dialer := func(address string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
		if len(opts) == 0 {
			opts = append(opts, grpc.WithInsecure()) // 这里加上 WithInsecure 选项
		}
		conn, err := grpc.Dial(address, opts...)
		if err != nil {
			return nil, err
		}
		return conn, nil
	}

	pool := NewConnPool(2, "localhost:50051", dialer)

	conn, err := pool.GetConnection()
	assert.NoError(t, err, "expected no error when getting a connection")

	assert.NotNil(t, conn, "expected a non-nil connection")
}

// TestConnPool_isHealthy tests the isHealthy method of the pool
func TestConnPool_isHealthy(t *testing.T) {
	dialer := func(address string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
		if len(opts) == 0 {
			opts = append(opts, grpc.WithInsecure()) // 这里加上 WithInsecure 选项
		}
		conn, err := grpc.Dial(address, opts...)
		if err != nil {
			return nil, err
		}
		return conn, nil
	}

	pool := NewConnPool(2, "localhost:50051", dialer)

	conn, _ := pool.GetConnection()

	assert.True(t, pool.isHealthy(conn), "expected the connection to be healthy")
}

// TestConnPool_cleanUp tests the cleanUp function for cleaning idle or unhealthy connections
func TestConnPool_cleanUp(t *testing.T) {
	dialer := func(address string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
		if len(opts) == 0 {
			opts = append(opts, grpc.WithInsecure()) // 这里加上 WithInsecure 选项
		}
		conn, err := grpc.Dial(address, opts...)
		if err != nil {
			return nil, err
		}
		return conn, nil
	}

	pool := NewConnPool(2, "localhost:50051", dialer)

	conn, _ := pool.GetConnection()
	conn.lastUsed = time.Now().Add(-10 * time.Minute)

	go pool.cleanUp()

	time.Sleep(2 * time.Second)

	assert.Len(t, pool.pool, 1, "expected 1 connection after cleanup")
}

// TestConnPool_DynamicResize tests the DynamicResize method
func TestConnPool_DynamicResize(t *testing.T) {
	dialer := func(address string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
		if len(opts) == 0 {
			opts = append(opts, grpc.WithInsecure()) // 这里加上 WithInsecure 选项
		}
		conn, err := grpc.Dial(address, opts...)
		if err != nil {
			return nil, err
		}
		return conn, nil
	}

	pool := NewConnPool(2, "localhost:50051", dialer)

	initialSize := pool.maxPoolSize

	pool.DynamicResize()

	assert.Equal(t, initialSize+1, pool.maxPoolSize, "expected pool size to increase by 1")
}

// TestConnPool_Close tests the Close method
func TestConnPool_Close(t *testing.T) {
	dialer := func(address string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
		if len(opts) == 0 {
			opts = append(opts, grpc.WithInsecure()) // 这里加上 WithInsecure 选项
		}
		conn, err := grpc.Dial(address, opts...)
		if err != nil {
			return nil, err
		}
		return conn, nil
	}

	pool := NewConnPool(2, "localhost:50051", dialer)

	err := pool.Close()
	assert.NoError(t, err, "expected no error when closing the pool")

	assert.Empty(t, pool.pool, "expected no connections in pool after close")
	assert.Equal(t, 0, pool.currentSize, "expected current size to be 0 after close")
}
