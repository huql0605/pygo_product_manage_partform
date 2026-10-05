package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/huql0605/go_api/internal/common/requestid"
)

const HeaderRequestID = "X-Request-ID"

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(HeaderRequestID)
		if id == "" {
			id = uuid.NewString()
		}

		// 1) 存进 gin 自己的 Keys，handler 里可以用 GetRequestID(c) 取。
		c.Set("request_id", id)
		// 2) 同时挂到标准 Go context 上，这样往后传 c.Request.Context()
		//    的任何代码（包括出站 HTTP client）都能拿到，用来做透传。
		c.Request = c.Request.WithContext(requestid.WithContext(c.Request.Context(), id))
		// 3) 回写响应头，客户端能直接看到。
		c.Header(HeaderRequestID, id)

		c.Next()
	}
}

func GetRequestID(c *gin.Context) string {
	if v, ok := c.Get("request_id"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return requestid.FromContext(c.Request.Context())
}
