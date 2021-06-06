module github.com/dubbogo/triple/test/integrate/dubbo/go-client

go 1.13

require (
	github.com/apache/dubbo-go v1.5.6-rc2.0.20210423025830-279ccd2e567c
	github.com/dubbogo/gost v1.11.7 // indirect
	github.com/dubbogo/triple v1.0.0
	github.com/golang/protobuf v1.5.2
	github.com/pelletier/go-toml v1.4.0 // indirect
	google.golang.org/grpc v1.36.0
	honnef.co/go/tools v0.0.1-2020.1.3 // indirect
)

replace (
	github.com/dubbogo/gost => github.com/dubbogo/gost v1.11.7
	github.com/dubbogo/triple => ../../../../../triple
	github.com/envoyproxy/go-control-plane => github.com/envoyproxy/go-control-plane v0.8.0
	github.com/nacos-group/nacos-sdk-go => github.com/nacos-group/nacos-sdk-go v1.0.7-0.20210325111144-d75caca21a46
	github.com/shirou/gopsutil => github.com/shirou/gopsutil v0.0.0-20181107111621-48177ef5f880
)
