package common

// PathExtractor extracts interface name from path
type PathExtractor interface {
	InterfaceName(string) (string, error)
}
