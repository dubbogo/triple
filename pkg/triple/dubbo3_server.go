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
	"sync"
)

import (
	"github.com/dubbogo/triple/internal/http2_handler"
	"github.com/dubbogo/triple/internal/path_matcher"
	"github.com/dubbogo/triple/internal/tools"
	"github.com/dubbogo/triple/pkg/config"
	triHttp2 "github.com/dubbogo/triple/pkg/http2"
	triHttp2Conf "github.com/dubbogo/triple/pkg/http2/config"
)

// TripleServer is the object that can be started and listening remote request
type TripleServer struct {
	http2Server   *triHttp2.Http2Server
	rpcServiceMap *sync.Map
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
	t.http2Server.Stop()
	t.done <- struct{}{}
}

// Start can start a triple server
func (t *TripleServer) Start() {
	t.opt.Logger.Debug("tripleServer Start at ", t.opt.Location)

	t.http2Server = triHttp2.NewHttp2Server(t.opt.Location, triHttp2Conf.ServerConfig{
		Logger:             t.opt.Logger,
		PathHandlerMatcher: path_matcher.NewDefaultPathHandlerMatcher(),
	})

	h2Handler, err := http2_handler.NewH2Controller(t.opt)
	if err != nil {
		t.opt.Logger.Error("new http2 controller failed with error = %v", err)
		return
	}

	t.rpcServiceMap.Range(func(key, value interface{}) bool {
		t.http2Server.RegisterHandler(key.(string), h2Handler.GetHandler(value))
		return true
	})

	t.http2Server.Start()
}
