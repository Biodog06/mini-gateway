package middleware

import (
	"fmt"
	"time"

	"go.opentelemetry.io/otel/codes"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Tracing 返回分布式追踪中间件
func Tracing() gin.HandlerFunc {
	return func(c *gin.Context) {
		tracer := otel.Tracer("mini-gateway")

		// 1. 提取上下文
		ctx := otel.GetTextMapPropagator().Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))

		// 2. 开始 Span，初始名字用 Method，因为 Path 还没匹配
		ctx, span := tracer.Start(ctx, fmt.Sprintf("%s %s", c.Request.Method, c.Request.URL.Path),
			trace.WithAttributes(
				attribute.String("http.method", c.Request.Method),
				attribute.String("http.host", c.Request.Host),
				attribute.String("http.url", c.Request.URL.String()),
				attribute.String("http.user_agent", c.Request.UserAgent()),
				attribute.String("http.client_ip", c.ClientIP()),
			),
			trace.WithSpanKind(trace.SpanKindServer),
		)
		defer span.End()

		// 3. 注入 Context 到 Gin
		c.Request = c.Request.WithContext(ctx)

		// 记录 TraceID
		spanCtx := span.SpanContext()
		if spanCtx.HasTraceID() {
			traceID := spanCtx.TraceID().String()
			c.Set("trace_id", traceID)
			// 在 Response Header 中返回 TraceID，方便调试
			c.Writer.Header().Set("X-Trace-Id", traceID)
		}

		start := time.Now()

		// 4. 处理请求
		c.Next()

		// 5. 请求后处理：更新 Span 名称和状态

		// 使用路由模式作为 Span 名称，避免高基数
		if routePath := c.Request.URL.Path; routePath != "" {
			span.SetName(fmt.Sprintf("%s %s", c.Request.Method, routePath))
			span.SetAttributes(attribute.String("http.route", routePath))
		}

		status := c.Writer.Status()
		duration := time.Since(start).Seconds()

		span.SetAttributes(
			attribute.Int("http.status_code", status),
			attribute.Float64("http.duration_seconds", duration),
		)

		// 处理错误状态
		if status >= 500 {
			span.SetStatus(codes.Error, fmt.Sprintf("HTTP Status %d", status))
		} else if len(c.Errors) > 0 {
			span.SetStatus(codes.Error, c.Errors.String())
		} else {
			span.SetStatus(codes.Ok, "OK")
		}

		// 记录显式的 Gin 错误
		for _, e := range c.Errors {
			span.RecordError(e)
		}
	}
}
