package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/namsral/flag"
	log "github.com/sirupsen/logrus"
)

var (
	port = flag.Int("port", 8081, "port to listen on")
)

func main() {
	flag.Parse()
	address := fmt.Sprintf(":%d", *port)

	s, err := New("backend", *port, 2*time.Second)
	if err != nil {
		panic(err)
	}

	sigs := make(chan os.Signal, 1)
	done := make(chan struct{}, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs

		select {
		case done <- struct{}{}:
			if err := s.Deregister(); err != nil {
				log.WithError(err).Fatal("Failed to deregister from consul agent")
			}
			close(done)
			return
		case <-done:
			return
		}
	}()
	defer func() {
		select {
		case done <- struct{}{}:
			if err := s.Deregister(); err != nil {
				log.WithError(err).Fatal("Failed to deregister from consul agent")
			}
			close(done)
			return
		case <-done:
			return
		}
	}()

	r := gin.Default()
	r.GET("/", s.handler)
	r.GET("/healthcheck", s.healthcheck)

	log.Infof("Starting to listen at %s...", address)
	if err := http.ListenAndServe(address, r); err != nil {
		log.WithError(err).Fatal("Failed to listen")
	}
}
