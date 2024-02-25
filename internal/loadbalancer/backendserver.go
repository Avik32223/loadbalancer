package loadbalancer

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type state int

const (
	newly_onboarded state = iota
	healthy
	failing
	failed
	recovering
	disabled
	removed
)

type BackendServer struct {
	Name                   string
	Host                   string
	HealthCheckEnabled     bool
	MaxHealthCheckFailures int8
	MinHealthCheckSuccess  int8
	HealthCheckFrequency   time.Duration
	Client                 http.Client

	mu                      sync.Mutex
	state                   state
	consecutiveFailureCount int8
	consecutiveSuccessCount int8
	lastCheckedOn           time.Time
}

type serverOption func(*BackendServer)

func NewBackendServer(name, host string, options ...serverOption) *BackendServer {
	s := &BackendServer{
		Name:                   name,
		Host:                   host,
		state:                  newly_onboarded,
		HealthCheckEnabled:     false,
		MaxHealthCheckFailures: 0,
		MinHealthCheckSuccess:  0,
		Client: http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        1024,
				TLSHandshakeTimeout: 1 * time.Minute,
				IdleConnTimeout:     10 * time.Minute,
			},
		},
	}
	for _, option := range options {
		option(s)
	}
	return s
}

func (s *BackendServer) IsHealthy() bool {
	return s.state == healthy || s.state == failing || s.state == newly_onboarded
}

func (s *BackendServer) CheckServerHealth() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(r)
		}
	}()
	resp, err := s.Client.Get(s.Host)
	s.mu.Lock()
	s.lastCheckedOn = time.Now()
	if err != nil {
		s.consecutiveSuccessCount = 0
		s.consecutiveFailureCount = clamp(
			s.consecutiveFailureCount+1, 0, s.MaxHealthCheckFailures,
		)
	} else {
		defer resp.Body.Close()
		if resp.StatusCode < 500 {
			s.consecutiveFailureCount = 0
			s.consecutiveSuccessCount = clamp(
				s.consecutiveSuccessCount+1, 0, s.MinHealthCheckSuccess,
			)
		} else {
			s.consecutiveSuccessCount = 0
			s.consecutiveFailureCount = clamp(
				s.consecutiveFailureCount+1, 0, s.MaxHealthCheckFailures,
			)
		}
	}

	switch {
	case s.consecutiveFailureCount >= s.MaxHealthCheckFailures:
		s.state = failed
	case s.consecutiveFailureCount > 0:
		s.state = failing
	case s.consecutiveSuccessCount >= s.MinHealthCheckSuccess:
		s.state = healthy
	case s.consecutiveSuccessCount > 0:
		if s.state == failed || s.state == failing || s.state == recovering {
			s.state = recovering
		} else {
			s.state = healthy
		}
	}
	s.mu.Unlock()
}

func (s *BackendServer) DoRequest(w http.ResponseWriter, r *http.Request) error {
	startTime := time.Now()
	responseCode := http.StatusOK
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(r)
			responseCode = http.StatusServiceUnavailable
		}
		diffTime := time.Until(startTime).Abs().Seconds()
		fmt.Printf("%s %f %d - %s %s %s %s\n",
			r.RemoteAddr,
			diffTime,
			responseCode,
			r.Method,
			r.URL.Path,
			r.Host,
			r.UserAgent(),
		)

	}()
	writer := bufio.NewWriter(w)
	defer writer.Flush()

	req, err := http.NewRequest(
		r.Method,
		s.Host+r.URL.Path,
		bufio.NewReader(r.Body),
	)
	if err != nil {
		return err
	}
	h := r.Header.Clone()
	h.Set("X-Forwarded-For", r.RemoteAddr)
	req.Header = h

	res, err := s.Client.Do(req)
	if err != nil {
		return err
	}
	responseCode = res.StatusCode

	if _, err = io.Copy(writer, res.Body); err != nil {
		return err
	}
	res.Body.Close()
	return nil
}
