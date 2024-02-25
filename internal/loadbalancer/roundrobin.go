package loadbalancer

import (
	"slices"
	"sync"

	local_slices "github.com/Avik32223/loadbalancer/pkg/slices"
)

type RoundRobin struct {
	mu         sync.Mutex
	lastServer *BackendServer
}

func (rr *RoundRobin) Next(s []*BackendServer) *BackendServer {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	ss := local_slices.Filter[*BackendServer](s, func(s *BackendServer) bool {
		return s.IsHealthy()
	})
	if len(ss) > 0 {
		lastIndex := slices.IndexFunc[[]*BackendServer](s, func(i *BackendServer) bool {
			return i == rr.lastServer
		})
		lastIndex = (lastIndex + 1) % len(s)
		rr.lastServer = s[lastIndex]
		return rr.lastServer
	}
	return nil
}
