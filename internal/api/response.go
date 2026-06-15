package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data:    data,
	})
}

func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, APIResponse{
		Code:    0,
		Message: "created",
		Data:    data,
	})
}

func BadRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, APIResponse{
		Code:    400,
		Message: msg,
	})
}

func NotFound(c *gin.Context, msg string) {
	c.JSON(http.StatusNotFound, APIResponse{
		Code:    404,
		Message: msg,
	})
}

func InternalError(c *gin.Context, msg string) {
	c.JSON(http.StatusInternalServerError, APIResponse{
		Code:    500,
		Message: msg,
	})
}

func Error(c *gin.Context, code int, msg string) {
	c.JSON(code, APIResponse{
		Code:    code,
		Message: msg,
	})
}
