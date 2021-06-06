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
	"testing"
)

import (
	"github.com/stretchr/testify/assert"
)

import (
	"github.com/dubbogo/triple/pkg/common/constant"
	"github.com/dubbogo/triple/pkg/common/logger/default_logger"
)

func TestNewTripleOption(t *testing.T) {
	opt := NewTripleOption()
	assert.NotNil(t, opt)
}

func TestWithClientTimeout(t *testing.T) {
	opt := NewTripleOption(
		WithClientTimeout(120),
	)
	assert.NotNil(t, opt)
	assert.Equal(t, opt.Timeout, uint32(120))
}

func TestWithSerializerType(t *testing.T) {
	opt := NewTripleOption(
		WithSerializerType(constant.HessianCodecName),
	)
	assert.NotNil(t, opt)
	assert.Equal(t, opt.CodecType, constant.HessianCodecName)

	opt = NewTripleOption(
		WithSerializerType(constant.PBCodecName),
	)
	assert.NotNil(t, opt)
	assert.Equal(t, opt.CodecType, constant.PBCodecName)
}

func TestWithBufferSize(t *testing.T) {
	opt := NewTripleOption(
		WithBufferSize(100000),
	)
	assert.NotNil(t, opt)
	assert.Equal(t, opt.BufferSize, uint32(100000))
}

func TestOption_SetEmptyFieldDefaultConfig(t *testing.T) {
	opt := NewTripleOption(
		WithBufferSize(100000),
	)
	assert.NotNil(t, opt)
	assert.Equal(t, uint32(100000), opt.BufferSize)
	assert.Equal(t, uint32(0), opt.Timeout)
	opt.Validate()
	assert.Equal(t, uint32(100000), opt.BufferSize)
	assert.Equal(t, uint32(constant.DefaultTimeout), opt.Timeout)

	opt = NewTripleOption()
	assert.Equal(t, uint32(0), opt.BufferSize)
	assert.Equal(t, uint32(0), opt.Timeout)
	opt.Validate()
	assert.Equal(t, uint32(constant.DefaultHttp2ControllerReadBufferSize), opt.BufferSize)
	assert.Equal(t, uint32(constant.DefaultTimeout), opt.Timeout)
}

func TestWithHeaderAppVersion(t *testing.T) {
	opt := NewTripleOption(
		WithHeaderAppVersion("1.0.0"),
	)
	assert.NotNil(t, opt)
	assert.Equal(t, "1.0.0", opt.HeaderAppVersion)
}

func TestWithHeaderAppGroup(t *testing.T) {
	opt := NewTripleOption(
		WithHeaderGroup("dubbo"),
	)
	assert.NotNil(t, opt)
	assert.Equal(t, "dubbo", opt.HeaderGroup)
}

func TestWithLogger(t *testing.T) {
	opt := NewTripleOption(
		WithLogger(default_logger.GetDefaultLogger()),
	)
	assert.NotNil(t, opt)
	assert.NotNil(t, opt.Logger)
}

func TestValidate(t *testing.T) {
	opt := NewTripleOption()
	opt.Validate()
	assert.Equal(t, uint32(constant.DefaultTimeout), opt.Timeout)
	assert.Equal(t, uint32(constant.DefaultHttp2ControllerReadBufferSize), opt.BufferSize)
	assert.Equal(t, constant.DefaultListeningAddress, opt.Location)
	assert.Equal(t, constant.TRIPLE, opt.Protocol)
	assert.Equal(t, constant.PBCodecName, opt.CodecType)
}
