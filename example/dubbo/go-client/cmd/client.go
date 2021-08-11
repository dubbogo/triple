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
	"sync"
	"time"
)

import (
	_ "dubbo.apache.org/dubbo-go/v3/cluster/cluster_impl"
	_ "dubbo.apache.org/dubbo-go/v3/cluster/loadbalance"
	"dubbo.apache.org/dubbo-go/v3/common/logger"
	_ "dubbo.apache.org/dubbo-go/v3/common/proxy/proxy_factory"
	"dubbo.apache.org/dubbo-go/v3/config"
	_ "dubbo.apache.org/dubbo-go/v3/filter/filter_impl"
	_ "dubbo.apache.org/dubbo-go/v3/protocol/dubbo3"
	_ "dubbo.apache.org/dubbo-go/v3/protocol/grpc"
	_ "dubbo.apache.org/dubbo-go/v3/registry/protocol"
	_ "dubbo.apache.org/dubbo-go/v3/registry/zookeeper"
	"go.uber.org/atomic"
)

import (
	"github.com/dubbogo/triple/example/dubbo/go-client/pkg"
	tripleConstant "github.com/dubbogo/triple/pkg/common/constant"
)

var greeterProvider = new(pkg.GreeterClientImpl)

func init() {
	config.SetConsumerService(greeterProvider)
}

// need to setup environment variable "CONF_CONSUMER_FILE_PATH" to "conf/client.yml" before run
func main() {
	config.Load()
	time.Sleep(time.Second * 3)
	testSayHello()

	// stream is not available for dubbo-java
	//testSayHelloStream()
}

func testSayHello() {
	logger.Infof("testSayHello")

	ctx := context.Background()
	ctx = context.WithValue(ctx, tripleConstant.TripleCtxKey(tripleConstant.TripleRequestID), "triple-request-id-demo")

	req := pkg.HelloRequest{
		Name: "laurence",
	}
	user := pkg.User{}
	err := greeterProvider.SayHello(ctx, &req, &user)
	if err != nil {
		panic(err)
	}
}


func testSayHelloWithHighParallel() {
	logger.Infof("testSayHello")

	ctx := context.Background()
	ctx = context.WithValue(ctx, tripleConstant.TripleCtxKey(tripleConstant.TripleRequestID), "triple-request-id-demo")

	req := pkg.HelloRequest{
		Name: "laurence",
	}
	user := pkg.User{}
	for {
		wg := sync.WaitGroup{}
		goodCounter := atomic.Uint32{}
		badCounter := atomic.Uint32{}
		for i := 0; i < 1000; i ++{
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := greeterProvider.SayHello(ctx, &req, &user)
				if err != nil {
					badCounter.Inc()
					logger.Error(err)
					return
				}
				goodCounter.Inc()
				logger.Infof("Receive user = %+v\n", user)
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

	req := pkg.HelloRequest{
		Name: "laurence",
	}

	client, err := greeterProvider.SayHelloStream(ctx)
	if err != nil {
		panic(err)
	}

	var user *pkg.User
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
