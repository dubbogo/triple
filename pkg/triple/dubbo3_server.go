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
	"fmt"
	"net"
	"reflect"
	"sync"
)

import (
	hessian "github.com/apache/dubbo-go-hessian2"
	"github.com/dubbogo/gost/log/logger"
	"github.com/dubbogo/grpc-go"
	"github.com/dubbogo/grpc-go/encoding"
	"github.com/dubbogo/grpc-go/metadata"
	perrors "github.com/pkg/errors"
)

import (
	"github.com/dubbogo/triple/pkg/common"
	"github.com/dubbogo/triple/pkg/common/constant"
	"github.com/dubbogo/triple/pkg/common/encoding"
	hessianGRPCCodec "github.com/dubbogo/triple/pkg/common/encoding/hessian"
	"github.com/dubbogo/triple/pkg/common/encoding/java_type"
	"github.com/dubbogo/triple/pkg/common/encoding/msgpack"
	"github.com/dubbogo/triple/pkg/common/encoding/proto_wrapper_api"
	"github.com/dubbogo/triple/pkg/common/encoding/raw_proto"
	"github.com/dubbogo/triple/pkg/config"
	"github.com/dubbogo/triple/pkg/tracing"
)

// TripleServer is the object that can be started and listening remote request
type TripleServer struct {
	lst           net.Listener
	grpcServer    *grpc.Server
	rpcServiceMap *sync.Map
	registeredKey map[string]bool
	// config
	opt *config.Option
}

// NewTripleServer can create Server with url and some user impl providers stored in @serviceMap
// @serviceMap should be sync.Map: "interfaceKey" -> Dubbo3GrpcService
func NewTripleServer(serviceMap *sync.Map, opt *config.Option) *TripleServer {
	if opt == nil {
		opt = config.NewTripleOption()
	}
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

func newGenericCodec() common.GenericCodec {
	return &GenericCodec{
		codec: raw_proto.NewProtobufCodec(),
	}
}

// GenericCodec is pb impl of TwoWayCodec
type GenericCodec struct {
	codec encoding.Codec
}

// UnmarshalRequest unmarshal bytes @data to interface
func (h *GenericCodec) UnmarshalRequest(data []byte) ([]interface{}, error) {
	wrapperRequest := proto_wrapper_api.TripleRequestWrapper{}
	err := h.codec.Unmarshal(data, &wrapperRequest)
	if err != nil {
		return nil, err
	}
	result := make([]interface{}, 0, len(wrapperRequest.Args))

	for _, value := range wrapperRequest.Args {
		decoder := hessian.NewDecoder(value)
		val, err := decoder.Decode()
		if err != nil {
			return nil, err
		}
		result = append(result, val)
	}
	return result, nil
}

func createGrpcDesc(serviceName string, service common.TripleUnaryService) *grpc.ServiceDesc {
	desc := grpc.ServiceDesc{
		ServiceName: serviceName,
		HandlerType: (*common.TripleUnaryService)(nil),
		Methods:     make([]grpc.MethodDesc, 0),
	}

	methods := service.GetServiceMethods()
	for _, methodName := range methods {
		desc.Methods = append(desc.Methods, grpc.MethodDesc{
			MethodName: methodName,
			Handler:    newMethodHandler(service, methodName),
		})
	}

	return &desc
}

func newMethodHandler(service common.TripleUnaryService, methodName string) func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	genericCodec := newGenericCodec()
	handler := func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
		// invoke
		responseAttachment, _ := ctx.Value("XXX_TRIPLE_GO_RESPONSE_ATTACHMENT").(metadata.MD)
		genericPayload, ok := ctx.Value("XXX_TRIPLE_GO_GENERIC_PAYLOAD").([]byte)
		base := srv.(common.TripleUnaryService)
		if methodName == "$invoke" && ok {
			args, err := genericCodec.UnmarshalRequest(genericPayload)
			if err != nil {
				return nil, perrors.Errorf("unaryProcessor.processUnaryRPC: generic invoke with request %s unmarshal error = %s", string(genericPayload), err.Error())
			}
			return base.InvokeWithArgs(ctx, methodName, args)
		} else {
			wrapperRequest := proto_wrapper_api.TripleRequestWrapper{}

			if err := dec(&wrapperRequest); err != nil {
				return nil, err
			}

			// TODO: abstract get Codec.
			var innerCodec encoding.Codec
			switch wrapperRequest.SerializeType {
			case "raw_hessian2":
				innerCodec = hessianGRPCCodec.NewHessianCodec()
			case "raw_msgpack":
				innerCodec = msgpack.NewMsgPackCodec()
			default:
				innerCodec, _ = common.GetTripleCodec(constant.CodecType(wrapperRequest.SerializeType))
			}

			reqParam, ok := service.GetReqParamsInterfaces(methodName)
			if !ok {
				return nil, perrors.Errorf("method name %s is not provided by service, please check if correct", methodName)
			}

			if len(reqParam) != len(wrapperRequest.Args) {
				return nil, perrors.Errorf("error ,request params len is %d, but exported method has %d", len(wrapperRequest.Args), len(reqParam))
			}

			for idx, value := range wrapperRequest.Args {
				if err := innerCodec.Unmarshal(value, reqParam[idx]); err != nil {
					return nil, err
				}
			}

			args := make([]interface{}, 0, len(reqParam))
			for _, v := range reqParam {
				tempParamObj := reflect.ValueOf(v).Elem().Interface()
				args = append(args, tempParamObj)
			}

			// Get local invoke rawReplyStruct
			reply, replyerr := base.InvokeWithArgs(ctx, methodName, args)

			// Prepare out attachments
			var rawReplyStruct interface{}
			if result, ok := reply.(grpc.OuterResult); ok {
				outerAttachment := result.Attachments()
				for k, v := range outerAttachment {
					if str, ok := v.(string); ok {
						responseAttachment[k] = []string{str}
					} else if strs, ok := v.([]string); ok {
						responseAttachment[k] = strs
					}
					// todo deal with unsupported attachment
					logger.Warnf("The attachment %v is unsupported now!", v)
				}
				rawReplyStruct = result.Result()
			}

			// Wrap reply with TripleResponseWrapper
			data, err := innerCodec.Marshal(rawReplyStruct)
			if err != nil {
				return nil, err
			}

			wrapperResp := &proto_wrapper_api.TripleResponseWrapper{
				SerializeType: wrapperRequest.SerializeType,
				Data:          data,
				Type:          java_type.GetArgType(rawReplyStruct),
			}
			// wrap reply with wrapperResponse
			return wrapperResp, replyerr
		}
	}

	return handler
}

