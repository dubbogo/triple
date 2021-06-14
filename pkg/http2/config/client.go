package config

type PostConfig struct {
	ContentType string
	BufferSize  uint32
	Timeout     uint32
	HeaderField map[string]string
}
