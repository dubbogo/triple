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
	"github.com/dubbogo/triple/internal/http2"
	"github.com/dubbogo/triple/internal/path"
	"github.com/dubbogo/triple/internal/tools"
	"github.com/dubbogo/triple/pkg/config"
	triHttp2 "github.com/dubbogo/triple/pkg/http2"
	triHttp2Conf "github.com/dubbogo/triple/pkg/http2/config"
)

// TripleServer is the object that can be started and listening remote request
type TripleServer struct {
	http2Server   *triHttp2.Server
	rpcServiceMap *sync.Map

	// config
	opt *config.Option
}

// NewTripleServer can create Server with url and some user impl providers stored in @serviceMap
// @serviceMap should be sync.Map: "interfaceKey" -> Dubbo3GrpcService
func NewTripleServer(serviceMap *sync.Map, opt *config.Option) *TripleServer {
	opt = tools.AddDefaultOption(opt)
	return &TripleServer{
		rpcServiceMap: serviceMap,
		opt:           opt,
	}
}

// Stop
func (t *TripleServer) Stop() {
	t.http2Server.Stop()
}

// Start can start a triple server
func (t *TripleServer) Start() {
	t.opt.Logger.Debug("TripleServer.Start: tripleServer Start at location = ", t.opt.Location)

	t.http2Server = triHttp2.NewServer(t.opt.Location, triHttp2Conf.ServerConfig{
		Logger:                 t.opt.Logger,
		PathExtractor:          path.NewDefaultExtractor(),
		HandlerGRManagedByUser: true,
	})
	tripleCtl, err := http2.NewTripleController(t.opt)
	if err != nil {
		t.opt.Logger.Error("TripleServer.Start: new http2 controller failed with error = %v", err)
		return
	}

	t.rpcServiceMap.Range(func(key, value interface{}) bool {
		t.opt.Logger.Debugf("TripleServer.Start: http2 register path = %s, with service = %+v", key.(string), value)
		t.http2Server.RegisterHandler(key.(string), tripleCtl.GetHandler(value))
		return true
	})

	t.http2Server.Start()
}

func (t *TripleServer) RefreshService() {
	t.opt.Logger.Debugf("TripleServer.Refresh: call refresh services")
	tripleCtl, err := http2.NewTripleController(t.opt)
	if err != nil {
		t.opt.Logger.Errorf("TripleServer.Refresh: new http2 controller failed with error = %v", err)
		return
	}

	t.rpcServiceMap.Range(func(key, value interface{}) bool {
		t.opt.Logger.Debugf("TripleServer.Refresh: http2 register path = %s, with service = %+v", key.(string), value)
		t.http2Server.RegisterHandler(key.(string), tripleCtl.GetHandler(value))
		return true
	})
}
