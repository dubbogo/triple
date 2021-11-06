module github.com/dubbogo/triple/pkg/grpc/security/advancedtls/examples

go 1.15

require (
	github.com/dubbogo/triple/pkg/grpc v1.38.0
	github.com/dubbogo/triple/pkg/grpc/examples v0.0.0-20201112215255-90f1b3ee835b
	github.com/dubbogo/triple/pkg/grpc/security/advancedtls v0.0.0-20201112215255-90f1b3ee835b
)

replace github.com/dubbogo/triple/pkg/grpc => ../../..

replace github.com/dubbogo/triple/pkg/grpc/examples => ../../../examples

replace github.com/dubbogo/triple/pkg/grpc/security/advancedtls => ../
