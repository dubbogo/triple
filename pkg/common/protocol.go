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

package common

import (
	"context"
	"fmt"
)

import (
	netTriple "github.com/dubbogo/net/http2/triple"

	perrors "github.com/pkg/errors"
)

import (
	"github.com/dubbogo/triple/pkg/config"
)

type ProtocolHeaderHandlerFactory func(opt *config.Option, ctx context.Context) netTriple.ProtocolHeaderHandler

var protocolHeaderHandlerFactoryMap = make(map[string]ProtocolHeaderHandlerFactory)

func GetProtocolHeaderHandler(opt *config.Option, ctx context.Context) (netTriple.ProtocolHeaderHandler, error) {
	if f, ok := protocolHeaderHandlerFactoryMap[opt.Protocol]; ok {
		return f(opt, ctx), nil
	}
	opt.Logger.Error("Protocol ", opt.Protocol, " header undefined!")
	return nil, perrors.New(fmt.Sprintf("Protocol %s header undefined!", opt.Protocol))
}

func SetProtocolHeaderHandler(protocol string, factory ProtocolHeaderHandlerFactory) {
	protocolHeaderHandlerFactoryMap[protocol] = factory
}

// PackageHandler is to handle http framedata and raw data
type PackageHandler interface {
	Frame2PkgData(frameData []byte) ([]byte, uint32)
	Pkg2FrameData(pkgData []byte) []byte
}

// nolint
type PackageHandlerFactory func() PackageHandler

var packageHandlerFactoryMap = make(map[string]PackageHandlerFactory, 8)

// nolint
func GetPackagerHandler(option *config.Option) (PackageHandler, error) {
	if f, ok := packageHandlerFactoryMap[option.Protocol]; ok {
		return f(), nil
	}
	option.Logger.Error("Protocol ", option.Protocol, " package handler undefined!")
	return nil, perrors.New(fmt.Sprintf("Protocol %s package handler undefined!", option.Protocol))
}

// nolint
func SetPackageHandler(protocol string, f PackageHandlerFactory) {
	packageHandlerFactoryMap[protocol] = f
}
