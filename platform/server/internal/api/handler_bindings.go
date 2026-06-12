package api

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/worryyy/k3s-platform/platform/server/internal/pkg/bizerr"
	"github.com/worryyy/k3s-platform/platform/server/internal/pkg/responses"
)

func bindJSON(c *gin.Context, req any) bool {
	if err := c.ShouldBindJSON(req); err != nil {
		responses.Fail(c, bizerr.ParamWrap("invalid request body", err))
		return false
	}
	return true
}

func requestLimit(c *gin.Context, defaultValue int) int {
	limit := defaultValue
	if raw := c.Query("limit"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value > 0 {
			limit = value
		}
	}
	return limit
}
