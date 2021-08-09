package main

import (
	"bytes"
	"fmt"
)

import (
	"github.com/dubbogo/triple/pkg/common/logger/default_logger"
	tconfig "github.com/dubbogo/triple/pkg/config"
	"github.com/dubbogo/triple/pkg/http2"
	"github.com/dubbogo/triple/pkg/http2/config"
)

const serverAddr = "localhost:1999"

func main() {
	testStream()
	testUnary()
}

func testStream() {
	client := http2.NewClient(tconfig.Option{Logger: default_logger.GetDefaultLogger()})
	header := make(map[string][]string)
	header["header1"] = []string{"header1-val"}
	header["header2"] = []string{"header2-val"}
	sendChan := make(chan *bytes.Buffer)
	dataChan, rspHeaderChan, err := client.StreamPost(serverAddr, "/stream", sendChan, &config.PostConfig{
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
	close(sendChan)
	rsp := <-dataChan
	fmt.Println("get rsp = ", rsp)
	trailer := <-rspHeaderChan
	fmt.Println("rspHeader = ", trailer)
}

func testUnary() {
	client := http2.NewClient(tconfig.Option{Logger: default_logger.GetDefaultLogger()})
	header := make(map[string][]string)
	header["header1"] = []string{"header1-val"}
	header["header2"] = []string{"header2-val"}
	data, rspHeader, err := client.Post(serverAddr, "/unary", []byte("hello"), &config.PostConfig{
		ContentType: "application",
		BufferSize:  4096,
		Timeout:     3,
		HeaderField: header,
	})

	fmt.Println("data = ", string(data))
	fmt.Println("rspHeader = ", rspHeader)
	fmt.Println("err = ", err)
}
