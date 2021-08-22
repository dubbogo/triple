package config

import (
	"github.com/dubbogo/triple/pkg/common"
	"github.com/dubbogo/triple/pkg/common/logger"
)

type ServerConfig struct {
	// Logger is User defined logger, if empty, use default.
	Logger logger.Logger

	// PathExtractor extracts interface name from path, if empty, use default
	PathExtractor common.PathExtractor

	/*
		HandlerGRManagedByUser is the flag that let user control his own gr in http2's Handler, default is false
		if HandlerGRManagedByUser is false:
		Here is an example:
			```go
			svr := http2.NewServer("localhost:1999", config.ServerConfig{
					Logger: default_logger.GetDefaultLogger(),
				})
				svr.RegisterHandler("/unary", func(path string, header http.Header, recvChan chan *bytes.Buffer,
					sendChan chan *bytes.Buffer, ctrlCh chan http.Header, errCh chan interface{}) {
					//your handler code
				})
			```
		if HandlerGRManagedByUser is true:
		Here is an example:
			```go
			svr := http2.NewServer("localhost:1999", config.ServerConfig{
					Logger: default_logger.GetDefaultLogger(),
					HandlerGRManagedByUser: true,
				})
				svr.RegisterHandler("/unary", func(path string, header http.Header, recvChan chan *bytes.Buffer,
					sendChan chan *bytes.Buffer, ctrlCh chan http.Header, errCh chan interface{}) {
					// you should start a gr here
					go func() {
						//your handler code
					}()
				})
			```
		Another example is with gr pool limitation scene:
			```go
			func(path string, header http.Header, recvChan chan *bytes.Buffer,
				sendChan chan *bytes.Buffer, ctrlch chan http.Header,
				errCh chan interface{}) {
				var (
					tripleStatus *status.Status
				)
				// Let user manage gr
				if err := hc.pool.Submit(func() {
					// handler codes
				}); err != nil {
					// failed to occupy worker, return error code
					go func() {
						rspHeader := make(map[string][]string)
						rspHeader["content-type"] = []string{constant.TripleContentType}
						rspHeader[constant.TrailerKeyGrpcStatus] = []string{constant.TrailerKey}
						rspHeader[constant.TrailerKeyGrpcMessage] = []string{constant.TrailerKey}
						rspHeader[constant.TrailerKeyTraceProtoBin] = []string{constant.TrailerKey}
						rspHeader[constant.TrailerKeyGrpcDetailsBin] = []string{constant.TrailerKey}
						ctrlch <- rspHeader
						close(sendChan)
						hc.option.Logger.Warnf("go routine pool full with error = %v", err)
						hc.handleStatusAndResponse(status.New(codes.ResourceExhausted, fmt.Sprintf("go routine pool full with error = %v", err)), ctrlch)
					}()
				}
			```
	*/
	HandlerGRManagedByUser bool
}
