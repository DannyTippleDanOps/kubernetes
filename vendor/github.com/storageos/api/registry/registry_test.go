package registry

import (
	"testing"

	"github.com/storageos/api/testutil"
)

func TestAddService(t *testing.T) {
	reg := NewDefaultRegistry()

	reg.Add("sos", "v1", "127.0.0.1")

	testutil.Expect(t, reg.registry["sos"]["v1"][0], "127.0.0.1")
}

func TestLookupService(t *testing.T) {
	reg := NewDefaultRegistry()

	reg.Add("sos", "v1", "127.0.0.1")

	serviceEndpoints, err := reg.Lookup("sos", "v1")
	testutil.Expect(t, err, nil)
	testutil.Expect(t, len(serviceEndpoints), 1)
	testutil.Expect(t, serviceEndpoints[0], "127.0.0.1")
}

func TestLookupMultiple(t *testing.T) {
	reg := NewDefaultRegistry()

	reg.Add("sos", "v1", "127.0.0.1")
	reg.Add("sos", "v1", "127.0.0.2")
	reg.Add("sos", "v1", "127.0.0.3")

	serviceEndpoints, err := reg.Lookup("sos", "v1")
	testutil.Expect(t, err, nil)
	testutil.Expect(t, len(serviceEndpoints), 3)
	testutil.Expect(t, serviceEndpoints[0], "127.0.0.1")
	testutil.Expect(t, serviceEndpoints[1], "127.0.0.2")
	testutil.Expect(t, serviceEndpoints[2], "127.0.0.3")
}

func TestDeleteEndpoint(t *testing.T) {
	reg := NewDefaultRegistry()

	reg.Add("sos", "v1", "127.0.0.1")
	reg.Add("sos", "v1", "127.0.0.2")
	reg.Add("sos", "v1", "127.0.0.3")

	reg.Delete("sos", "v1", "127.0.0.2")

	serviceEndpoints, err := reg.Lookup("sos", "v1")
	testutil.Expect(t, err, nil)
	testutil.Expect(t, len(serviceEndpoints), 2)
	testutil.Expect(t, serviceEndpoints[0], "127.0.0.1")
	testutil.Expect(t, serviceEndpoints[1], "127.0.0.3")
}

func TestAddTheSame(t *testing.T) {
	reg := NewDefaultRegistry()

	reg.Add("sos", "v1", "127.0.0.1")
	reg.Add("sos", "v1", "127.0.0.1")

	endpoints, err := reg.Lookup("sos", "v1")
	testutil.Expect(t, err, nil)
	testutil.Expect(t, len(endpoints), 1)
}
