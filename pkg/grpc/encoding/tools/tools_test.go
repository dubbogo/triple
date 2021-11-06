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

package tools

import (
	"testing"
)

import (
	"github.com/stretchr/testify/assert"
)

import (
	"github.com/dubbogo/triple/pkg/common/constant"
	"github.com/dubbogo/triple/pkg/config"
)

func TestValidate(t *testing.T) {
	opt := config.NewTripleOption()
	opt.Validate()
	assert.Equal(t, uint32(constant.DefaultTimeout), opt.Timeout)
	assert.Equal(t, uint32(constant.DefaultHttp2ControllerReadBufferSize), opt.BufferSize)
	assert.Equal(t, constant.DefaultListeningAddress, opt.Location)
	assert.Equal(t, constant.TRIPLE, opt.Protocol)
	assert.Equal(t, constant.PBCodecName, opt.CodecType)
}
