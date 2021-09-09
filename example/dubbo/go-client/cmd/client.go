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

package main

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"runtime"
	"sync"
	"time"
)

import (
	"dubbo.apache.org/dubbo-go/v3/common/logger"
	"dubbo.apache.org/dubbo-go/v3/config"
	_ "dubbo.apache.org/dubbo-go/v3/imports"

	"go.uber.org/atomic"
)

import (
	"github.com/dubbogo/triple/example/dubbo/proto"
	tripleConstant "github.com/dubbogo/triple/pkg/common/constant"
)

var greeterProvider = new(proto.GreeterClientImpl)

func init() {
	config.SetConsumerService(greeterProvider)
	runtime.SetMutexProfileFraction(1)
}
// need to setup environment variable "CONF_CONSUMER_FILE_PATH" to "conf/client.yml" before run
func main() {
	go func() {
		_ = http.ListenAndServe("0.0.0.0:6061", nil)
	}()
	config.Load()
	time.Sleep(time.Second * 3)
	testSayHello()

	// stream is not available for dubbo-java
	//testSayHelloStream()

	// high parallel and gr pool limitaion
	//testSayHelloWithHighParallel()
}

func testSayHello() {
	logger.Infof("testSayHello")

	ctx := context.Background()
	ctx = context.WithValue(ctx, tripleConstant.TripleCtxKey(tripleConstant.TripleRequestID), "triple-request-id-demo")

	req := proto.HelloRequest{
		Name: "laurence",
	}
	user, err := greeterProvider.SayHello(ctx, &req)
	if err != nil {
		panic(err)
	}
	logger.Infof("get response user = %+v\n", user)
}


func testSayHelloWithHighParallel() {
	logger.Infof("testSayHello")

	ctx := context.Background()
	ctx = context.WithValue(ctx, tripleConstant.TripleCtxKey(tripleConstant.TripleRequestID), "triple-request-id-demo")

	req := proto.HelloRequest{
		Name: "laurence",
	}
	for {
		wg := sync.WaitGroup{}
		goodCounter := atomic.Uint32{}
		badCounter := atomic.Uint32{}
		for i := 0; i < 2000; i ++{
			wg.Add(1)
			go func() {
				defer wg.Done()
				usr, err := greeterProvider.SayHello(ctx, &req)
				if err != nil {
					badCounter.Inc()
					logger.Error(err)
					return
				}
				goodCounter.Inc()
				logger.Infof("Receive user = %+v\n", usr)
			}()
		}
		wg.Wait()
		fmt.Println("goodCounter = ", goodCounter.Load())
		fmt.Println("badCounter = ", badCounter.Load())
		time.Sleep(time.Second*5)
	}
}

func testSayHelloStream() {
	logger.Infof("testSayHelloStream")

	ctx := context.Background()
	ctx = context.WithValue(ctx, tripleConstant.TripleCtxKey(tripleConstant.TripleRequestID), "triple-request-id-demo")

	req := proto.HelloRequest{
		Name: "laurence",
	}

	client, err := greeterProvider.SayHelloStream(ctx)
	if err != nil {
		panic(err)
	}

	var user *proto.User
	err = client.Send(&req)
	if err != nil {
		panic(err)
	}

	user, err = client.Recv()
	if err != nil {
		panic(err)
	}
	logger.Infof("Receive user = %+v\n", *user)
}
