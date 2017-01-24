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

const (
	volumePath = "/storageos/volumes"
)

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
