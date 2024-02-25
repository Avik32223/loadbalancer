package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Avik32223/loadbalancer/internal/loadbalancer"

	"github.com/ghodss/yaml"
)

type ConfigServer struct {
	Name string `json:"name,omitempty"`
	Host string `json:"host,omitempty"`
}

type Config struct {
	Host                          string         `json:"host,omitempty"`
	Port                          int            `json:"port,omitempty"`
	HealthCheckEnabled            bool           `json:"health_check_enabled,omitempty"`
	MaxHealthCheckFailures        int8           `json:"max_health_check_failures,omitempty"`
	MinHealthCheckSuccess         int8           `json:"min_health_check_success,omitempty"`
	HealthCheckFrequencyInSeconds int            `json:"health_check_frequency_in_seconds,omitempty"`
	Servers                       []ConfigServer `json:"servers"`
	Strategy                      string         `json:"strategy"`
}

func main() {
	var configFileFlag string
	flag.StringVar(&configFileFlag, "c", "./config.yaml", "yaml config file. e.g.: -c config.yaml")
	flag.Parse()
	if configFileFlag == "" {
		log.Fatal("config file not provided.")
	}
	configFile, err := os.ReadFile(configFileFlag)
	if err != nil {
		log.Fatal(err)
	}
	configData, err := yaml.YAMLToJSON(configFile)
	if err != nil {
		log.Fatal(err)
	}
	var config Config
	json.Unmarshal(configData, &config)

	var strategy loadbalancer.Strategy
	if config.Strategy == "roundrobin" {
		strategy = &loadbalancer.RoundRobin{}
	} else {
		log.Fatalf("%s is not a valid strategy", config.Strategy)
	}

	servers := make([]*loadbalancer.BackendServer, 0)
	for _, server := range config.Servers {
		instance := loadbalancer.NewBackendServer(
			server.Name,
			server.Host,
		)
		instance.HealthCheckEnabled = config.HealthCheckEnabled
		instance.HealthCheckFrequency = time.Second * time.Duration(config.HealthCheckFrequencyInSeconds)
		instance.MaxHealthCheckFailures = config.MaxHealthCheckFailures
		instance.MinHealthCheckSuccess = config.MinHealthCheckSuccess
		servers = append(servers, instance)
	}
	lb := loadbalancer.NewLoadBalancer(servers, strategy)

	http.HandleFunc("/", lb.Handle)

	if config.Host == "" {
		config.Host = "127.0.0.1"
	}

	if config.Port == 0 {
		config.Port = 80
	}

	domain := fmt.Sprintf("%s:%d", config.Host, config.Port)
	fmt.Printf("Listening on %s 🚀\n", domain)
	lb.StartHealthCheck()
	fmt.Println(http.ListenAndServe(domain, nil))
}
