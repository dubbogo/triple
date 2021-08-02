package config

import (
	"net/http"
)

type PostConfig struct {
	ContentType string
	BufferSize  uint32
	Timeout     uint32
	HeaderField http.Header
}
