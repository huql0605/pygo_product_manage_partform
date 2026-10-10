package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func Success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "ok",
		"data": data,
	})
}

func Fail(c *gin.Context, httpStatus, code int, msg string) {
	c.JSON(httpStatus, gin.H{
		"code": code,
		"msg":  msg,
		"data": nil,
	})
}

func BadRequest(c *gin.Context, code int, msg string) {
	Fail(c, http.StatusBadRequest, code, msg)
}

func NotFound(c *gin.Context, msg string) {
	Fail(c, http.StatusNotFound, 10002, msg)
}

func InternalError(c *gin.Context, msg string) {
	Fail(c, http.StatusInternalServerError, 10004, msg)
}

func BadGateway(c *gin.Context, code int, msg string) {
	Fail(c, http.StatusBadGateway, code, msg)
}
