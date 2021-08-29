package http2

import (
	gxlog "github.com/dubbogo/gost/log"

	"github.com/dubbogo/net/http2"
)

func writeResponse(w *http2.Http2ResponseWriter, logger gxlog.Logger, code int, message string) {
	w.WriteHeader(code)
	if _, err := w.Write([]byte(message)); err != nil {
		logger.Errorf("write response failed, message: %s, err: %v\n", message, err)
	}
}
