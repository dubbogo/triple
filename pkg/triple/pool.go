package triple

import (
	"fmt"
	"github.com/pkg/errors"
	"log"
	"sync"
	"sync/atomic"
	"time"
)
import (
	"github.com/dubbogo/grpc-go/connectivity"
	"github.com/go-co-op/gocron"
)

// ConnPool manages a pool of TripleConn instances
type ConnPool struct {
	pool []*TripleConn
	mu   sync.Mutex

	currentSize  int32
	closeChannel chan struct{}
	opt          Options
	healthCheck  *gocron.Scheduler
	address      string
	maxPoolSize  int32
	resizeCount  int32
}

// NewConnPool initializes a connection pool
func NewConnPool(address string, option Options) (*ConnPool, error) {
	if address == "" {
		return nil, errors.New("invalid address settings")
	}
	if option.Dial == nil {
		return nil, errors.New("invalid dial settings")
	}
	if option.MaxIdle <= 0 || option.MaxActive <= 0 || option.MaxIdle > option.MaxActive {
		return nil, errors.New("invalid maximum settings")
	}

	pool := &ConnPool{
		mu:           sync.Mutex{},
		currentSize:  0,
		closeChannel: make(chan struct{}),
		opt:          option,
		healthCheck:  gocron.NewScheduler(time.UTC),
		address:      address,
		pool:         make([]*TripleConn, 0, option.MaxIdle),
		maxPoolSize:  int32(option.MaxIdle),
		resizeCount:  0,
	}

	for i := 0; i < option.MaxIdle; i++ {
		c, err := option.Dial(address)
		if err != nil {
			pool.Close()
			return nil, fmt.Errorf("dial is not able to fill the pool: %s", err)
		}

		pool.pool = append(pool.pool, &TripleConn{
			grpcConn:     c,
			timeout:      DialTimeout,
			LastUsedTime: time.Now(),
		})
	}

	log.Printf("new pool success: %v\n", pool.Status())

	go pool.startHealthCheck()
	go pool.cleanUp()

	return pool, nil
}

// GetConnection Get a connection from the connection pool
func (cp *ConnPool) Get() (*TripleConn, error) {
	select {
	case <-cp.closeChannel:
		return nil, fmt.Errorf("connection pool is closed")
	default:
	}

	cp.mu.Lock()
	defer cp.mu.Unlock()

	if len(cp.pool) > 0 {
		conn := cp.pool[len(cp.pool)-1]
		cp.pool = cp.pool[:len(cp.pool)-1]
		atomic.AddInt32(&cp.currentSize, 1)
		return conn, nil
	}

	if atomic.LoadInt32(&cp.currentSize) < int32(cp.opt.MaxActive) {
		cp.DynamicResize()
	}

	if len(cp.pool) > 0 {
		conn := cp.pool[len(cp.pool)-1]
		cp.pool = cp.pool[:len(cp.pool)-1]
		atomic.AddInt32(&cp.currentSize, 1)
		return conn, nil
	}

	if atomic.LoadInt32(&cp.currentSize) < int32(cp.maxPoolSize) {
		conn, err := cp.opt.Dial(cp.address)
		if err != nil {
			return nil, fmt.Errorf("unable to create new connection: %w", err)
		}

		newConn := &TripleConn{
			timeout:      DialTimeout,
			grpcConn:     conn,
			LastUsedTime: time.Now(),
		}
		atomic.AddInt32(&cp.currentSize, 1)
		return newConn, nil
	}

	return nil, fmt.Errorf("unable to acquire connection, pool is at max size")
}

func (cp *ConnPool) Put(conn *TripleConn) {
	select {
	case <-cp.closeChannel:
		conn.grpcConn.Close()
		return
	default:
	}

	cp.mu.Lock()
	defer cp.mu.Unlock()

	if int32(len(cp.pool)) < atomic.LoadInt32(&cp.maxPoolSize) && cp.isHealthy(conn) {
		conn.LastUsedTime = time.Now()
		cp.pool = append(cp.pool, conn)
		atomic.AddInt32(&cp.currentSize, -1)
	} else {
		conn.grpcConn.Close()
	}
}

