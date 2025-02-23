module github.com/dubbogo/triple/example

go 1.13

require (
	dubbo.apache.org/dubbo-go/v3 v3.0.0-rc2.0.20211029104429-bfc60c2496f9
	github.com/dubbogo/triple v1.0.9
	github.com/golang/protobuf v1.5.2
	github.com/pkg/errors v0.9.1
	github.com/stretchr/testify v1.8.2
	go.uber.org/atomic v1.9.0
	google.golang.org/grpc v1.41.0
	google.golang.org/protobuf v1.27.1
)

replace github.com/dubbogo/triple => ../
