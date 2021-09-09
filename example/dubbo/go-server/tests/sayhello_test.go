package tests

import (
	"context"
	"testing"
)

import (
	"github.com/stretchr/testify/assert"
)

import (
	"github.com/dubbogo/triple/example/dubbo/proto"
)

func TestSayHello(t *testing.T) {
	ctx := context.Background()

	//ctx = context.WithValue(ctx, tripleConstant.TripleCtxKey(tripleConstant.TripleRequestID), "triple-request-id-demo")
	//ctx = context.WithValue(ctx, tripleConstant.TripleAttachement, "triple-request-id-demo")

	req := proto.HelloRequest{
		Name: "laurence",
	}
	user, err := greeterProvider.SayHello(ctx, &req)
	assert.Nil(t, err)

	assert.Equal(t, "laurence", user.Name)
	assert.Equal(t, "12345", user.Id)
	assert.Equal(t, int32(21), user.Age)

}
