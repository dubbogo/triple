package common

// PathExtractor extracts interface name from path
type PathExtractor interface {
	InterfaceKey(string) (string, error)
}
