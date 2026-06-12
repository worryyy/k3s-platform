package responses

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/worryyy/k3s-platform/platform/server/internal/pkg/bizerr"
)

type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

type successResponder struct{}

var Success successResponder

func (successResponder) RespData(c *gin.Context, data any) {
	write(c, responseStatus(c, http.StatusOK), "", data)
}

func (successResponder) RespMessage(c *gin.Context, message string) {
	write(c, responseStatus(c, http.StatusOK), message, nil)
}

func Fail(c *gin.Context, err error) {
	fail(c, err, "")
}

func FailMessage(c *gin.Context, err error, message string) {
	fail(c, err, message)
}

func fail(c *gin.Context, err error, message string) {
	status := http.StatusInternalServerError
	responseMessage := "internal server error"

	var businessError *bizerr.Error
	if errors.As(err, &businessError) {
		status = businessError.Code
		responseMessage = businessError.Message
	}
	if status == 0 {
		status = http.StatusInternalServerError
	}
	if responseMessage == "" {
		responseMessage = http.StatusText(status)
	}
	if message != "" {
		responseMessage = message
	}

	write(c, status, responseMessage, nil)
}

func write(c *gin.Context, status int, message string, data any) {
	if status == 0 {
		status = http.StatusInternalServerError
	}
	c.JSON(status, Response{
		Code:    status,
		Message: message,
		Data:    data,
	})
}

func responseStatus(c *gin.Context, defaultStatus int) int {
	status := c.Writer.Status()
	if status == 0 {
		return defaultStatus
	}
	return status
}
