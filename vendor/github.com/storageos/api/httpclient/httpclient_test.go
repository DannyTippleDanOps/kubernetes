package httpclient

import (
	"io"
	"net/http"
	"testing"

	"github.com/storageos/api/registry"
	"github.com/storageos/api/testutil"
)

func hello(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "Hello world!")
}

func StartTestingServer(port string) *http.Server {
	// http.HandleFunc("/testurl", hello)
	// http.ListenAndServe(port, nil)

	server := http.Server{
		Addr:    port,
		Handler: http.HandlerFunc(hello),
	}
	server.ListenAndServe()

	return &server
}

func TestGetEndpoint(t *testing.T) {
	go StartTestingServer(":13000")

	reg := registry.NewDefaultRegistry()
	reg.Add("servicename", "v1", "localhost:13000")

	c := &http.Client{}

	ha := NewHAHTTPClient(reg, c, "servicename", "v1")

	req, err := http.NewRequest("GET", "http://servicename:13000/v1/testing", nil)
	testutil.Expect(t, err, nil)

	resp, err := ha.Do(req)
	testutil.Expect(t, err, nil)
	testutil.Expect(t, resp.StatusCode, 200)
}

func TestGetEndpointWithMultipleFailing(t *testing.T) {
	go StartTestingServer(":13006")

	reg := registry.NewDefaultRegistry()
	// 13001, 13002, 13003, 13004, 13005 are dead
	reg.Add("servicename", "v1", "localhost:13001")
	reg.Add("servicename", "v1", "localhost:13002")
	reg.Add("servicename", "v1", "localhost:13003")
	reg.Add("servicename", "v1", "localhost:13004")
	reg.Add("servicename", "v1", "localhost:13005")
	reg.Add("servicename", "v1", "localhost:13006")

	c := &http.Client{}

	ha := NewHAHTTPClient(reg, c, "servicename", "v1")

	// calling the dead one
	req, err := http.NewRequest("GET", "http://servicename:13001/v1/testing", nil)
	testutil.Expect(t, err, nil)

	resp, err := ha.Do(req)
	testutil.Expect(t, err, nil)
	testutil.Expect(t, resp.StatusCode, 200)
}
