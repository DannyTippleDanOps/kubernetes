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

// import (
// 	storageosapi "github.com/storageos/client/api"
// 	"k8s.io/kubernetes/pkg/types"
// )
//
// // VolumeManager is interface for managing StorageOS volumes.
// type VolumeManager interface {
// 	GetVolume(volumeRef string) (*api.Volume, error)
//
// 	// AttachDisk attaches given disk to the node with the specified NodeName.
// 	// Current instance is used when instanceID is empty string.
// 	AttachDisk(diskName string, nodeName types.NodeName, readOnly bool) error
//
// 	// DetachDisk detaches given disk to the node with the specified NodeName.
// 	// Current instance is used when nodeName is empty string.
// 	DetachDisk(devicePath string, nodeName types.NodeName) error
//
// 	// DiskIsAttached checks if a disk is attached to the node with the specified NodeName.
// 	DiskIsAttached(diskName string, nodeName types.NodeName) (bool, error)
//
// 	// CreateDisk creates a new volume with given properties. Tags are serialized
// 	// as JSON into Description field.
// 	CreateDisk(name string, diskType string, zone string, sizeGb int64, tags map[string]string) error
//
// 	// DeleteDisk deletes volume.
// 	DeleteDisk(diskToDelete string) error
//
// 	// GetAutoLabelsForPD returns labels to apply to PersistentVolume
// 	// representing this PD, namely failure domain and zone.
// 	// zone can be provided to specify the zone for the PD,
// 	// if empty all managed zones will be searched.
// 	// GetAutoLabelsForPD(name string, zone string) (map[string]string, error)
// }
//
// type V1VolumeManager struct {
// 	config interface{} // TODO
// }
//
// func NewVolumeManager(apiVersion string) VolumeManager {
// 	switch apiVersion {
// 	case "1":
// 		return &V1VolumeManager{}
// 	default:
// 		return nil
// 	}
// }
//
// func (m *V1VolumeManager) GetVolume(volumeRef string) (*api.Volume, error) {
// 	return m.GetVolume(volumeRef)
// }
//
// func (m *V1VolumeManager) AttachDisk(diskName string, nodeName types.NodeName, readOnly bool) error {
// 	return nil
// }
//
// func (m *V1VolumeManager) DetachDisk(devicePath string, nodeName types.NodeName) error {
// 	return nil
// }
//
// func (m *V1VolumeManager) DiskIsAttached(diskName string, nodeName types.NodeName) (bool, error) {
// 	return true, nil
// }
//
// func (m *V1VolumeManager) CreateDisk(name string, diskType string, zone string, sizeGb int64, tags map[string]string) error {
// 	return nil
// }
//
// func (m *V1VolumeManager) DeleteDisk(diskToDelete string) error {
// 	return nil
// }
//
// // func (m *DefaultVolumeManager) GetAutoLabelsForPD(name string, zone string) (map[string]string, error) {
// // 	return nil
// // }
