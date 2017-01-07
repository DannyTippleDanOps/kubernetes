package storageos

import (
	"net/http"
	"testing"

	"github.com/storageos/api/httpclient"
	"github.com/storageos/api/registry"
	"github.com/storageos/api/testutil"
)

var vol = `
  {
        "controller": "a669319d-17ed-ce42-cb5a-b45bd261d22d",
        "created_by": "storageos",
        "datacentre": "",
        "description": "",
        "health": "",
        "id": "492a5643-b5ca-e721-36bd-1e5193a0bcf6",
        "inode": 1753,
        "master_controller": "a669319d-17ed-ce42-cb5a-b45bd261d22d",
        "master_volume": "",
        "mounted": false,
        "mounted_at": "0001-01-01T00:00:00Z",
        "mounted_by": "",
        "name": "test01",
        "no_of_mounts": 0,
        "pool": "4e33541d-d409-ea0a-8d74-4fb035fab92c",
        "replica": false,
        "replica_controllers": null,
        "replica_inodes": null,
        "replica_volumes": null,
        "size": 10,
        "status": "active",
        "status_message": "deploying to controller: a669319d-17ed-ce42-cb5a-b45bd261d22d",
        "tags": [
            "filesystem",
            "compression"
        ],
        "tenant": "",
        "volume_groups": null
    }`

func TestGetVolume(t *testing.T) {
	server, teardown := testutil.NewTestingServer(200, vol)
	defer teardown()

	reg := registry.NewDefaultRegistry()
	reg.Add(ControllerEndpoint, APIVersion, server.Addr)
	haClient := httpclient.NewHAHTTPClient(reg, &http.Client{}, ControllerEndpoint, APIVersion)

	manager := NewHTTPClientManager(server.Addr, haClient)

	volume, err := manager.GetVolume("492a5643-b5ca-e721-36bd-1e5193a0bcf6")
	testutil.Expect(t, err, nil)
	testutil.Expect(t, volume.Name, "test01")
}

func TestGetVolumeNotExist(t *testing.T) {
	server, teardown := testutil.NewTestingServer(404, "")
	defer teardown()

	reg := registry.NewDefaultRegistry()
	reg.Add(ControllerEndpoint, APIVersion, server.Addr)
	haClient := httpclient.NewHAHTTPClient(reg, &http.Client{}, ControllerEndpoint, APIVersion)

	manager := NewHTTPClientManager(server.Addr, haClient)

	_, err := manager.GetVolume("00000000-0000-0000-0000-000000000000")
	testutil.Refute(t, err, nil)
	testutil.Expect(t, err.Error(), ErrNotFound)
}
