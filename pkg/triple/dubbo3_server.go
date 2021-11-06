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

	"net"
)

import (
	"github.com/dubbogo/triple/pkg/grpc"
	perrors "github.com/pkg/errors"
)

import (
	"github.com/dubbogo/triple/pkg/common"
	"github.com/dubbogo/triple/pkg/config"
)

// TripleServer is the object that can be started and listening remote request
type TripleServer struct {
	lst           net.Listener
	rpcServiceMap *sync.Map
	registeredKey map[string]bool
	// config
	opt *config.Option
}

// NewTripleServer can create Server with url and some user impl providers stored in @serviceMap
// @serviceMap should be sync.Map: "interfaceKey" -> Dubbo3GrpcService
func NewTripleServer(serviceMap *sync.Map, opt *config.Option) *TripleServer {
	return &TripleServer{
		rpcServiceMap: serviceMap,
		opt:           opt,
		registeredKey: make(map[string]bool),
	}
}

// Stop
func (t *TripleServer) Stop() {
	t.lst.Close()
}

/*
var Greeter_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "api.Greeter",
	HandlerType: (*GreeterServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "SayHello",
			Handler:    _Greeter_SayHello_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "SayHelloStream",
			Handler:       _Greeter_SayHelloStream_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "samples_api.proto",
}

*/

/*

func _Greeter_SayHello_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(HelloRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	base := srv.(dubbo3.Dubbo3GrpcService)
	args := []interface{}{}
	args = append(args, in)
	invo := invocation.NewRPCInvocation("SayHello", args, nil)
	if interceptor == nil {
		result := base.XXX_GetProxyImpl().Invoke(ctx, invo)
		return result, result.Error()
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/api.Greeter/SayHello",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GreeterServer).SayHello(ctx, req.(*HelloRequest))
	}
	return interceptor(ctx, in, info, handler)
}
*/

func createGrpcDesc(serviceName string, service common.TripleUnaryService) *grpc.ServiceDesc {
	return &grpc.ServiceDesc{
		ServiceName: serviceName,
		HandlerType: (*common.TripleUnaryService)(nil),
		Methods: []grpc.MethodDesc{
			{
				MethodName: "InvokeWithArgs",
				Handler: func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
					methodName := ctx.Value("XXX_TRIPLE_GO_METHOD_NAME").(string)
					base := srv.(common.TripleUnaryService)
					if methodName == "$invoke" {
						var args []interface{}
						// todo generic invocation
						//args, err = p.genericCodec.UnmarshalRequest(readBuf)
						//if err != nil {
						//	p.opt.Logger.Errorf("unaryProcessor.processUnaryRPC: generic invoke with request %s unmarshal error = %s", string(readBuf), err.Error())
						//	return nil, *common.NewErrorWithAttachment(status.Errorf(codes.Internal, "generic invoke with request %s unmarshal error = %s", string(readBuf), err.Error()), responseAttachment)
						//}
						//p.opt.Logger.Debugf("unaryProcessor.processUnaryRPC: generic invoke service with header %+v and args %v", header, args)
						return base.InvokeWithArgs(ctx, methodName, args)
					} else {
						reqParam, ok := service.GetReqParamsInterfaces(methodName)
						if !ok {
							return nil, perrors.Errorf("method name %s is not provided by service, please check if correct", methodName)
						}
						if e := dec(reqParam); e != nil {
							return nil, e
						}
						args := make([]interface{}, 0, len(reqParam))
						for _, v := range reqParam {
							tempParamObj := reflect.ValueOf(v).Elem().Interface()
							args = append(args, tempParamObj)
						}
						return base.InvokeWithArgs(ctx, methodName, args)
					}
				},
			},
		},
	}
}

// Start can start a triple server
func (t *TripleServer) Start() {
	grpcServer := grpc.NewServer()
	lst, err := net.Listen("tcp", t.opt.Location)
	if err != nil {
		panic(err)
	}

	t.rpcServiceMap.Range(func(key, value interface{}) bool {
		t.registeredKey[key.(string)] = true
		grpcService, ok := value.(common.TripleGrpcService)
		if ok {
			desc := grpcService.XXX_ServiceDesc()
			desc.ServiceName = key.(string)
			grpcServer.RegisterService(desc, value)
			return true
		}
		desc := createGrpcDesc(key.(string), value.(common.TripleUnaryService))
		grpcServer.RegisterService(desc, value)
		return true
	})

	go grpcServer.Serve(lst)
	t.lst = lst
}

func (t *TripleServer) RefreshService() {
	t.opt.Logger.Debugf("TripleServer.Refresh: call refresh services")
	t.rpcServiceMap.Range(func(key, value interface{}) bool {
		if _, ok := t.registeredKey[key.(string)]; ok {
			return true
		}
		grpcServer := grpc.NewServer()
		grpcService, ok := value.(common.TripleGrpcService)
		if ok {
			desc := grpcService.XXX_ServiceDesc()
			desc.ServiceName = key.(string)
			grpcServer.RegisterService(desc, value)
			return true
		}
		desc := createGrpcDesc(key.(string), value.(common.TripleUnaryService))
		grpcServer.RegisterService(desc, value)
		go grpcServer.Serve(t.lst)
		return true
	})
}