func newGrpcServerWithCodec(opt *config.Option) *grpc.Server {
	var innerCodec encoding.Codec
	serverOpts := []grpc.ServerOption{}

	if opt.JaegerAddress != "" {
		tracer := tracing.NewJaegerTracerDirect(opt.JaegerServiceName, opt.JaegerAddress, opt.Logger)
		serverOpts = append(serverOpts,
			grpc.UnaryInterceptor(tracing.OpenTracingServerInterceptor(tracer)),
			grpc.StreamInterceptor(tracing.OpenTracingStreamServerInterceptor(tracer)),
		)
	}

	if opt.GRPCMaxServerRecvMsgSize != 0 {
		serverOpts = append(serverOpts, grpc.MaxRecvMsgSize(opt.GRPCMaxServerSendMsgSize))
	}
	if opt.GRPCMaxCallSendMsgSize != 0 {
		serverOpts = append(serverOpts, grpc.MaxSendMsgSize(opt.GRPCMaxServerRecvMsgSize))
	}

	if opt.ProxyModeEnable {
		serverOpts = append(serverOpts, grpc.ProxyModeEnable(true))
	}

	var err error
	switch opt.CodecType {
	case constant.PBCodecName:
		return grpc.NewServer(serverOpts...)
	case constant.HessianCodecName:
		innerCodec = hessianGRPCCodec.NewHessianCodec()
	case constant.MsgPackCodecName:
		innerCodec = msgpack.NewMsgPackCodec()
	default:
		innerCodec, err = common.GetTripleCodec(opt.CodecType)
		if err != nil {
			fmt.Printf("TripleServer.Start: serialization %s not supported", opt.CodecType)
		}
	}
	serverOpts = append(serverOpts, grpc.ForceServerCodec(pbwrapper.NewPBWrapperTwoWayCodec(string(opt.CodecType), innerCodec, raw_proto.NewProtobufCodec())))

	return grpc.NewServer(serverOpts...)
}

// Start can start a triple server
func (t *TripleServer) Start() {
	lst, err := net.Listen("tcp", t.opt.Location)
	if err != nil {
		panic(err)
	}
	grpcServer := newGrpcServerWithCodec(t.opt)
	t.rpcServiceMap.Range(func(key, value interface{}) bool {
		t.registeredKey[key.(string)] = true
		grpcService, ok := value.(common.TripleGrpcService)
		if ok {
			desc := grpcService.XXX_ServiceDesc()
			desc.ServiceName = key.(string)
			grpcServer.RegisterService(desc, value)
		} else {
			desc := createGrpcDesc(key.(string), value.(common.TripleUnaryService))
			grpcServer.RegisterService(desc, value)
		}
		if key == "grpc.reflection.v1alpha.ServerReflection" {
			grpcService.(common.TripleGrpcReflectService).SetGRPCServer(grpcServer)
		}
		return true
	})

	go grpcServer.Serve(lst)
	t.lst = lst
	t.grpcServer = grpcServer
}

func (t *TripleServer) RefreshService() {
	t.opt.Logger.Debugf("TripleServer.Refresh: call refresh services")
	grpcServer := newGrpcServerWithCodec(t.opt)
	t.rpcServiceMap.Range(func(key, value interface{}) bool {
		grpcService, ok := value.(common.TripleGrpcService)
		if ok {
			desc := grpcService.XXX_ServiceDesc()
			desc.ServiceName = key.(string)
			grpcServer.RegisterService(desc, value)
		} else {
			desc := createGrpcDesc(key.(string), value.(common.TripleUnaryService))
			grpcServer.RegisterService(desc, value)
		}
		if key == "grpc.reflection.v1alpha.ServerReflection" {
			grpcService.(common.TripleGrpcReflectService).SetGRPCServer(grpcServer)
		}
		return true
	})
	t.grpcServer.Stop()
	t.lst.Close()
	lst, _ := net.Listen("tcp", t.opt.Location)
	go grpcServer.Serve(lst)
	t.grpcServer = grpcServer
	t.lst = lst
}
