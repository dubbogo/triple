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
	"time"
)

import (
	"github.com/dubbogo/grpc-go"
)

import (
	"github.com/dubbogo/triple/pkg/common"
)

// TripleConn is the struct that called in pb.go file
// Its client field contains all net logic of dubbo3
type TripleConn struct {
	timeout  time.Duration
	grpcConn *grpc.ClientConn
}

// Invoke called by unary rpc 's pb.go file in dubbo-go 3.0 design
// @method is /interfaceKey/functionName e.g. /com.apache.dubbo.sample.basic.IGreeter/BigUnaryTest
// @arg is request body, must be proto.Message type
func (t *TripleConn) Invoke(ctx context.Context, method string, args, reply interface{}, opts ...grpc.CallOption) common.ErrorWithAttachment {
	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()
	trailer, err := t.grpcConn.Invoke(ctx, method, args, reply, opts...)
	//return t.client.Request(ctx, method, args, reply)
	atta := make(common.DubboAttachment, len(trailer))
	for k, v := range trailer {
		atta[k] = v
	}
	return *common.NewErrorWithAttachment(err, atta)
}

// NewStream called when streaming rpc 's pb.go file
// @method is /interfaceKey/functionName e.g. /com.apache.dubbo.sample.basic.IGreeter/BigStreamTest
func (t *TripleConn) NewStream(ctx context.Context, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return grpc.NewClientStream(ctx, t.grpcConn, method, opts...)
}

// newTripleConn new a triple conn with given @tripleclient, which contains all net logic
func newTripleConn(timeout time.Duration, address string, opts ...grpc.DialOption) *TripleConn {
	//grpcConn, _ := grpc.Dial(address,grpc.WithInsecure())
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	grpcConn, _ := grpc.DialContext(ctx, address, opts...)
	return &TripleConn{
		timeout:  timeout,
		grpcConn: grpcConn,
	}
}

// getInvoker return invoker that have service method
func getInvoker(impl interface{}, conn *TripleConn) interface{} {
	in := make([]reflect.Value, 0, 16)
	in = append(in, reflect.ValueOf(conn))

	method := reflect.ValueOf(impl).MethodByName("GetDubboStub")
	res := method.Call(in)
	// res[0] is a struct that contains SayHello method, res[0] is greeter Client in example
	// it's SayHello methodwill call specific of conn's invoker.
	return res[0].Interface()
}