// isHealthy checks if the connection is healthy based on its state and last used time.
func (cp *ConnPool) isHealthy(conn *TripleConn) bool {
	if conn.grpcConn.GetState() == connectivity.Shutdown {
		return false
	}

	if time.Since(conn.LastUsedTime) > cp.opt.IdleTimeout {
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

			for i := len(cp.pool) - 1; i >= 0; i-- {
				conn := cp.pool[i]
				if !cp.isHealthy(conn) || time.Since(conn.LastUsedTime) > cp.opt.IdleTimeout {
					conn.grpcConn.Close()
					cp.pool = append(cp.pool[:i], cp.pool[i+1:]...)
					atomic.AddInt32(&cp.currentSize, -1)
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

// cleanupExpiredConnections Check and clean up expired connections
func (cp *ConnPool) cleanupExpiredConnections() {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	now := time.Now()

	for i := len(cp.pool) - 1; i >= 0; i-- {
		conn := cp.pool[i]
		if now.Sub(conn.LastUsedTime) > cp.opt.IdleTimeout {
			conn.grpcConn.Close()
			cp.pool = append(cp.pool[:i], cp.pool[i+1:]...)
			atomic.AddInt32(&cp.currentSize, -1)
		}
	}

	cp.ShrinkPool()

	log.Printf("cleanupExpiredConnections: current pool size: %d\n", len(cp.pool))
}

// DynamicResize performs dynamic resizing of the connection pool
func (cp *ConnPool) DynamicResize() {
	if atomic.LoadInt32(&cp.currentSize) >= int32(cp.opt.MaxActive) {
		return
	}

	increase := int32(1 << atomic.LoadInt32(&cp.resizeCount))
	if increase > int32(cp.opt.MaxIdle) {
		increase = int32(cp.opt.MaxIdle)
	}

	newMaxPoolSize := atomic.AddInt32(&cp.maxPoolSize, increase)

	newMaxIdle := int32(float64(newMaxPoolSize) * 0.8)
	if newMaxIdle > int32(cp.opt.MaxActive) {
		newMaxIdle = int32(cp.opt.MaxActive)
	}

	cp.opt.MaxIdle = int(newMaxIdle)
	if newMaxPoolSize > int32(cp.opt.MaxActive) {
		atomic.StoreInt32(&cp.maxPoolSize, int32(cp.opt.MaxActive))
	}

	for i := int32(0); i < increase && atomic.LoadInt32(&cp.currentSize) < atomic.LoadInt32(&cp.maxPoolSize); i++ {
		conn, err := cp.opt.Dial(cp.address)
		if err != nil {
			continue
		}

		newConn := &TripleConn{
			timeout:      DialTimeout,
			grpcConn:     conn,
			LastUsedTime: time.Now(),
		}
		cp.pool = append(cp.pool, newConn)
	}

	atomic.AddInt32(&cp.resizeCount, 1)
	log.Printf("Resized connection pool, new max pool size: %d", atomic.LoadInt32(&cp.maxPoolSize))
}

// ShrinkPool performs shrinking of the connection pool by reducing idle connections
func (cp *ConnPool) ShrinkPool() {
	idealSize := int32(float64(cp.opt.MaxActive) * 0.8)

	if atomic.LoadInt32(&cp.currentSize) < idealSize && len(cp.pool) > int(idealSize) {
		decrease := int32(1 << atomic.LoadInt32(&cp.resizeCount))

		if decrease > int32(cp.opt.MaxIdle) {
			decrease = int32(cp.opt.MaxIdle)
		}

		newMaxPoolSize := atomic.LoadInt32(&cp.maxPoolSize) - decrease
		if newMaxPoolSize < int32(cp.opt.MaxIdle) {
			newMaxPoolSize = int32(cp.opt.MaxIdle)
		}

		atomic.StoreInt32(&cp.maxPoolSize, newMaxPoolSize)

		newMaxIdle := int32(float64(newMaxPoolSize) * 0.8)
		if newMaxIdle < int32(cp.opt.MaxIdle) {
			newMaxIdle = int32(cp.opt.MaxIdle)
		}
		cp.opt.MaxIdle = int(newMaxIdle)

		cp.deleteFrom(newMaxPoolSize)

		log.Printf("Shrunk connection pool, new max pool size: %d", atomic.LoadInt32(&cp.maxPoolSize))
	}
}

// deleteFrom will delete excess connections from the pool
func (cp *ConnPool) deleteFrom(maxSize int32) {
	for i := int32(len(cp.pool)) - 1; i >= maxSize; i-- {
		conn := cp.pool[i]
		if conn != nil {
			if closeErr := conn.grpcConn.Close(); closeErr != nil {
				fmt.Errorf("error closing connection: %v", closeErr)
				return
			}
			cp.pool = cp.pool[:i]
		}
	}
}

// Close shuts down the connection pool and closes all connections
func (cp *ConnPool) Close() error {
	if cp.closeChannel == nil {
		return nil
	}

	close(cp.closeChannel)

	cp.mu.Lock()
	defer cp.mu.Unlock()

	var err error
	for _, conn := range cp.pool {
		if closeErr := conn.grpcConn.Close(); closeErr != nil {
			err = fmt.Errorf("error closing connection: %v", closeErr)
		}
	}

	cp.pool = nil
	atomic.StoreInt32(&cp.currentSize, 0)

	if err != nil {
		return err
	}

	return nil
}

func (p *ConnPool) incrPoolSize() int32 {
	newSize := atomic.AddInt32(&p.currentSize, 1)
	return newSize
}

func (p *ConnPool) decrPoolSize() int32 {
	newSize := atomic.AddInt32(&p.currentSize, -1)
	return newSize
}

func (p *ConnPool) setMaxPoolSize(size int32) {
	atomic.StoreInt32(&p.maxPoolSize, size)
}

func (p *ConnPool) getPoolSize() int32 {
	return atomic.LoadInt32(&p.currentSize)
}

func (p *ConnPool) getMaxPoolSize() int32 {
	return atomic.LoadInt32(&p.maxPoolSize)
}

func (p *ConnPool) Status() string {
	return fmt.Sprintf("address:%s, current:%d, maxPoolSize:%d, poolSize:%d, resizeCount:%d. option:%v",
		p.address, p.currentSize, p.maxPoolSize, len(p.pool), p.resizeCount, p.opt)
}
