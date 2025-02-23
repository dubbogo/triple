package triple

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

import (
	"github.com/dubbogo/grpc-go/connectivity"
	"github.com/pkg/errors"
)

import (
	"github.com/dubbogo/triple/pkg/common/constant"
)

// ConnPool manages a pool of TripleConn instances
type ConnPool struct {
	pool []*TripleConn
	mu   sync.RWMutex

	currentSize  int32
	closeChannel chan struct{}
	opt          Options
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
		mu:           sync.RWMutex{},
		currentSize:  0,
		closeChannel: make(chan struct{}),
		opt:          option,
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

	go pool.cleanUpAndHealthCheck()

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
		cp.DynamicResize(constant.ResizeTypeIncrease)
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
	state := conn.grpcConn.GetState()
	if state == connectivity.Shutdown || state == connectivity.TransientFailure {
		return false
	}

	if time.Since(conn.LastUsedTime) > cp.opt.IdleTimeout {
		return false
	}

	return true
}

// cleanUpAndHealthCheck performs both health checks and cleans up expired or unhealthy connections
func (cp *ConnPool) cleanUpAndHealthCheck() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-cp.closeChannel:
			return
		case <-ticker.C:
			cp.mu.Lock()

			now := time.Now()
			for i := len(cp.pool) - 1; i >= 0; i-- {
				conn := cp.pool[i]
				if !cp.isHealthy(conn) || now.Sub(conn.LastUsedTime) > cp.opt.IdleTimeout {
					conn.grpcConn.Close()
					cp.pool = append(cp.pool[:i], cp.pool[i+1:]...)
					atomic.AddInt32(&cp.currentSize, -1)
				}
			}

			cp.DynamicResize(constant.ResizeTypeDecrease)

			cp.mu.Unlock()
		}
	}
}

// DynamicResize performs dynamic resizing of the connection pool, either resizing up or down.
func (cp *ConnPool) DynamicResize(resizeType int) {
	currentSize := atomic.LoadInt32(&cp.currentSize)
	maxActive := int32(cp.opt.MaxActive)
	maxIdle := int32(cp.opt.MaxIdle)

	switch resizeType {
	case constant.ResizeTypeIncrease:
		if currentSize >= maxActive {
			return
		}

		increase := int32(1 << atomic.LoadInt32(&cp.resizeCount))
		if increase > maxIdle {
			increase = maxIdle
		}

		newMaxPoolSize := atomic.AddInt32(&cp.maxPoolSize, increase)

		newMaxIdle := int32(float64(newMaxPoolSize) * 0.8)
		if newMaxIdle > maxActive {
			newMaxIdle = maxActive
		}
		cp.opt.MaxIdle = int(newMaxIdle)

		if newMaxPoolSize > maxActive {
			atomic.StoreInt32(&cp.maxPoolSize, maxActive)
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

	case constant.ResizeTypeDecrease:
		idealSize := int32(float64(maxActive) * 0.8)

		if currentSize < idealSize && len(cp.pool) > int(idealSize) {
			decrease := int32(1 << atomic.LoadInt32(&cp.resizeCount))

			if decrease > maxIdle {
				decrease = maxIdle
			}

			newMaxPoolSize := atomic.LoadInt32(&cp.maxPoolSize) - decrease
			if newMaxPoolSize < maxIdle {
				newMaxPoolSize = maxIdle
			}

			atomic.StoreInt32(&cp.maxPoolSize, newMaxPoolSize)

			newMaxIdle := int32(float64(newMaxPoolSize) * 0.8)
			if newMaxIdle < maxIdle {
				newMaxIdle = maxIdle
			}
			cp.opt.MaxIdle = int(newMaxIdle)

			cp.deleteFrom(newMaxPoolSize)

			log.Printf("Shrunk connection pool, new max pool size: %d", atomic.LoadInt32(&cp.maxPoolSize))
		}

	default:
		log.Printf("Unknown resize type: %d", resizeType)
	}
}

// deleteFrom will delete excess connections from the pool
func (cp *ConnPool) deleteFrom(maxSize int32) {
	for i := int32(len(cp.pool)) - 1; i >= maxSize; i-- {
		conn := cp.pool[i]
		if conn != nil {
			if closeErr := conn.grpcConn.Close(); closeErr != nil {
				log.Printf("error closing connection: %v", closeErr)
				return
			}
			cp.pool = cp.pool[:i]
		}
	}
}

// Close shuts down the connection pool and closes all connections
func (cp *ConnPool) Close() error {
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

func (p *ConnPool) Status() string {
	return fmt.Sprintf("address:%s, current:%d, maxPoolSize:%d, poolSize:%d, resizeCount:%d. option:%v",
		p.address, p.currentSize, p.maxPoolSize, len(p.pool), p.resizeCount, p.opt)
}
