package main

import (
	"bytes"
	"fmt"
	"github.com/dubbogo/triple/pkg/common/logger/default_logger"
	"github.com/dubbogo/triple/pkg/http2"
	"github.com/dubbogo/triple/pkg/http2/config"
	"time"
)

func main() {
	svr := http2.NewHttp2Server("localhost:1999", config.ServerConfig{
		Logger: default_logger.GetDefaultLogger(),
	})
	svr.RegisterHandler("/unary", func(path string, header map[string]string, recvChan chan *bytes.Buffer, sendChan chan *bytes.Buffer, ctrlCh chan map[string]string) {
		fmt.Println("path = ", path)
		fmt.Println("header = ", header)

		rspHeader := make(map[string]string)
		rspHeader["content-type"] = "application/grpc+proto"
		rspHeader["grpc-status"] = "Trailer"
		rspHeader["grpc-message"] = "Trailer"
		rspHeader["tri-bin"] = "Trailer"
		ctrlCh <- rspHeader

		body := <-recvChan
		fmt.Println(body)
		sendChan <- body
		close(sendChan)
		time.Sleep(time.Second)

		rspHeader2 := make(map[string]string)
		rspHeader2["grpc-status"] = "grpc-status-val"
		rspHeader2["grpc-message"] = "grpc-message-val"
		rspHeader2["tri-bin"] = "tri-bin-val"
		ctrlCh <- rspHeader2
	})

	svr.RegisterHandler("/stream", func(path string, header map[string]string, recvChan chan *bytes.Buffer, sendChan chan *bytes.Buffer, ctrlCh chan map[string]string) {
		fmt.Println("path = ", path)
		fmt.Println("header = ", header)

		rspHeader := make(map[string]string)
		rspHeader["content-type"] = "application/grpc+proto"
		rspHeader["grpc-status"] = "Trailer"
		rspHeader["grpc-message"] = "Trailer"
		rspHeader["tri-bin"] = "Trailer"
		ctrlCh <- rspHeader

		body := bytes.NewBuffer([]byte("hello"))
		for string(body.Bytes()) == "hello" {
			body = <-recvChan
			fmt.Println(body)
			sendChan <- body
		}
		close(sendChan)
		time.Sleep(time.Second)

		rspHeader2 := make(map[string]string)
		rspHeader2["grpc-status"] = "grpc-status-val"
		rspHeader2["grpc-message"] = "grpc-message-val"
		rspHeader2["tri-bin"] = "tri-bin-val"
		ctrlCh <- rspHeader2
	})
	svr.Start()
	select {}
}
