package common

// PathExtractor extracts interface name from path
type PathExtractor interface {
	// HttpHandlerKey extracts key from the path for http handler
	HttpHandlerKey(string) (string, error)
}
