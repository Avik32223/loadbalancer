package loadbalancer

import (
	"fmt"
	"math"
	"net/http"
	"time"
)

func clamp(a, min_val, max_val int8) int8 {
	ans := math.Max(float64(min_val), float64(a))
	ans = math.Min(ans, float64(max_val))
	return int8(ans)
}

type LoadBalancer struct {
	servers  []*BackendServer
	strategy Strategy
}

func NewLoadBalancer(servers []*BackendServer, s Strategy) *LoadBalancer {
	return &LoadBalancer{
		servers:  servers,
		strategy: s,
	}
}

func (lb *LoadBalancer) next() *BackendServer {
	return lb.strategy.Next(lb.servers)
}

func (lb *LoadBalancer) StartHealthCheck() {
	for _, s := range lb.servers {
		if s.HealthCheckEnabled {
			go func(s *BackendServer) {
				for {
					s.CheckServerHealth()
					time.Sleep(s.HealthCheckFrequency)
				}
			}(s)
		}
	}
}

func (lb *LoadBalancer) Handle(w http.ResponseWriter, r *http.Request) {
	for i := 0; i < 3; i++ {
		defer func() {
			if r := recover(); r != nil {
				fmt.Println(r)
			}
		}()
		nextServer := lb.next()
		if nextServer == nil {
			http.Error(
				w,
				http.StatusText(http.StatusServiceUnavailable),
				http.StatusServiceUnavailable,
			)
			return
		}
		if err := nextServer.DoRequest(w, r); err != nil {
			fmt.Println(err)
			continue
		}
		break
	}
}
