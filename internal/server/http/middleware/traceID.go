package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const TraceIDKey = "X-Trace-ID"

func NewTraceID() gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := uuid.New().String()
		c.Set(TraceIDKey, traceID)
		c.Writer.Header().Set(TraceIDKey, traceID)
		c.Next()
	}
}
