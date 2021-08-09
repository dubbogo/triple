package main

import (
	"bytes"
	"fmt"
	"net/http"
	"runtime"
	"time"
)

import (
	"github.com/dubbogo/triple/pkg/common/logger/default_logger"
	"github.com/dubbogo/triple/pkg/http2"
	"github.com/dubbogo/triple/pkg/http2/config"
)

func main() {
	svr := http2.NewServer("localhost:1999", config.ServerConfig{
		Logger: default_logger.GetDefaultLogger(),
		NumWorkers: runtime.NumCPU(),
	})
	svr.RegisterHandler("/unary", func(path string, header http.Header, recvChan chan *bytes.Buffer, sendChan chan *bytes.Buffer, ctrlCh chan http.Header, errCh chan interface{}) {
		fmt.Println("path = ", path)
		fmt.Println("header = ", header)

		rspHeader := make(map[string][]string)
		rspHeader["content-type"] = []string{"application/grpc+proto"}
		rspHeader["grpc-status"] = []string{"Trailer"}
		rspHeader["grpc-message"] = []string{"Trailer"}
		rspHeader["tri-bin"] = []string{"Trailer"}
		ctrlCh <- rspHeader

		body := <-recvChan
		fmt.Println(body)
		sendChan <- body
		close(sendChan)
		time.Sleep(time.Second)

		rspHeader2 := make(map[string][]string)
		rspHeader2["grpc-status"] = []string{"grpc-status-val"}
		rspHeader2["grpc-message"] = []string{"grpc-message-val"}
		rspHeader2["tri-bin"] = []string{"tri-bin-val"}
		ctrlCh <- rspHeader2
	})

	svr.RegisterHandler("/stream", func(path string, header http.Header, recvChan chan *bytes.Buffer, sendChan chan *bytes.Buffer, ctrlCh chan http.Header, errCh chan interface{}) {
		fmt.Println("path = ", path)
		fmt.Println("header = ", header)

		rspHeader := make(map[string][]string)
		rspHeader["content-type"] = []string{"application/grpc+proto"}
		rspHeader["grpc-status"] = []string{"Trailer"}
		rspHeader["grpc-message"] = []string{"Trailer"}
		rspHeader["tri-bin"] = []string{"Trailer"}
		ctrlCh <- rspHeader

		body := bytes.NewBuffer([]byte("hello"))
		for string(body.Bytes()) == "hello" {
			body = <-recvChan
			fmt.Println(body)
			sendChan <- body
		}
		close(sendChan)
		time.Sleep(time.Second)

		rspHeader2 := make(map[string][]string)
		rspHeader2["grpc-status"] = []string{"grpc-status-val"}
		rspHeader2["grpc-message"] = []string{"grpc-message-val"}
		rspHeader2["tri-bin"] = []string{"tri-bin-val"}
		ctrlCh <- rspHeader2
	})
	svr.Start()
	select {}
}
