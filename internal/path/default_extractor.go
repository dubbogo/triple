package path

import (
	"github.com/dubbogo/triple/internal/tools"
	"github.com/dubbogo/triple/pkg/common"
)

type DefaultExtractor struct{}

func (e *DefaultExtractor) HttpHandlerKey(path string) (string, error) {
	interfaceKey, _, err := tools.GetServiceKeyAndUpperCaseMethodNameFromPath(path)
	if err != nil {
		return "", err
	}
	return interfaceKey, nil
}

func NewDefaultExtractor() common.PathExtractor {
	return &DefaultExtractor{}
}
