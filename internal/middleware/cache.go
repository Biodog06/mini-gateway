package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/DoraZa/mini-gateway/config"
	"github.com/DoraZa/mini-gateway/internal/core/health" // 引入 health 包
	"github.com/DoraZa/mini-gateway/internal/core/observability"
	"github.com/DoraZa/mini-gateway/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type CachedResponse struct {
	StatusCode int                 `json:"status"`
	Headers    map[string][]string `json:"headers"`
	Body       string              `json:"body"`
}

// 辅助函数（可用）：排序参数，生成规范化的 CacheKey
func generateCacheKey(c *gin.Context) string {
	// 获取所有参数
	params := c.Request.URL.Query()
	// 这里的 Encode 方法默认会按 Key 字母顺序排序
	sortedQuery := params.Encode()

	if sortedQuery == "" {
		return c.Request.URL.Path
	}
	return c.Request.URL.Path + "?" + sortedQuery
}

func CacheMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !config.GetConfig().Caching.Enabled {
			c.Next()
			return
		}

		path := c.Request.URL.Path
		// RequestURI 包含了 Query String
		requestURI := generateCacheKey(c)
		method := c.Request.Method
		rule := config.GetConfig().GetCacheRuleByPath(path)

		if rule == nil || rule.Method != method {
			c.Next()
			return
		}

		// [修改点]：强制只处理 GET 请求
		// 如果不是 GET 请求，直接放行，不走缓存逻辑
		// -------------------------------------------------------
		if c.Request.Method != http.MethodGet {
			c.Next()
			return
		}

		// 获取目标主机（假设从路由规则中提取第一个目标）
		target := ""
		if rules, ok := config.GetConfig().Routing.Rules[path]; ok && len(rules) > 0 {
			host, err := health.NormalizeTarget(rules[0])
			if err == nil {
				target = host
			}
		}

		// 增加请求计数并检查阈值
		count := health.GetGlobalHealthChecker().IncrementRequestCount(c.Request.Context(), path, rule.TTL)
		logger.Debug("Request count", zap.String("path", path), zap.Int64("count", count))

		// 检查缓存
		if content, found := health.GetGlobalHealthChecker().CheckCache(c.Request.Context(), method, requestURI, target); found {
			var cachedResp CachedResponse
			if err := json.Unmarshal([]byte(content), &cachedResp); err == nil {
				for k, values := range cachedResp.Headers {
					for _, value := range values {
						c.Writer.Header().Add(k, value)
					}
				}
				observability.CacheHits.WithLabelValues(method, path, target).Inc()
				c.Data(http.StatusOK, c.Writer.Header().Get("Content-Type"), []byte(cachedResp.Body))
				c.Abort()
				return
			}
			// 如果反序列化失败，说明缓存数据坏了，当作未命中处理，继续向下执行
			logger.Warn("Failed to unmarshal cached response", zap.String("key", requestURI))
		}

		if count < int64(rule.Threshold) {
			c.Next()
			return
		}

		// 捕获响应并缓存
		writer := &responseWriter{ResponseWriter: c.Writer}
		c.Writer = writer
		c.Next()

		observability.CacheMisses.WithLabelValues(method, path, target).Inc()
		if c.Writer.Status() == http.StatusOK {
			cacheResp := CachedResponse{
				StatusCode: c.Writer.Status(),
				Headers:    c.Writer.Header(),
				Body:       writer.body.String(),
			}
			jsonData, err := json.Marshal(cacheResp)
			if err == nil {
				err := health.GetGlobalHealthChecker().SetCache(c.Request.Context(), method, requestURI, string(jsonData), rule.TTL)
				if err != nil {
					logger.Error("Failed to cache response", zap.Error(err))
				}
			} else {
				logger.Error("Failed to marshal cache response", zap.Error(err))
			}
		}
	}
}

// responseWriter 用于捕获响应内容
type responseWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *responseWriter) Write(b []byte) (int, error) {
	if w.body == nil {
		w.body = bytes.NewBuffer(nil)
	}
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}
