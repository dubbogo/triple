/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package triple

import (
	"io"
	"net"
	"net/http"
	"runtime"
	"sync"
	"time"
)

import (
	"github.com/dubbogo/net/http2"
)

import (
	"github.com/dubbogo/triple/internal/tools"
	"github.com/dubbogo/triple/pkg/config"
)

// TripleServer is the object that can be started and listening remote request
type TripleServer struct {
	lst           net.Listener
	rpcServiceMap *sync.Map
	h2Controller  *H2Controller
	done          chan struct{}

	// config
	opt *config.Option
}

// NewTripleServer can create Server with url and some user impl providers stored in @serviceMap
// @serviceMap should be sync.Map: "interfaceKey" -> Dubbo3GrpcService
func NewTripleServer(serviceMap *sync.Map, opt *config.Option) *TripleServer {
	opt = tools.AddDefaultOption(opt)
	return &TripleServer{
		rpcServiceMap: serviceMap,
		done:          make(chan struct{}, 1),
		opt:           opt,
	}
}

// Stop
func (t *TripleServer) Stop() {
	if t.h2Controller != nil {
		t.h2Controller.Destroy()
	}
	t.done <- struct{}{}
}

// Start can start a triple server
func (t *TripleServer) Start() {
	t.opt.Logger.Debug("tripleServer Start at ", t.opt.Location)

	lst, err := net.Listen("tcp", t.opt.Location)
	if err != nil {
		panic(err)
	}

	t.lst = lst

	go t.run()
}

const (
	// DefaultMaxSleepTime max sleep interval in accept
	DefaultMaxSleepTime = 1 * time.Second
	// DefaultListenerTimeout tcp listener timeout
	DefaultListenerTimeout = 1.5e9
)

// run can start a loop to accept tcp conn
func (t *TripleServer) run() {
	var (
		ok       bool
		ne       net.Error
		tmpDelay time.Duration
	)

	tl := t.lst.(*net.TCPListener)
	for {
		select {
		case <-t.done:
			return
		default:
		}

		if tl != nil {
			tl.SetDeadline(time.Now().Add(DefaultListenerTimeout))
		}
		c, err := t.lst.Accept()
		if err != nil {
			if ne, ok = err.(net.Error); ok && (ne.Temporary() || ne.Timeout()) {
				if tmpDelay != 0 {
					tmpDelay <<= 1
				} else {
					tmpDelay = 5 * time.Millisecond
				}
				if tmpDelay > DefaultMaxSleepTime {
					tmpDelay = DefaultMaxSleepTime
				}
				time.Sleep(tmpDelay)
				continue
			}
			return
		}

		go func() {
			defer func() {
				if r := recover(); r != nil {
					const size = 64 << 10
					buf := make([]byte, size)
					buf = buf[:runtime.Stack(buf, false)]
					t.opt.Logger.Errorf("http: panic serving %v: %v\n%s", c.RemoteAddr(), r, buf)
					c.Close()
				}
			}()

			if err := t.handleRawConn(c); err != nil && err != io.EOF {
				t.opt.Logger.Error(" handle raw conn err = ", err)
			}
		}()
	}
}

// handleRawConn create a H2 Controller to deal with new conn
func (t *TripleServer) handleRawConn(conn net.Conn) error {
	srv := &http2.Server{}
	h2Controller, err := NewH2Controller(true, t.rpcServiceMap, t.opt)
	if err != nil {
		return err
	}
	t.h2Controller = h2Controller
	opts := &http2.ServeConnOpts{Handler: http.HandlerFunc(h2Controller.GetHandler())}
	srv.ServeConn(conn, opts)
	return nil
}
