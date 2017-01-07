package storageos

import (
	"fmt"
	"net/http"
	"time"
)

// Volume is used to represent a presentable storage device
type Volume struct {
	ID string `json:"id,omitempty"`

	// Replica is false for master and true for any replica volumes
	Replica bool `json:"replica"`
	// this is for director to make reading configfs easier
	Virtual bool `json:"virtual,omitempty"`
	// IDs of replica volumes
	MasterVolume   string   `json:"master_volume" mapstructure:"master_volume"`
	ReplicaVolumes []string `json:"replica_volumes" mapstructure:"replica_volumes"`
	ReplicaInodes  []string `json:"replica_inodes" mapstructure:"replica_inodes"`

	CreatedBy string `json:"created_by"`

	Datacenter    string   `json:"datacentre"`
	Tenant        string   `json:"tenant"`
	Name          string   `json:"name"`
	Status        string   `json:"status"`
	StatusMessage string   `json:"status_message"`
	Health        string   `json:"health"`
	Pool          string   `json:"pool"`
	Description   string   `json:"description"`
	Size          int      `json:"size"`
	Inode         uint32   `json:"inode"`
	Dev           string   `json:"dev,omitempty"`
	VolumeGroups  []string `json:"volume_groups" mapstructure:"volume_groups"`
	Tags          []string `json:"tags"`

	Deleted bool `json:"deleted,omitempty"`

	// Controller UUIDs for replication
	Controller string `json:"controller" mapstructure:"controller"`

	MasterController   string   `json:"master_controller" mapstructure:"master_controller"`
	ReplicaControllers []string `json:"replica_controllers" mapstructure:"replica_controllers"`

	Mounted        bool      `json:"mounted"`
	NumberOfMounts int       `json:"no_of_mounts"`
	MountedBy      string    `json:"mounted_by"`
	MountedAt      time.Time `json:"mounted_at"`
}

// Client - get specified client
func (m *HTTPClientManager) GetVolume(ref string) (*Volume, error) {

	resp, err := m.request(http.MethodGet, fmt.Sprintf("%s/%s", VolumeEndpoint, ref), nil)
	if err != nil || resp == nil {
		return nil, err
	}

	var volume Volume
	err = m.decode(resp, &volume)
	return &volume, err
}
