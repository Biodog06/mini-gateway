package loadbalancer

import (
	"fmt"
	"path"
	"strings"

	"github.com/DoraZa/mini-gateway/config"
)

func NewLoadBalancer(algorithm string, cfg *config.Config) (LoadBalancer, error) {
	switch algorithm {
	case "round-robin", "round_robin":
		return NewRoundRobin(), nil
	case "ketama":
		return NewKetama(160), nil
	case "consul":
		return NewConsulBalancer(cfg.Consul.Addr)
	case "weighted-round-robin", "weighted_round_robin":
		rules := buildWeightedRoundRobinRules(cfg)
		return NewWeightedRoundRobin(rules), nil
	default:
		return nil, fmt.Errorf("unknown load balancer algorithm: %s", algorithm)
	}
}

func buildWeightedRoundRobinRules(cfg *config.Config) map[string][]TargetWeight {
	rules := make(map[string][]TargetWeight)
	prefix := cfg.Routing.Prefix
	for p, targetRules := range cfg.Routing.GetHTTPRules() {
		// 构造完整路径以匹配请求 URL
		fullPath := path.Join(prefix, p)
		// 确保路径以 / 开头（path.Join 可能会移除开头的 / 如果 prefix 为空且 p 不以 / 开头，或者合并后）
		// 但通常 path.Join("/api", "/v1") -> "/api/v1"
		// path.Join("", "v1") -> "v1"
		if !strings.HasPrefix(fullPath, "/") {
			fullPath = "/" + fullPath
		}

		rules[fullPath] = make([]TargetWeight, len(targetRules))
		for i, rule := range targetRules {
			rules[fullPath][i] = TargetWeight{
				Target: rule.Target,
				Weight: rule.Weight,
			}
		}
	}
	return rules
}
