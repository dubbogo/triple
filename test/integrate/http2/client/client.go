package main

import (
	"bytes"
	"fmt"
	"github.com/dubbogo/triple/pkg/common/logger/default_logger"
	"github.com/dubbogo/triple/pkg/http2"
	"github.com/dubbogo/triple/pkg/http2/config"

	tconfig "github.com/dubbogo/triple/pkg/config"
)

func main() {
	testStream()
	testUnary()
}

func testStream() {
	client := http2.NewHttp2Client(tconfig.Option{Logger: default_logger.GetDefaultLogger()})
	header := make(map[string]string)
	header["header1"] = "header1-val"
	header["header2"] = "header2-val"
	sendChan := make(chan *bytes.Buffer)
	dataChan, rspHeaderChan, err := client.StreamPost("localhost:1999", "/stream", sendChan, &config.PostConfig{
		ContentType: "application/grpc+proto",
		BufferSize:  4096,
		Timeout:     3,
		HeaderField: header,
	})
	if err != nil {
		panic(err)
	}

	for i := 0; i < 10; i++ {
		sendChan <- bytes.NewBuffer([]byte("hello"))
		rsp := <-dataChan
		fmt.Println("get rsp = ", rsp)
	}

	sendChan <- bytes.NewBuffer([]byte("hello Laurence"))
	rsp := <-dataChan
	fmt.Println("get rsp = ", rsp)
	trailer := <-rspHeaderChan
	fmt.Println("rspHeader = ", trailer)
}

func testUnary() {
	client := http2.NewHttp2Client(tconfig.Option{Logger: default_logger.GetDefaultLogger()})
	header := make(map[string]string)
	header["header1"] = "header1-val"
	header["header2"] = "header2-val"
	data, rspHeader, err := client.Post("localhost:1999", "/unary", []byte("hello"), &config.PostConfig{
		ContentType: "application",
		BufferSize:  4096,
		Timeout:     3,
		HeaderField: header,
	})

	fmt.Println("data = ", string(data))
	fmt.Println("rspHeader = ", rspHeader)
	fmt.Println("err = ", err)
}
