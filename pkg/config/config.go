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

package config

import (
	"time"

	"github.com/dubbogo/triple/pkg/common/constant"
	loggerInterface "github.com/dubbogo/triple/pkg/common/logger"
	"github.com/dubbogo/triple/pkg/common/logger/default_logger"
)

// triple option
type Option struct {
	// network opts
	Timeout    time.Duration
	BufferSize uint32

	// service opts
	Location  string
	Protocol  string
	CodecType constant.CodecType
	//SerializerTypeInWrapper  is used in pbWrapperCodec, to write serializeType field, if empty, use Option.CodecType as default
	SerializerTypeInWrapper string

	// triple header opts
	HeaderGroup      string
	HeaderAppVersion string

	// grpc opts
	GRPCMaxCallSendMsgSize   int
	GRPCMaxServerSendMsgSize int
	GRPCMaxCallRecvMsgSize   int
	GRPCMaxServerRecvMsgSize int

	// tracing
	JaegerAddress     string
	JaegerServiceName string
	JaegerUseAgent    bool

	// logger
	Logger loggerInterface.Logger

	// NumWorkers is num of gr in ConnectionPool
	NumWorkers uint32

	// proxy mode for gateway
	ProxyModeEnable bool

	//tls
	CACertFile    string
	TLSCertFile   string
	TLSKeyFile    string
	TLSServerName string
}

// Validate sets empty field to default config
func (o *Option) Validate() {
	if o.Timeout == time.Duration(0) {
		o.Timeout = constant.DefaultTimeout
	}

	if o.BufferSize == uint32(0) {
		o.BufferSize = uint32(constant.DefaultHttp2ControllerReadBufferSize)
	}

	if o.Location == "" {
		o.Location = constant.DefaultListeningAddress
	}

	if o.Logger == nil {
		o.Logger = default_logger.GetDefaultLogger()
	}

	if o.Protocol == "" {
		o.Protocol = constant.TRIPLE
	}

	if o.CodecType == "" {
		o.CodecType = constant.PBCodecName
	}

	if o.NumWorkers <= 0 {
		o.NumWorkers = constant.DefaultNumWorkers
	}
}

// nolint
type OptionFunction func(o *Option)

// NewTripleOption return Triple Option with given config defined by @fs
func NewTripleOption(fs ...OptionFunction) *Option {
	opt := &Option{}
	for _, v := range fs {
		v(opt)
	}
	opt.Validate()
	return opt
}

// WithClientTimeout return OptionFunction with timeout of @timeout
func WithClientTimeout(timeout time.Duration) OptionFunction {
	return func(o *Option) {
		o.Timeout = timeout
	}
}

// WithBufferSize return OptionFunction with buffer read size of @size
func WithBufferSize(size uint32) OptionFunction {
	return func(o *Option) {
		o.BufferSize = size
	}
}

// WithCodecType return OptionFunction with target @serializerType, now we support "protobuf" and "hessian2"
func WithCodecType(serializerType constant.CodecType) OptionFunction {
	return func(o *Option) {
		o.CodecType = serializerType
	}
}

// WithProtocol return OptionFunction with target @protocol, now we support "tri"
func WithProtocol(protocol string) OptionFunction {
	return func(o *Option) {
		o.Protocol = protocol
	}
}

// WithLocation return OptionFunction with target @location, for example "127.0.0.1:20001"
func WithLocation(location string) OptionFunction {
	return func(o *Option) {
		o.Location = location
	}
}

// WithHeaderAppVersion return OptionFunction with target @appVersion, for example "1.0.0"
func WithHeaderAppVersion(appVersion string) OptionFunction {
	return func(o *Option) {
		o.HeaderAppVersion = appVersion
	}
}

// WithHeaderGroup return OptionFunction with target @group, for example "dubbogo"
func WithHeaderGroup(group string) OptionFunction {
	return func(o *Option) {
		o.HeaderGroup = group
	}
}

// WithLogger return OptionFunction with target @logger, which must impl triple/pkg/common/logger.Logger
// the input @logger should be AddCallerSkip(1)
func WithLogger(logger loggerInterface.Logger) OptionFunction {
	return func(o *Option) {
		o.Logger = loggerInterface.NewLoggerWrapper(logger)
	}
}

// WithSerializerTypeInWrapper return OptionFunction with target @name as SerializerTypeInWrapper
func WithSerializerTypeInWrapper(name string) OptionFunction {
	return func(o *Option) {
		o.SerializerTypeInWrapper = name
	}
}

func WithNumWorker(numWorkers uint32) OptionFunction {
	return func(o *Option) {
		o.NumWorkers = numWorkers
	}
}

func WithGRPCMaxCallSendMessageSize(maxCallSendMsgSize int) OptionFunction {
	return func(o *Option) {
		o.GRPCMaxCallSendMsgSize = maxCallSendMsgSize
	}
}

func WithGRPCMaxCallRecvMessageSize(maxCallRecvMsgSize int) OptionFunction {
	return func(o *Option) {
		o.GRPCMaxCallRecvMsgSize = maxCallRecvMsgSize
	}
}

func WithGRPCMaxServerSendMessageSize(maxServerSendMsgSize int) OptionFunction {
	return func(o *Option) {
		o.GRPCMaxServerSendMsgSize = maxServerSendMsgSize
	}
}

func WithGRPCMaxServerRecvMessageSize(maxServerRecvMsgSize int) OptionFunction {
	return func(o *Option) {
		o.GRPCMaxServerRecvMsgSize = maxServerRecvMsgSize
	}
}

func WithJaegerConfig(address, serviceName string, useAgent bool) OptionFunction {
	return func(o *Option) {
		o.JaegerAddress = address
		o.JaegerServiceName = serviceName
		o.JaegerUseAgent = useAgent
	}
}

func WithProxyModeEnable(enable bool) OptionFunction {
	return func(o *Option) {
		o.ProxyModeEnable = enable
	}
}
