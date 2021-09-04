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

package stream

import (
	"testing"
	"time"
)

import (
	"github.com/stretchr/testify/assert"

	"google.golang.org/grpc"
)

import (
	"github.com/dubbogo/triple/internal/message"
)

type TestRPCService struct {
}

func (t *TestRPCService) ServiceDesc() *grpc.ServiceDesc {
	return nil
}

func TestBaseUserStream(t *testing.T) {
	service := &TestRPCService{}
	baseUserStream := newBaseStream(service)
	assert.NotNil(t, baseUserStream)
	assert.Equal(t, baseUserStream.service, service)

	// get msg
	sendGetChan := baseUserStream.GetSend()
	recvGetChan := baseUserStream.GetRecv()
	closeChan := make(chan struct{})
	testData := []byte("test message of TestBaseUserStream")
	counter := 0
	go func() {
		for {
			select {
			case <-closeChan:
				// todo
				//baseUserStre//am.putRecvErr(errors.New("close error"))
				return
			case sendGetBuf := <-sendGetChan:
				counter++
				assert.Equal(t, sendGetBuf.Bytes(), testData)
				assert.Equal(t, sendGetBuf.MsgType, message.DataMsgType)
			case recvGetBuf := <-recvGetChan:
				counter++
				assert.Equal(t, recvGetBuf.Bytes(), testData)
				assert.Equal(t, recvGetBuf.MsgType, message.ServerStreamCloseMsgType)
			}
		}

	}()

	// put msg
	for i := 0; i < 500; i++ {
		baseUserStream.PutRecv(testData, message.ServerStreamCloseMsgType)
		baseUserStream.PutSend(testData, make(map[string]string), message.DataMsgType)
	}
	time.Sleep(time.Second)
	assert.Equal(t, counter, 1000)
	closeChan <- struct{}{}
	// test close with error
	//closeMsg := <-recvGetChan
	//assert.Equal(t, closeMsg.msgType, ServerStreamCloseMsgType)
	//assert.Equal(t, closeMsg.err, errors.New("close error"))
}
