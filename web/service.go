package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	consul "github.com/hashicorp/consul/api"
	log "github.com/sirupsen/logrus"
)

type Service struct {
	ID         string
	Name       string
	Hostname   string
	TTL        time.Duration
	TTLCheckID string

	ConsulAgent   *consul.Agent
	ConsulCatalog *consul.Catalog
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
		ID:   name + "-" + hostname,
		Name: name,
		Port: port,
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
		ID:            name + "-" + hostname,
		Name:          name,
		Hostname:      hostname,
		ConsulAgent:   agent,
		ConsulCatalog: c.Catalog(),
		TTL:           ttl,
		TTLCheckID:    ttlCheckID,
	}

	go s.UpdateTTL(s.Check)

	return s, nil
}

func (s *Service) healthcheck(c *gin.Context) {
	c.String(http.StatusOK, "OK")
}

func (s *Service) callBackend(c *gin.Context) {
	// NOTE:
	// *** Globally registered services
	//
	// services, _, err := s.ConsulCatalog.Service("backend", "", nil)
	// if err != nil {
	// 	c.String(http.StatusInternalServerError,
	// 		"NOK - s.ConsulCatalog.Service(...); from "+s.Hostname+", err: "+err.Error())
	// 	return
	// }

	// for _, v := range services {
	// 	log.Infof("service:  %+v", v)
	// 	log.Infof("service proxy:  %+v", v.ServiceProxy)
	// 	log.Infoln()
	// }

	// NOTE:
	// *** Locally registered services
	//
	// mm, err := s.ConsulAgent.Services()
	// if err != nil {
	// 	c.String(http.StatusInternalServerError,
	// 		"NOK - s.ConsulAgent.Services(); from "+s.Hostname+", err: "+err.Error())
	// 	return
	// }
	// for k, v := range mm {
	// 	log.Infof("service: %s -  %+v", k, v)
	// 	log.Infoln()
	// }

	// Queries Consul for a list of addresses where "my-service" is available,
	// the result will be sorted to get the addresses closest to the agent first.
	// rslv := &segmentio.Resolver{}
	// addrs, err := rslv.LookupService(context.Background(), "backend")
	// if err != nil {
	// 	c.String(http.StatusInternalServerError,
	// 		"NOK - rslv.LookupService(); from "+s.Hostname+", err: "+err.Error())
	// 	return
	// }

	// for _, addr := range addrs {
	// 	log.Infof("service: %+v", addr)
	// }

	resp, err := http.Get("http://backend")
	if err != nil {
		c.String(http.StatusInternalServerError,
			"NOK - http.Get(); from "+s.Hostname+", err: "+err.Error())
		return
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		c.String(http.StatusInternalServerError,
			"NOK - ioutil.ReadAll(); from "+s.Hostname+", err: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"hostname":            s.Hostname,
		"downstream_response": string(b),
	})
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
