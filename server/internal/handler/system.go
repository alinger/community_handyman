// Package handler 是 HTTP 层，只做参数绑定、调用 service、封装响应。
package handler

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/alinger/community-handyman/server/pkg/response"
)

// Health 供负载均衡与部署检查探活。
// @Summary      健康检查
// @Tags         运维
// @Produce      json
// @Success      200  {object}  response.Body
// @Router       /healthz [get]
func Health(c *gin.Context) {
	response.OK(c, gin.H{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	})
}
