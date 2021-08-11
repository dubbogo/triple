package common

// Codec is used to marshal interface to bytes and unmarshal bytes to interface.
// It is not used directly by triple network, instead, it used by TwoWayCodec, and TwoWayCodec is
// directly used by triple processor/h2Controller
// Codec is used to marshal interface to bytes and unmarshal bytes to interface.
// It is not used directly by triple network, instead, it used by TwoWayCodec, and TwoWayCodec is
// directly used by triple processor/h2Controller
// Codec is used to marshal interface to bytes and unmarshal bytes to interface.
// It is not used directly by triple network, instead, it used by TwoWayCodec, and TwoWayCodec is
// directly used by triple processor/h2Controller
// PathExtractor extracts interface name from path
type PathExtractor interface {
	// HttpHandlerKey extracts key from the path for http handler
	HttpHandlerKey(string) (string, error)
}
