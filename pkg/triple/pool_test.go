package triple

import (
	"context"
	"fmt"
	"github.com/dubbogo/grpc-go"
	"github.com/stretchr/testify/assert"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func DialTest(address string, options ...grpc.DialOption) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), DialTimeout)
	defer cancel()
	return grpc.DialContext(ctx, address, grpc.WithInsecure())
}

func TestConnPoolInitialization(t *testing.T) {
	options := Options{
		// 设定为一个有效的 Dial 函数
		Dial:        DialTest,
		MaxIdle:     5,
		MaxActive:   10,
		IdleTimeout: 5 * time.Second,
	}

	pool, err := NewConnPool("localhost:8080", options)
	assert.NoError(t, err)
	assert.NotNil(t, pool)
	assert.Equal(t, int32(0), pool.currentSize)
	assert.Equal(t, int32(5), pool.maxPoolSize)

	assert.NotNil(t, pool.healthCheck)

	err = pool.Close()
	if err != nil {
		t.Fatalf("Failed to close connection pool: %v", err)
	}
}

func TestGetConnection(t *testing.T) {
	options := Options{
		Dial:        DialTest,
		MaxIdle:     2,
		MaxActive:   5,
		IdleTimeout: 2 * time.Second,
	}

	pool, err := NewConnPool("localhost:8080", options)
	assert.NoError(t, err)

	conn, err := pool.Get()
	assert.NoError(t, err)
	assert.NotNil(t, conn)

	conn2, err := pool.Get()
	assert.NoError(t, err)
	assert.NotNil(t, conn2)

	conn3, err := pool.Get()
	assert.NoError(t, err)
	assert.NotNil(t, conn3)

	fmt.Printf("pool,status：%v\n\n", pool.Status())

	err = pool.Close()
	if err != nil {
		t.Fatalf("Failed to close connection pool: %v", err)
	}
}

func TestPutConnection(t *testing.T) {
	options := Options{
		Dial:        DialTest,
		MaxIdle:     2,
		MaxActive:   5,
		IdleTimeout: 2 * time.Second,
	}

	pool, err := NewConnPool("localhost:8080", options)
	assert.NoError(t, err)

	conn, err := pool.Get()
	assert.NoError(t, err)
	pool.Put(conn)

	conn2, err := pool.Get()
	assert.NoError(t, err)
	pool.Put(conn2)

	conn3, err := pool.Get()
	assert.NoError(t, err)
	pool.Put(conn3)

	assert.Len(t, pool.pool, 2)

	err = pool.Close()
	if err != nil {
		t.Fatalf("Failed to close connection pool: %v", err)
	}
}

func TestHealthCheck(t *testing.T) {
	options := Options{
		Dial:        DialTest,
		MaxIdle:     2,
		MaxActive:   5,
		IdleTimeout: 2 * time.Second,
	}

	pool, err := NewConnPool("localhost:8080", options)
	assert.NoError(t, err)

	time.Sleep(3 * time.Second)

	assert.Len(t, pool.pool, 0)

	err = pool.Close()
	if err != nil {
		t.Fatalf("Failed to close connection pool: %v", err)
	}
}

func TestDynamicResize(t *testing.T) {
	options := Options{
		Dial:        DialTest,
		MaxIdle:     2,
		MaxActive:   10,
		IdleTimeout: 2 * time.Second,
	}

	pool, err := NewConnPool("localhost:8080", options)
	assert.NoError(t, err)

	assert.Equal(t, int32(2), pool.maxPoolSize)

	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			// 获取连接
			conn, err := pool.Get()
			assert.NoError(t, err)

			time.Sleep(100 * time.Millisecond)

			pool.Put(conn)

			if i >= 2 {
				assert.Greater(t, pool.maxPoolSize, int32(2))
			}
		}(i)
	}

	wg.Wait()
	assert.Equal(t, int32(10), pool.maxPoolSize)

	err = pool.Close()
	if err != nil {
		t.Fatalf("Failed to close connection pool: %v", err)
	}
}

func TestClosePool(t *testing.T) {
	options := Options{
		Dial:        DialTest,
		MaxIdle:     2,
		MaxActive:   5,
		IdleTimeout: 2 * time.Second,
	}

	pool, err := NewConnPool("localhost:8080", options)
	assert.NoError(t, err)

	conn, err := pool.Get()
	assert.NoError(t, err)
	pool.Put(conn)

	err = pool.Close()
	assert.NoError(t, err)

	conn2, err := pool.Get()
	assert.Error(t, err)
	assert.Nil(t, conn2)
}

func TestConnPoolShrinkPool(t *testing.T) {
	options := Options{
		Dial:        DialTest,
		MaxIdle:     5,
		MaxActive:   10,
		IdleTimeout: 5 * time.Second,
	}

	pool, err := NewConnPool("localhost:8080", options)
	assert.NoError(t, err)
	assert.NotNil(t, pool)

	// mock
	atomic.StoreInt32(&pool.currentSize, 6)
	pool.pool = make([]*TripleConn, 12)
	atomic.StoreInt32(&pool.maxPoolSize, 12)

	pool.ShrinkPool()

	assert.Equal(t, int32(11), atomic.LoadInt32(&pool.maxPoolSize))
	assert.Equal(t, 8, pool.opt.MaxIdle)

}
