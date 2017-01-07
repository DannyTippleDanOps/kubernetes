package storageos

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"

	"github.com/storageos/api/httpclient"
)

// HTTPClientManager - http based client manager
type HTTPClientManager struct {
	HTTPClient       httpclient.HTTPClient
	ConnectionString string
}

// NewHTTPClientManager - returns new HTTP based controller API
func NewHTTPClientManager(address string, c httpclient.HTTPClient) *HTTPClientManager {

	manager := &HTTPClientManager{
		HTTPClient:       c,
		ConnectionString: getConnectionString(address),
	}

	return manager
}

func getConnectionString(address string) string {
	host, port, _ := net.SplitHostPort(address)
	if host == "" {
		host = DefaultAPIHost
	}
	if port == "" {
		port = DefaultAPIPort
	}

	return fmt.Sprintf("%s://%s:%s/%s", DefaultAPIScheme, host, port, APIVersion)
}

// request - prepares request, adds auth, does it and returns response
func (m *HTTPClientManager) request(method, prefix string, body []byte) (*http.Response, error) {
	r := bytes.NewReader(body)
	req, err := http.NewRequest(method, fmt.Sprintf("%s/%s", m.ConnectionString, prefix), r)
	if err != nil {
		return nil, err
	}

	resp, err := m.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (m *HTTPClientManager) decode(resp *http.Response, obj interface{}) error {

	// checking response status code
	if resp.StatusCode == 404 {
		return fmt.Errorf(ErrNotFound)
	}
	if resp.StatusCode > 300 {
		return fmt.Errorf("%s: %d", ErrHTTPOther, resp.StatusCode)
	}

	// var v Volume
	err := json.NewDecoder(resp.Body).Decode(&obj)
	if err != nil {
		return fmt.Errorf("%s: %s", ErrJSONUnMarshall, err.Error())
	}

	return nil
}
