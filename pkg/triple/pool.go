package triple

import (
	"fmt"
	"sync"
	"time"
)
import (
	"github.com/dubbogo/grpc-go"
	"github.com/go-co-op/gocron"
	"google.golang.org/grpc/connectivity"
)

// ConnPool manages a pool of TripleConn instances
type ConnPool struct {
	pool              []*TripleConn
	selector          *RoundRobinSelector
	mu                sync.Mutex
	maxPoolSize       int
	currentSize       int
	closeChannel      chan struct{}
	idleTimeout       time.Duration
	connectionTimeout time.Duration
	dialer            func(address string, opts ...grpc.DialOption) (*grpc.ClientConn, error)
	healthCheck       *gocron.Scheduler
	address           string
}

// RoundRobinSelector helps to round-robin select a connection from the pool
type RoundRobinSelector struct {
	i  int
	mu sync.Mutex
}

func (rr *RoundRobinSelector) Select(max int) int {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	i := rr.i
	rr.i = (rr.i + 1) % max

	return i
}

// NewConnPool initializes a connection pool
func NewConnPool(size int, address string, dialer func(address string, opts ...grpc.DialOption) (*grpc.ClientConn, error), opts ...grpc.DialOption) *ConnPool {
	pool := &ConnPool{
		maxPoolSize:       size,
		currentSize:       0,
		closeChannel:      make(chan struct{}),
		selector:          &RoundRobinSelector{},
		idleTimeout:       5 * time.Second, // 更短的 idleTimeout 方便调试
		connectionTimeout: 30 * time.Second,
		dialer:            dialer,
		address:           address,
		pool:              make([]*TripleConn, 0, size), // 初始化连接池切片
	}

	// 创建连接
	for i := 0; i < size; i++ {
		conn, err := dialer(address, opts...)
		fmt.Printf("conn:%v\n", conn)
		if err == nil && conn != nil {
			pool.pool = append(pool.pool, &TripleConn{
				grpcConn:  conn,
				createdAt: time.Now(),
				lastUsed:  time.Now(),
				timeout:   pool.idleTimeout,
			})
			pool.currentSize++
		} else {
			fmt.Printf("Failed to create connection: %v\n", err)
			// 可以考虑返回错误，或者继续初始化更多连接
		}
	}

	fmt.Printf("Pool initialized with %d connections\n", pool.currentSize)

	// 启动健康检查和清理协程
	go pool.startHealthCheck()
	go pool.cleanUp()

	return pool
}

// GetConnection 从连接池中获取一个连接
func (cp *ConnPool) GetConnection() (*TripleConn, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	idx := cp.selector.Select(len(cp.pool))
	conn := cp.pool[idx]

	if !cp.isHealthy(conn) {
		// 获取下一个健康的连接
		if healthyConn, err := cp.getNextHealthyConn(idx, len(cp.pool)); err == nil {
			return healthyConn, nil
		}
	}

	return conn, nil
}

func (cp *ConnPool) getNextHealthyConn(startIdx, poolSize int) (*TripleConn, error) {
	for i := 1; i < poolSize; i++ {
		idx := (startIdx + i) % poolSize
		conn := cp.pool[idx]
		if cp.isHealthy(conn) {
			return conn, nil
		}
	}
	return nil, fmt.Errorf("No healthy connections available")
}

//// ReturnConnection returns a connection to the pool
//func (cp *ConnPool) ReturnConnection(conn *TripleConn) {
//	cp.mu.Lock()
//	defer cp.mu.Unlock()
//
//	conn.lastUsed = time.Now()
//
//	if cp.isHealthy(conn) {
//		conn.SetStatus(constant.StatusIdle)
//		conn.timeout = cp.idleTimeout
//	} else {
//		conn.onceClose.Do(func() {
//			conn.grpcConn.Close()
//		})
//		cp.pool = removeConnection(cp.pool, conn)
//		cp.currentSize--
//	}
//}

// 从池中移除连接的辅助函数
func removeConnection(pool []*TripleConn, conn *TripleConn) []*TripleConn {
	for i, c := range pool {
		if c == conn {
			return append(pool[:i], pool[i+1:]...)
		}
	}
	return pool
}

// isHealthy checks whether a connection is healthy
func (cp *ConnPool) isHealthy(conn *TripleConn) bool {
	if conn.grpcConn == nil {
		return false
	}
	if time.Since(conn.lastUsed) > cp.idleTimeout {
		return false
	}

	state := conn.grpcConn.GetState()
	if connectivity.State(state) != connectivity.Ready && connectivity.State(state) != connectivity.Idle {
		return false
	}

	return true
}

// cleanUp periodically cleans up idle or unhealthy connections
func (cp *ConnPool) cleanUp() {
	ticker := time.NewTicker(100 * time.Millisecond)

	defer ticker.Stop()

	for {
		select {
		case <-cp.closeChannel:
			return
		case <-ticker.C:
			cp.mu.Lock()

			// Clean up connections
			for i := len(cp.pool) - 1; i >= 0; i-- {
				conn := cp.pool[i]
				if !cp.isHealthy(conn) || time.Since(conn.lastUsed) > cp.idleTimeout {
					conn.grpcConn.Close()
					cp.pool = append(cp.pool[:i], cp.pool[i+1:]...)
					cp.currentSize--
				}
			}

			cp.mu.Unlock()
		}
	}
}

// startHealthCheck performs periodic health checks
func (cp *ConnPool) startHealthCheck() {
	cp.healthCheck = gocron.NewScheduler(time.UTC)
	_, err := cp.healthCheck.Every(30).Seconds().Do(func() {
		cp.cleanupExpiredConnections()
	})
	if err != nil {
		fmt.Printf("Error starting health check: %s\n", err)
	}
	cp.healthCheck.StartAsync()
}

// cleanupExpiredConnections removes expired connections from the pool
func (cp *ConnPool) cleanupExpiredConnections() {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	for i := 0; i < len(cp.pool); i++ {
		conn := cp.pool[i]
		if time.Since(conn.createdAt) > conn.timeout {
			// Close and remove expired connection
			conn.grpcConn.Close()
			cp.pool = append(cp.pool[:i], cp.pool[i+1:]...)
			cp.currentSize--
			i-- // Adjust the index since an element was removed
		}
	}
}

// DynamicResize adjusts the size of the pool dynamically based on usage
func (cp *ConnPool) DynamicResize() {
	if cp.currentSize >= cp.maxPoolSize {
		cp.maxPoolSize++
	}
}

// Close shuts down the connection pool and closes all connections
func (cp *ConnPool) Close() error {
	close(cp.closeChannel)
	cp.mu.Lock()
	defer cp.mu.Unlock()

	for _, conn := range cp.pool {
		conn.grpcConn.Close()
	}

	cp.pool = nil
	cp.currentSize = 0
	return nil
}
