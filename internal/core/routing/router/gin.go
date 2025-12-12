package router

import (
	"github.com/DoraZa/mini-gateway/config"
	"github.com/DoraZa/mini-gateway/internal/core/routing/proxy"
	"github.com/DoraZa/mini-gateway/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
	"regexp"
)

// regexRouteEntry 用于存储编译好的正则路由规则
type regexRouteEntry struct {
	Pattern     *regexp.Regexp
	TargetRules config.RoutingRules // 假设这是你的规则结构
}

// GinRouter 管理 Gin 框架的 HTTP 路由设置
type GinRouter struct {
	regexRoutes []regexRouteEntry
}

// NewGinRouter 创建并初始化 GinRouter 实例
func NewGinRouter() *GinRouter {
	logger.Info("GinRouter initialized")
	return &GinRouter{
		regexRoutes: make([]regexRouteEntry, 0),
	}
}

// Setup 在提供的 Gin 路由器中配置 HTTP 路由规则
func (gr *GinRouter) Setup(r gin.IRouter, httpProxy *proxy.HTTPProxy, cfg *config.Config, mwRegistry map[string]gin.HandlerFunc) {
	rules := cfg.Routing.GetHTTPRules()
	if len(rules) == 0 {
		logger.Warn("No HTTP routing rules found in configuration")
		return
	}

	mode := cfg.Routing.Engine

	// 为每个路径注册路由规则
	for path, targetRules := range rules {
		logger.Info("Registering HTTP route",
			zap.String("path", path),
			zap.Any("targets", targetRules))
		isRegex := targetRules[0].IsRegex
		if isRegex && mode == "regex" {
			pattern := "^" + path + "$"
			re, err := regexp.Compile(pattern)
			if err != nil {
				logger.Error("Failed to compile regular expression for route",
					zap.String("path", path),
					zap.Error(err))
				continue
			}
			gr.regexRoutes = append(gr.regexRoutes, regexRouteEntry{
				Pattern:     re,
				TargetRules: targetRules,
			})
			logger.Info("Registered REGEX route", zap.String("pattern", path))
		} else {
			// 1. 定义处理函数切片
			var handlers []gin.HandlerFunc

			// 2. 解析中间件：既然不考虑 Method，我们默认取该路径下第一个规则的中间件配置
			if len(targetRules) > 0 {
				// 遍历配置中的中间件名字 (例如 ["auth", "logger"])
				for _, name := range targetRules[0].Middlewares {
					// 从注册表中查找对应的 HandlerFunc
					if mwFunc, ok := mwRegistry[name]; ok {
						handlers = append(handlers, mwFunc)
					} else {
						logger.Warn("Middleware configured but not found in registry",
							zap.String("path", path),
							zap.String("middleware", name))
					}
				}
			}

			// 3. 将原本的代理 Handler 追加到切片末尾
			handlers = append(handlers, httpProxy.CreateHTTPHandler(targetRules))

			// 4. 注册路由 (展开 handlers 切片)
			r.Any(path, handlers...)
		}
	}
	if len(gr.regexRoutes) > 0 {
		if engine, ok := r.(*gin.Engine); ok {
			engine.NoRoute(func(c *gin.Context) {
				gr.handleRegexMatch(c, httpProxy, mwRegistry)
			})
		}
	}
}

func (gr *GinRouter) handleRegexMatch(c *gin.Context, httpProxy *proxy.HTTPProxy, mwRegistry map[string]gin.HandlerFunc) {
	requestPath := c.Request.URL.Path

	for _, entry := range gr.regexRoutes {
		if entry.Pattern.MatchString(requestPath) {
			logger.Info("Regex route matched", zap.String("path", requestPath))

			targetRules := entry.TargetRules

			if len(targetRules) > 0 {
				for _, name := range targetRules[0].Middlewares {
					if mwFunc, ok := mwRegistry[name]; ok {
						mwFunc(c)
						// 关键点：每次执行完检查是否 Abort
						// 如果鉴权失败调用了 c.Abort()，这里必须立即停止
						if c.IsAborted() {
							return
						}
					} else {
						logger.Warn("Middleware not found", zap.String("name", name))
					}
				}
			}

			// 只有前面的中间件全部通过，才执行代理
			proxyHandler := httpProxy.CreateHTTPHandler(targetRules)
			proxyHandler(c)

			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": http.StatusNotFound})
}
