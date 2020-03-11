# Consul service

This repo contains a simple Go backend comprised of:

* 1 frontend facing service `web`
* 1 backend service `backend`

`web` uses Consul for service discovery to connect to `backend` instances and
issue requests against it.
Several Consul checks are also registered with Consul agent:

* TTL based checks which are basically heartbeats issued from services towards
  Consul agent
* HTTP healthecks which are basically HTTP requests issues from the Consul agent
  against defined services' healthcheck endpoints

To observe the state of the system please open http://localhost:8500/ in your
browser.

## How to run

```
docker-compose up -d
```

Optionally one can dynamically adjust number of backend services to observe how they
dynamically register and deregister with Consul agent.

```
docker-compose up --no-recreate -d --scale backend=5
```

## How to teardown

```
docker-compose down --remove-orphans
```
