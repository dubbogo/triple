package http2

import (
	"context"
	h2Triple "github.com/dubbogo/net/http2/triple"
	"net/http"
)

// ProtocolHeaderHandlerImpl is the triple imple of net.ProtocolHeaderHandler
// it handles the change of triple header field and h2 field
type ProtocolHeaderHandlerImpl struct {
	Ctx         context.Context
	reqFieldMap map[string]string
}

// NewProtocolHeaderHandlerImpl returns new TripleHeaderHandler
func NewProtocolHeaderHandlerImpl(reqFieldMap map[string]string) h2Triple.ProtocolHeaderHandler {
	return &ProtocolHeaderHandlerImpl{
		reqFieldMap: reqFieldMap,
	}
}

// WriteTripleReqHeaderField called before consumer calling remote,
// it parse field of opt and ctx to HTTP2 Header field, developer must assure "tri-" prefix field be string
// if not, it will cause panic!
func (t *ProtocolHeaderHandlerImpl) WriteTripleReqHeaderField(header http.Header) http.Header {
	for k, v := range t.reqFieldMap {
		header[k] = []string{v}
	}
	return header
}

// WriteTripleFinalRspHeaderField returns trailers header fields that triple and grpc defined
func (t *ProtocolHeaderHandlerImpl) WriteTripleFinalRspHeaderField(w http.ResponseWriter, grpcStatusCode int, grpcMessage string, traceProtoBin int) {
}

// ReadFromTripleReqHeader read meta header field from h2 header, and parse it to ProtocolHeader as developer defined
func (t *ProtocolHeaderHandlerImpl) ReadFromTripleReqHeader(r *http.Request) h2Triple.ProtocolHeader {
	return nil
}
