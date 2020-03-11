package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	consul "github.com/hashicorp/consul/api"
	log "github.com/sirupsen/logrus"
)

type Service struct {
	Name       string
	ID         string
	Hostname   string
	TTL        time.Duration
	TTLCheckID string

	ConsulAgent *consul.Agent
}

func New(name string, port int, ttl time.Duration) (*Service, error) {
	c, err := consul.NewClient(consul.DefaultConfig())
	if err != nil {
		return nil, err
	}

	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	ttlCheckID := fmt.Sprintf("%s-%s-ttl-check", name, hostname)
	serviceDef := &consul.AgentServiceRegistration{
		ID:      name + "-" + hostname,
		Name:    name,
		Address: hostname,
		Port:    port,
		Checks: consul.AgentServiceChecks{
			{
				Name:    "TTL check",
				CheckID: ttlCheckID,
				TTL:     ttl.String(),
				Notes:   "TTL based heartbeat",
			},
			{
				Name:     "HTTP /healthcheck",
				CheckID:  fmt.Sprintf("%s-%s-http-check", name, hostname),
				HTTP:     fmt.Sprintf("http://%s:%d/healthcheck", hostname, port),
				Interval: "10s",
				Notes:    "HTTP /healthcheck",
			},
		},
	}

	agent := c.Agent()
	if err := agent.ServiceRegister(serviceDef); err != nil {
		return nil, err
	}
	s := &Service{
		ID:          name + "-" + hostname,
		Name:        name,
		Hostname:    hostname,
		ConsulAgent: agent,
		TTL:         ttl,
		TTLCheckID:  ttlCheckID,
	}

	go s.UpdateTTL(s.Check)

	return s, nil
}

func (s *Service) healthcheck(c *gin.Context) {
	c.String(http.StatusOK, "OK")
}

func (s *Service) handler(c *gin.Context) {
	c.String(http.StatusOK, "OK - from "+s.Hostname)
}

func (s *Service) Check() (bool, error) {
	return true, nil
}

func (s *Service) UpdateTTL(check func() (bool, error)) {
	ticker := time.NewTicker(s.TTL / 2)
	for range ticker.C {
		s.update(check)
	}
}

func (s *Service) Deregister() error {
	log.Infof("Deregistering service ID %s...", s.ID)
	return s.ConsulAgent.ServiceDeregister(s.ID)
}

func (s *Service) update(check func() (bool, error)) {
	ok, err := check()
	if !ok {
		log.Printf(`err="Check failed" msg="%s"`, err.Error())
		if agentErr := s.ConsulAgent.FailTTL(s.TTLCheckID, err.Error()); agentErr != nil {
			log.Print(agentErr)
		}
	} else {
		if agentErr := s.ConsulAgent.PassTTL(s.TTLCheckID, ""); agentErr != nil {
			log.Print(agentErr)
		}
	}
}
