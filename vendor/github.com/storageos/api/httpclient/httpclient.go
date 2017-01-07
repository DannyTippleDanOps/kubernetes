package httpclient

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/storageos/api/registry"

	log "github.com/Sirupsen/logrus"
)

var HTTPClientTimeout = time.Duration(10 * time.Second)

// HTTPClient - special http client used to communicate with the controlplane
// via HTTP(s) protocols
type HTTPClient interface {
	Do(req *http.Request) (resp *http.Response, err error)
}

// HAHTTPClient - HTTP client on steroids - HA based on service registry, default authentication support
type HAHTTPClient struct {
	client   *http.Client
	registry registry.Registry

	// service name and version, i.e. controller, v1
	service, version string

	username, password, token string
}

// NewHAHTTPClient - returns new HA HTTP client
func NewHAHTTPClient(reg registry.Registry, c *http.Client, service, version string) *HAHTTPClient {

	// setting default timeout
	c.Timeout = HTTPClientTimeout

	newHAClient := &HAHTTPClient{registry: reg, service: service, version: version}

	// preparing transport with inbuilt load balancing
	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		Dial:                newHAClient.getEndpoint,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	c.Transport = transport

	// adding http client
	newHAClient.client = c

	return newHAClient
}

// Common errors
var (
	ErrInvalidService = errors.New("invalid service/version")
)

// SetAuth - ability to add default authentication headers
func (c *HAHTTPClient) SetAuth(username, password, token string) error {
	c.username = username
	c.password = password
	c.token = token
	return nil
}

func (c *HAHTTPClient) getEndpoint(network, addr string) (net.Conn, error) {
	log.WithFields(log.Fields{
		"addr":            addr,
		"default_service": c.service,
		"default_version": c.version,
	}).Debug("HA Client: determining service name and version")
	if c.service != "" && c.version != "" {
		log.Debug("HA Client: default service and version supplied, using defaults...")
		return c.loadBalance(network, c.service, c.version)
	}

	addr = strings.Split(addr, ":")[0]
	tmp := strings.Split(addr, "/")
	if len(tmp) != 2 {
		return nil, ErrInvalidService
	}
	log.WithFields(log.Fields{
		"service": tmp[0],
		"version": tmp[1],
	}).Debug("HA Client: getting load balance info")
	// passing in network, service name
	return c.loadBalance(network, tmp[0], tmp[1])
}

// Do - main method to perform requests
func (c *HAHTTPClient) Do(req *http.Request) (resp *http.Response, err error) {
	// if set - adding basic authentication
	if c.username != "" && c.password != "" {
		req.SetBasicAuth(c.username, c.password)
	}
	return c.client.Do(req)
}

// loadBalance is a basic loadBalancer which randomly
// tries to connect to one of the endpoints and try again
// in case of failure.
func (c *HAHTTPClient) loadBalance(network, serviceName, serviceVersion string) (net.Conn, error) {
	endpoints, err := c.registry.Lookup(serviceName, serviceVersion)
	if err != nil {
		return nil, err
	}
	for {
		// No more endpoint, stop
		if len(endpoints) == 0 {
			break
		}
		// Select a random endpoint
		i := rand.Int() % len(endpoints)
		endpoint := endpoints[i]

		log.WithFields(log.Fields{
			"endpoint": endpoint,
		}).Debug("HA Client: trying to dial endpoint...")

		// Try to connect
		conn, err := net.Dial(network, endpoint)
		if err != nil {
			c.registry.Failure(serviceName, serviceVersion, endpoint, err)
			// Failure: remove the endpoint from the current list and try again.
			endpoints = append(endpoints[:i], endpoints[i+1:]...)
			continue
		}
		// Success: return the connection.
		return conn, nil
	}
	// No available endpoint.
	return nil, fmt.Errorf("No endpoint available for %s/%s", serviceName, serviceVersion)
}
