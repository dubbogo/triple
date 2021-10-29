module github.com/dubbogo/triple/example

go 1.13

require (
	dubbo.apache.org/dubbo-go/v3 v3.0.0-rc2.0.20211029104429-bfc60c2496f9
	github.com/dubbogo/triple v1.0.9-0.20211029101533-26805fcfcf62
	github.com/golang/protobuf v1.5.2
	github.com/pkg/errors v0.9.1
	github.com/stretchr/objx v0.2.0 // indirect
	github.com/stretchr/testify v1.7.0
	go.uber.org/atomic v1.7.0
	google.golang.org/grpc v1.41.0
	google.golang.org/protobuf v1.27.1
)

replace github.com/dubbogo/triple => ../
