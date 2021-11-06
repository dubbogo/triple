all: vet test testrace

build:
	go build github.com/dubbogo/triple/pkg/grpc/...

clean:
	go clean -i github.com/dubbogo/triple/pkg/grpc/...

deps:
	GO111MODULE=on go get -d -v github.com/dubbogo/triple/pkg/grpc/...

proto:
	@ if ! which protoc > /dev/null; then \
		echo "error: protoc not installed" >&2; \
		exit 1; \
	fi
	go generate github.com/dubbogo/triple/pkg/grpc/...

test:
	go test -cpu 1,4 -timeout 7m github.com/dubbogo/triple/pkg/grpc/...

testsubmodule:
	cd security/advancedtls && go test -cpu 1,4 -timeout 7m github.com/dubbogo/triple/pkg/grpc/security/advancedtls/...
	cd security/authorization && go test -cpu 1,4 -timeout 7m github.com/dubbogo/triple/pkg/grpc/security/authorization/...

testrace:
	go test -race -cpu 1,4 -timeout 7m github.com/dubbogo/triple/pkg/grpc/...

testdeps:
	GO111MODULE=on go get -d -v -t github.com/dubbogo/triple/pkg/grpc/...

vet: vetdeps
	./vet.sh

vetdeps:
	./vet.sh -install

.PHONY: \
	all \
	build \
	clean \
	proto \
	test \
	testrace \
	vet \
	vetdeps
