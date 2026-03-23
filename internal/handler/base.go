package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 统一API响应格式
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// SuccessResponse 返回成功响应
func SuccessResponse(data interface{}) Response {
	return Response{
		Code:    0,
		Message: "success",
		Data:    data,
	}
}

// ErrorResponse 返回错误响应
func ErrorResponse(code int, message string) Response {
	return Response{
		Code:    code,
		Message: message,
	}
}

// HandleSuccess 处理成功响应
func HandleSuccess(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, SuccessResponse(data))
}

// HandleError 处理错误响应
func HandleError(c *gin.Context, code int, err error) {
	c.JSON(http.StatusBadRequest, ErrorResponse(code, err.Error()))
}
