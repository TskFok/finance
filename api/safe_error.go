package api

import (
	"finance/config"
)

// SafeErrorMessage 生产环境下不向客户端暴露内部错误详情，避免信息泄露
func SafeErrorMessage(err error, fallback string) string {
	return config.SafeErrorMessage(err, fallback)
}
