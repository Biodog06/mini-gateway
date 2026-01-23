package traffic

import (
	"testing"
	"time"

	"github.com/DoraZa/mini-gateway/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// 初始化测试环境，参考 leaky_bucket_test 的初始化
func initTokenBucketTest() {
	gin.SetMode(gin.TestMode)
	config.InitTestConfigManager()
	newTokenBucketTestConfig()
}

func newTokenBucketTestConfig() *config.Config {
	return &config.Config{
		Traffic: config.Traffic{
			RateLimit: config.TrafficRateLimit{
				Enabled:   true,
				QPS:       10, // 每秒 10 个请求，对应周期约 100ms
				Burst:     20,
				Algorithm: "token_bucket",
				IPLimits: map[string]config.TrafficRateLimit{
					"192.168.1.1": {QPS: 5, Burst: 10, Enabled: true},
				},
				RouteLimits: map[string]config.TrafficRateLimit{
					"/api/v1/user": {QPS: 8, Burst: 15, Enabled: true},
				},
			},
		},
	}
}

// 测试单个 TokenBucketLimiter 的行为
func TestTokenBucketLimiter(t *testing.T) {
	initTokenBucketTest()
	limiter := NewTokenBucketLimiter(20, 3)

	// 连续获取突发容量内的令牌，并记录每个令牌的时间戳
	var tokenTimes []time.Time
	for i := 0; i < 3; i++ {
		tokenTimes = append(tokenTimes, limiter.Take())
	}

	// 对于连续获取的令牌，时间戳之间的间隔应接近 50ms（20 QPS）
	for i := 1; i < len(tokenTimes); i++ {
		interval := tokenTimes[i].Sub(tokenTimes[i-1])
		// 允许一定的误差
		assert.InDelta(t, 50, interval.Milliseconds(), 20, "Token interval 应接近 50ms")
	}

	// 超出突发容量后，下一次获取令牌的等待时间应不小于 50ms
	nextToken := limiter.Take()
	extraDelay := nextToken.Sub(tokenTimes[len(tokenTimes)-1])
	assert.GreaterOrEqual(t, extraDelay, 50*time.Millisecond, "超出突发容量后，额外令牌的延时应至少为 50ms")

}

// 测试全局维度下 MultiDimensionalTokenBucket 的全局限流器
func TestMultiDimensionalTokenBucket_Global(t *testing.T) {
	initTokenBucketTest()
	cfg := config.GetConfig()
	cfg.Traffic.RateLimit.QPS = 20
	cfg.Traffic.RateLimit.Burst = 3
	mdt := NewMultiDimensionalTokenBucket(cfg)

	var tokenTimes []time.Time
	for i := 0; i < cfg.Traffic.RateLimit.Burst; i++ {
		tokenTimes = append(tokenTimes, mdt.globalLimiter.Take())
	}

	for i := 1; i < len(tokenTimes); i++ {
		interval := tokenTimes[i].Sub(tokenTimes[i-1])
		assert.InDelta(t, 50, interval.Milliseconds(), 20, "全局令牌间隔应接近 50ms")
	}

	nextToken := mdt.globalLimiter.Take()
	extraDelay := nextToken.Sub(tokenTimes[len(tokenTimes)-1])
	assert.GreaterOrEqual(t, extraDelay, 50*time.Millisecond, "全局超出突发容量后，额外令牌应至少延时 50ms")
}

// 测试针对 IP 维度的限流器
func TestMultiDimensionalTokenBucket_IP(t *testing.T) {
	initTokenBucketTest()
	cfg := config.GetConfig()
	mdt := NewMultiDimensionalTokenBucket(cfg)
	ipLimiter := mdt.getOrCreateLimiter("ip", "192.168.1.1", 20, 3)

	var tokenTimes []time.Time
	for i := 0; i < 3; i++ {
		tokenTimes = append(tokenTimes, ipLimiter.Take())
	}

	// QPS=20 => 周期约 50ms
	for i := 1; i < len(tokenTimes); i++ {
		interval := tokenTimes[i].Sub(tokenTimes[i-1])
		assert.InDelta(t, 50, interval.Milliseconds(), 20, "IP 令牌间隔应接近 50ms")
	}

	nextToken := ipLimiter.Take()
	extraDelay := nextToken.Sub(tokenTimes[len(tokenTimes)-1])
	assert.GreaterOrEqual(t, extraDelay, 50*time.Millisecond, "IP 超出突发容量后，额外令牌应至少延时 50ms")
}

// 测试针对路由维度的限流器
func TestMultiDimensionalTokenBucket_Route(t *testing.T) {
	initTokenBucketTest()
	cfg := config.GetConfig()
	mdt := NewMultiDimensionalTokenBucket(cfg)
	routeLimiter := mdt.getOrCreateLimiter("route", "/api/v1/user", 20, 3)

	var tokenTimes []time.Time
	for i := 0; i < 3; i++ {
		tokenTimes = append(tokenTimes, routeLimiter.Take())
	}

	// QPS=20 => 周期约 50ms
	for i := 1; i < len(tokenTimes); i++ {
		interval := tokenTimes[i].Sub(tokenTimes[i-1])
		assert.InDelta(t, 50, interval.Milliseconds(), 20, "路由令牌间隔应接近 50ms")
	}

	nextToken := routeLimiter.Take()
	extraDelay := nextToken.Sub(tokenTimes[len(tokenTimes)-1])
	assert.GreaterOrEqual(t, extraDelay, 50*time.Millisecond, "路由超出突发容量后，额外令牌应至少延时 50ms")
}
