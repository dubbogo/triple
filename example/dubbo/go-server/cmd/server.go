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
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

import (
	"dubbo.apache.org/dubbo-go/v3/common/logger"
	"dubbo.apache.org/dubbo-go/v3/config"
	_ "dubbo.apache.org/dubbo-go/v3/imports"

	perrors "github.com/pkg/errors"
)

import (
	"github.com/dubbogo/triple/example/dubbo/proto"
	tripleConstant "github.com/dubbogo/triple/pkg/common/constant"
	_ "github.com/dubbogo/triple/pkg/triple"
)

var (
	survivalTimeout = int(3 * time.Second)
)

func init() {
	runtime.SetMutexProfileFraction(1)
}

// need to setup environment variable "CONF_PROVIDER_FILE_PATH" to "conf/server.yml" before run
func main() {
	config.SetProviderService(&GreeterProvider{})
	go func() {
		_ = http.ListenAndServe("0.0.0.0:6060", nil)
	}()
	config.Load()
	initSignal()
}

func initSignal() {
	signals := make(chan os.Signal, 1)
	// It is not possible to block SIGKILL or syscall.SIGSTOP
	signal.Notify(signals, os.Interrupt, os.Kill, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)
	for {
		sig := <-signals
		logger.Infof("get signal %s", sig.String())
		switch sig {
		case syscall.SIGHUP:
			// reload()
		default:
			time.Sleep(time.Second * 5)
			time.AfterFunc(time.Duration(survivalTimeout), func() {
				logger.Warnf("app exit now by force...")
				os.Exit(1)
			})

			// The program exits normally or timeout forcibly exits.
			fmt.Println("provider app exit now...")
			return
		}
	}
}



type GreeterProvider struct {
	proto.GreeterProviderBase
}

func (s *GreeterProvider) SayHelloStream(svr proto.Greeter_SayHelloStreamServer) error {
	c, err := svr.Recv()
	if err != nil {
		return err
	}
	logger.Infof("Dubbo-go3 GreeterProvider recv 1 user, name = %s\n", c.Name)

	err = svr.Send(&proto.User{
		Name: "hello " + c.Name,
		Age:  18,
		Id:   "123456789",
	})

	if err != nil {
		return err
	}
	return nil
}

func (s *GreeterProvider) SayHello(ctx context.Context, in *proto.HelloRequest) (*proto.User, error) {
	logger.Infof("Dubbo3 GreeterProvider get user name = %s\n" + in.Name)
	fmt.Println("get triple header tri-req-id = ", ctx.Value(tripleConstant.TripleCtxKey(tripleConstant.TripleRequestID)))
	time.Sleep(time.Second * 1)
	fmt.Println("get triple header tri-service-version = ", ctx.Value(tripleConstant.TripleCtxKey(tripleConstant.TripleServiceVersion)))
	return &proto.User{Name: in.Name, Id: "12345", Age: 21}, nil
}

func (s *GreeterProvider) SayHelloWithError(ctx context.Context, in *proto.HelloRequest) (*proto.User, error) {
	logger.Infof("Dubbo3 GreeterProvider get user name = %s\n" + in.Name)
	fmt.Println("get triple header tri-req-id = ", ctx.Value(tripleConstant.TripleCtxKey(tripleConstant.TripleRequestID)))
	time.Sleep(time.Second * 1)
	err := perrors.New("user defined error")
	fmt.Println("get triple header tri-service-version = ", ctx.Value(tripleConstant.TripleCtxKey(tripleConstant.TripleServiceVersion)))
	return &proto.User{Name: in.Name, Id: "12345", Age: 21}, err
}
