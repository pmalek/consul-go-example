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
	"github.com/segmentio/consul-go/httpconsul"
	log "github.com/sirupsen/logrus"
)

var (
	port = flag.Int("port", 8080, "port to listen on")
)

func main() {
	flag.Parse()
	address := fmt.Sprintf(":%d", *port)

	// Replace the DialContext method on the default transport to use consul for
	// all host name resolutions.
	// http.DefaultTransport = &http.Transport{
	// 	Proxy: http.ProxyFromEnvironment,
	// 	DialContext: (&consul.Dialer{
	// 		Timeout:   30 * time.Second,
	// 		KeepAlive: 30 * time.Second,
	// 		DualStack: true,
	// 	}).DialContext,
	// 	ForceAttemptHTTP2:     true,
	// 	MaxIdleConns:          100,
	// 	IdleConnTimeout:       90 * time.Second,
	// 	TLSHandshakeTimeout:   10 * time.Second,
	// 	ExpectContinueTimeout: 1 * time.Second,
	// }

	// Wraps the default transport so all service names are looked up in consul.
	// The consul client uses its own transport so there's no risk of recursive
	// loop here.
	http.DefaultTransport = httpconsul.NewTransport(http.DefaultTransport)

	s, err := New("web", *port, 2*time.Second)
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
	r.GET("/", s.callBackend)
	r.GET("/healthcheck", s.healthcheck)

	log.Infof("Starting to listen at %s...", address)
	if err := http.ListenAndServe(address, r); err != nil {
		log.WithError(err).Fatal("Failed to listen")
	}
}
