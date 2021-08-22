package http2

import (
	"fmt"
)

import (
	gxlog "github.com/dubbogo/gost/log"

	"github.com/dubbogo/net/http2"

	perrors "github.com/pkg/errors"
)

func writeResponse(w *http2.Http2ResponseWriter, logger gxlog.Logger, code int, message interface{}) {
	bytes, ok := message.([]byte)
	if !ok {
		logger.Error(perrors.New(fmt.Sprintf("unexpected message type: %T", message)))
	}
	w.WriteHeader(code)
	if _, err := w.Write(bytes); err != nil {
		logger.Errorf("write response failed, message: %s, err: %v\n", string(bytes), err)
	}
}
