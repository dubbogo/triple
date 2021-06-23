package path_matcher

import (
	"github.com/dubbogo/triple/internal/tools"
	"github.com/dubbogo/triple/pkg/common"
)

type DefaultPathHandlerMatcher struct {
}

// Match match the given @interfaceName and @path read from http2 conn from java/go client
func (d *DefaultPathHandlerMatcher) Match(path string, interfaceName string) bool {
	interfaceKey, _, err := tools.GetServiceKeyAndUpperCaseMethodNameFromPath(path)
	if err != nil {
		return false
	}

	return interfaceName == interfaceKey
}

func NewDefaultPathHandlerMatcher() common.PathHandlerMatcher {
	return &DefaultPathHandlerMatcher{}
}
