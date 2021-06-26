package tests

import (
	"bytes"
	"fmt"
	"net/http"
	"testing"
)
import (
	"github.com/stretchr/testify/assert"
)
import (
	"github.com/dubbogo/triple/pkg/common/logger/default_logger"
	tconfig "github.com/dubbogo/triple/pkg/config"
	"github.com/dubbogo/triple/pkg/http2"
	"github.com/dubbogo/triple/pkg/http2/config"
)

func TestStream(t *testing.T) {
	client := http2.NewHttp2Client(tconfig.Option{Logger: default_logger.GetDefaultLogger()})
	header := make(http.Header)
	header["header1"] = []string{"header1-val"}
	header["header2"] = []string{"header2-val"}
	sendChan := make(chan *bytes.Buffer)
	dataChan, rspHeaderChan, err := client.StreamPost("localhost:1999", "/stream", sendChan, &config.PostConfig{
		ContentType: "application/grpc+proto",
		BufferSize:  4096,
		Timeout:     3,
		HeaderField: header,
	})
	assert.Nil(t, err)

	for i := 0; i < 10; i++ {
		sendChan <- bytes.NewBuffer([]byte("hello"))
		rsp := <-dataChan
		assert.Equal(t, "hello", string(rsp.Bytes()))
	}

	sendChan <- bytes.NewBuffer([]byte("hello Laurence"))
	close(sendChan)
	rsp := <-dataChan
	assert.Equal(t, "hello Laurence", string(rsp.Bytes()))
	<-rspHeaderChan
	// todo test header
}

func TestUnary(t *testing.T) {
	client := http2.NewHttp2Client(tconfig.Option{Logger: default_logger.GetDefaultLogger()})
	header := make(http.Header)
	header["header1"] = []string{"header1-val"}
	header["header2"] = []string{"header2-val"}
	data, rspHeader, err := client.Post("localhost:1999", "/unary", []byte("hello"), &config.PostConfig{
		ContentType: "application",
		BufferSize:  4096,
		Timeout:     3,
		HeaderField: header,
	})
	assert.Nil(t, err)
	assert.Equal(t, "hello", string(data))
	fmt.Println(rspHeader) // todo test
}
