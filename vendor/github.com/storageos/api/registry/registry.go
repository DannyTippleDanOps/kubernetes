package registry

import (
	"errors"
	"sync"

	log "github.com/Sirupsen/logrus"
)

// Common errors.
var (
	ErrServiceNotFound = errors.New("service name/version not found")
)

// Registry is an interface used to lookup the target host
// for a given service name / version pair.
type Registry interface {
	Add(name, version, endpoint string)                // Add an endpoint to our registry
	Delete(name, version, endpoint string)             // Remove an endpoint to our registry
	Failure(name, version, endpoint string, err error) // Mark an endpoint as failed.
	Lookup(name, version string) ([]string, error)     // Return the endpoint list for the given service name/version
}

// DefaultRegistry is a basic registry using the following format:
// {
//   "serviceName": {
//     "serviceVersion": [
//       "endpoint1:port",
//       "endpoint2:port"
//     ],
//   },
// }
type DefaultRegistry struct {
	mu       *sync.RWMutex
	registry map[string]map[string][]string
}

// NewDefaultRegistry - returns an instance of default registry
func NewDefaultRegistry() *DefaultRegistry {
	mu := &sync.RWMutex{}
	reg := make(map[string]map[string][]string)
	return &DefaultRegistry{mu: mu, registry: reg}
}

// Lookup return the endpoint list for the given service name/version.
func (r *DefaultRegistry) Lookup(name, version string) ([]string, error) {
	r.mu.RLock()
	targets, ok := r.registry[name][version]
	r.mu.RUnlock()
	if !ok {
		return nil, ErrServiceNotFound
	}
	return targets, nil
}

// Failure marks the given endpoint for service name/version as failed.
func (r *DefaultRegistry) Failure(name, version, endpoint string, err error) {
	// Would be used to remove an endpoint from the rotation, log the failure, etc.
	log.Infof("Error accessing %s/%s (%s): %s", name, version, endpoint, err)
}

// Add adds the given endpoit for the service name/version.
func (r *DefaultRegistry) Add(name, version, endpoint string) {
	// check whether endpoint already exists
	current, _ := r.Lookup(name, version)
	if InArray(endpoint, current) {
		// endpoint already exists, skipping
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	service, ok := r.registry[name]
	if !ok {
		service = map[string][]string{}
		r.registry[name] = service
	}
	service[version] = append(service[version], endpoint)
}

// Delete removes the given endpoit for the service name/version.
func (r *DefaultRegistry) Delete(name, version, endpoint string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	service, ok := r.registry[name]
	if !ok {
		return
	}
begin:
	for i, svc := range service[version] {
		if svc == endpoint {
			copy(service[version][i:], service[version][i+1:])
			service[version][len(service[version])-1] = ""
			service[version] = service[version][:len(service[version])-1]
			goto begin
		}
	}
}

// InArray returns true if string is in array.
func InArray(val string, array []string) bool {
	for _, v := range array {
		if v == val {
			return true
		}
	}
	return false
}
