module github.com/dubbogo/triple/example

go 1.13

require (
	dubbo.apache.org/dubbo-go/v3 v3.0.0-20210613014016-7fedc22e18a6
	github.com/dubbogo/triple v1.0.1-0.20210606070217-0815f3a0a013
	github.com/golang/protobuf v1.5.2
	github.com/stretchr/testify v1.7.0
	google.golang.org/grpc v1.38.0
)

replace github.com/dubbogo/triple => ../
