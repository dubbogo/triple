package tests

import (
	"os"
	"testing"
	"time"
)

import (
	"dubbo.apache.org/dubbo-go/v3/config"
	_ "dubbo.apache.org/dubbo-go/v3/imports"
)

import (
	"github.com/dubbogo/triple/example/dubbo/proto"
)

var greeterProvider = new(proto.GreeterClientImpl)

func init() {
	config.SetConsumerService(greeterProvider)
}

// need to setup environment variable "CONF_CONSUMER_FILE_PATH" to "conf/client.yml" before run
func TestMain(m *testing.M) {
	config.Load()
	time.Sleep(time.Second * 3)

	os.Exit(m.Run())
}
