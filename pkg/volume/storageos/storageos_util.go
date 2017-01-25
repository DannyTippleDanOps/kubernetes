/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package storageos

import (
	"strings"

	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/kubernetes/pkg/volume"

	"github.com/storageos/go-api/types"
)

const (
	volumePath = "/storageos/volumes"
)

type storageosVolumeManager struct {
	config *storageosAPIConfig
}

func (manager *storageosVolumeManager) createVolume(provisioner *storageosVolumeProvisioner) (storageos *v1.StorageOSVolumeSource, size int, err error) {
	capacity := provisioner.options.PVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)]
	volumeSize := int(volume.RoundUpSize(capacity.Value(), 1024*1024*1024))
	volumeOptions := &types.CreateVolumeOptions{
		Name: provisioner.volumeRef,
		Size: volumeSize,
	}

	if _, err := manager.createStorageOSClient().CreateVolume(volumeOptions); err != nil {
		return &v1.StorageOSVolumeSource{}, volumeSize, err
	}

	glog.V(4).Infof("Created StorageOS volume %s", provisioner.volume)
	return &v1.StorageOSVolumeSource{
		VolumeRef: provisioner.volumeRef,
	}, volumeSize, nil
}

// func (manager *storageosVolumeManager) deleteVolume(deleter *storageosVolumeDeleter) error {
// 	return manager.createStorageOSClient().RemoveVolume(deleter.volume)
// }

func (manager *storageosVolumeManager) createStorageOSClient() *storageos.Client {
	return storageos.NewClient("tcp://localhost:8000")
}

func (mounter *storageosMounter) pluginDirIsMounted(pluginDir string) (bool, error) {
	mounts, err := mounter.mounter.List()
	if err != nil {
		return false, err
	}

	for _, mountPoint := range mounts {
		if strings.HasPrefix(mountPoint.Type, "storageos") {
			continue
		}

		if mountPoint.Path == pluginDir {
			glog.V(4).Infof("storageos: found mountpoint %s in /proc/mounts", mountPoint.Path)
			return true, nil
		}
	}

	return false, nil
}

// type StorageOSUtil struct{}

//
// func (util *StorageOSUtil) DeleteVolume(d *StorageOSDiskDeleter) error {
// 	var deleted = true
// 	// cloud, err := getCloudProvider(d.awsElasticBlockStore.plugin.host.GetCloudProvider())
// 	// if err != nil {
// 	// 	return err
// 	// }
// 	//
// 	// deleted, err := cloud.DeleteDisk(d.volumeID)
// 	// if err != nil {
// 	// 	// AWS cloud provider returns volume.deletedVolumeInUseError when
// 	// 	// necessary, no handling needed here.
// 	// 	glog.V(2).Infof("Error deleting EBS Disk volume %s: %v", d.volumeID, err)
// 	// 	return err
// 	// }
// 	if deleted {
// 		glog.V(2).Infof("Successfully deleted EBS Disk volume %s", d.volumeID)
// 	} else {
// 		glog.V(2).Infof("Successfully deleted EBS Disk volume %s (actually already deleted)", d.volumeID)
// 	}
// 	return nil
// }
