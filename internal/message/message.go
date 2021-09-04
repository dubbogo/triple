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

package message

import (
	"bytes"
)

import (
	"github.com/dubbogo/triple/internal/status"
	"github.com/dubbogo/triple/pkg/common"
)

////////////////////////////////Buffer and MsgType
// Message is the basic transfer unit in one stream
type Message struct {
	Buffer     *bytes.Buffer
	MsgType    MsgType
	Status     *status.Status
	Err        error // todo delete it, all change to status
	Attachment common.TripleAttachment
}

func (bm *Message) Read(p []byte) (int, error) {
	return bm.Buffer.Read(p)
}

func (bm *Message) Bytes() []byte {
	return bm.Buffer.Bytes()
}

func (bm *Message) Write(data []byte) {
	bm.Buffer.Write(data)
}

func (bm *Message) Reset() {
	bm.Buffer.Reset()
}

func (bm *Message) Len() int {
	return bm.Buffer.Len()
}

// GetMsgType can get message's type
func (bm *Message) GetMsgType() MsgType {
	return bm.MsgType
}

// MsgQueue contain the chan of Message
type MsgQueue struct {
	c chan Message
}

// NewBufferMsgQueue returns new MsgQueue
func NewBufferMsgQueue() *MsgQueue {
	b := &MsgQueue{
		c: make(chan Message),
	}
	return b
}

// Put if stream close by force, the Put function doesn't send anything.
func (b *MsgQueue) Put(r Message) {
	if b.c != nil {
		b.c <- r
	}
}

func (b *MsgQueue) Get() <-chan Message {
	return b.c
}

func (b *MsgQueue) Close() {
	close(b.c)
}

/////////////////////////////////stream state
