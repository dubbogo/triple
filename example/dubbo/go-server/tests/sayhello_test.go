package tests

import (
	"context"
	"testing"
)

import (
	"github.com/stretchr/testify/assert"
)
import (
	client "github.com/dubbogo/triple/example/dubbo/go-client/pkg"
	tripleConstant "github.com/dubbogo/triple/pkg/common/constant"
)

func TestSayHello(t *testing.T) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, tripleConstant.TripleCtxKey(tripleConstant.TripleRequestID), "triple-request-id-demo")

	req := client.HelloRequest{
		Name: "laurence",
	}
	user := client.User{}
	err := greeterProvider.SayHello(ctx, &req, &user)
	assert.Nil(t, err)

	assert.Equal(t, "laurence", user.Name)
	assert.Equal(t, "12345", user.Id)
	assert.Equal(t, int32(21), user.Age)

}
