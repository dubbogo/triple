package common

import (
	"github.com/dubbogo/grpc-go/codes"
	"github.com/dubbogo/grpc-go/status"
)

type TripleError interface {
	Stacks() string
	Code() codes.Code
	Message() string
	GRPCStatus() *status.Status
}
