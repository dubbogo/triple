package common

type PathHandlerMatcher interface {
	Match(path, rule string) bool
}
