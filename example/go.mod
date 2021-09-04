module github.com/dubbogo/triple/example

go 1.13

require (
	dubbo.apache.org/dubbo-go/v3 v3.0.0-rc2.0.20210904062533-3e0412eab250
	github.com/dubbogo/triple v1.0.6-0.20210904050749-5721796f3fd6
	github.com/golang/protobuf v1.5.2
	github.com/stretchr/objx v0.2.0 // indirect
	github.com/stretchr/testify v1.7.0
	go.uber.org/atomic v1.7.0
	google.golang.org/grpc v1.38.0
)

replace github.com/dubbogo/triple => ../
