package config

import (
	"github.com/dubbogo/triple/pkg/common"
	"github.com/dubbogo/triple/pkg/common/logger"
)

type ServerConfig struct {
	Logger             logger.Logger
	PathHandlerMatcher common.PathHandlerMatcher
}
