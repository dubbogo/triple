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
	"context"
	"reflect"
	"sync"
)

import (
	"google.golang.org/grpc"
)

import (
	"github.com/dubbogo/triple/internal/http2_handler"
	"github.com/dubbogo/triple/internal/tools"
	"github.com/dubbogo/triple/pkg/common"
	"github.com/dubbogo/triple/pkg/common/constant"
	"github.com/dubbogo/triple/pkg/config"
)

// TripleClient client endpoint that using triple protocol
type TripleClient struct {
	h2Controller *http2_handler.H2Controller

	stubInvoker reflect.Value

	//once is used when destroy
	once sync.Once

	// triple config
	opt *config.Option

	// serializer is triple serializer to do codec
	serializer common.Codec
}

// NewTripleClient creates triple client
// it returns tripleClient, which contains invoker and triple connection.
// @impl must have method: GetDubboStub(cc *dubbo3.TripleConn) interface{}, to be capable with grpc
// @opt is used to init http2 controller, if it's nil, use the default config
func NewTripleClient(impl interface{}, opt *config.Option) (*TripleClient, error) {
	opt = tools.AddDefaultOption(opt)
	h2Controller, err := http2_handler.NewH2Controller(opt)
	if err != nil {
		opt.Logger.Errorf("NewH2Controller err = %v", err)
		return nil, err
	}
	tripleClient := &TripleClient{
		opt:          opt,
		h2Controller: h2Controller,
	}

	// put dubbo3 network logic to tripleConn, creat pb stub invoker
	if opt.CodecType == constant.PBCodecName {
		tripleClient.stubInvoker = reflect.ValueOf(getInvoker(impl, newTripleConn(tripleClient)))
	}

	return tripleClient, nil
}

// Invoke call remote using stub
func (t *TripleClient) Invoke(methodName string, in []reflect.Value, reply interface{}) error {
	if t.opt.CodecType == constant.PBCodecName {
		method := t.stubInvoker.MethodByName(methodName)
		res := method.Call(in)
		if res[1].IsValid() && res[1].Interface() != nil {
			return res[1].Interface().(error)
		}
		_ = tools.ReflectResponse(res[0], reply)
	} else {
		ctx := in[0].Interface().(context.Context)
		interfaceKey := ctx.Value(constant.InterfaceKey).(string)
		err := t.Request(ctx, "/"+interfaceKey+"/"+methodName, in[1].Interface(), reply)
		if err != nil {
			return err
		}
	}
	return nil
}

// Request call h2Controller to send unary rpc req to server
// @path is /interfaceKey/functionName e.g. /com.apache.dubbo.sample.basic.IGreeter/BigUnaryTest
// @arg is request body
func (t *TripleClient) Request(ctx context.Context, path string, arg, reply interface{}) error {
	return t.h2Controller.UnaryInvoke(ctx, path, arg, reply)
}

// StreamRequest call h2Controller to send streaming request to sever, to start link.
// @path is /interfaceKey/functionName e.g. /com.apache.dubbo.sample.basic.IGreeter/BigStreamTest
func (t *TripleClient) StreamRequest(ctx context.Context, path string) (grpc.ClientStream, error) {
	return t.h2Controller.StreamInvoke(ctx, path)
}

// Close destroy http controller and return
func (t *TripleClient) Close() {
	t.opt.Logger.Debug("Triple Client Is closing")
	t.h2Controller.Destroy()
}

// IsAvailable returns if triple client is available
func (t *TripleClient) IsAvailable() bool {
	return t.h2Controller.IsAvailable()
}
