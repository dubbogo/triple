module github.com/dubbogo/triple/example

go 1.13

require (
	dubbo.apache.org/dubbo-go/v3 v3.0.0-rc2
	github.com/dubbogo/gost v1.11.15
	github.com/dubbogo/triple v1.0.1
	github.com/golang/protobuf v1.5.2
	github.com/stretchr/testify v1.7.0
	google.golang.org/grpc v1.38.0
)

replace github.com/dubbogo/triple => ../
