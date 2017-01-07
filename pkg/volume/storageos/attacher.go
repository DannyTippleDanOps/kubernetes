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
	"fmt"
	"os"
	"path"
	"time"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/types"
	"k8s.io/kubernetes/pkg/util/exec"
	"k8s.io/kubernetes/pkg/util/mount"
	"k8s.io/kubernetes/pkg/volume"
	volumeutil "k8s.io/kubernetes/pkg/volume/util"
)

type storageosAttacher struct {
	host       volume.VolumeHost
	apiVersion string
	volMgr     VolumeManager
}

var _ volume.Attacher = &storageosAttacher{}

var _ volume.AttachableVolumePlugin = &storageosPlugin{}

func (plugin *storageosPlugin) NewAttacher() (volume.Attacher, error) {
	apiVersion := "1"
	return &storageosAttacher{
		host:       plugin.host,
		apiVersion: apiVersion,
		volMgr:     NewVolumeManager(apiVersion),
	}, nil
}

func (plugin *storageosPlugin) GetDeviceMountRefs(deviceMountPath string) ([]string, error) {
	mounter := plugin.host.GetMounter()
	return mount.GetMountRefs(mounter, deviceMountPath)
}

// Attach checks with the GCE cloud provider if the specified volume is already
// attached to the node with the specified Name.
// If the volume is attached, it succeeds (returns nil).
// If it is not, Attach issues a call to the GCE cloud provider to attach it.
// Callers are responsible for retrying on failure.
// Callers are responsible for thread safety between concurrent attach and
// detach operations.
func (attacher *storageosAttacher) Attach(spec *volume.Spec, nodeName types.NodeName) (string, error) {
	volumeSource, readOnly, err := getVolumeSource(spec)
	if err != nil {
		return "", err
	}

	volumeRef := volumeSource.VolumeRef

	attached, err := attacher.volMgr.DiskIsAttached(volumeRef, nodeName)
	if err != nil {
		// Log error and continue with attach
		glog.Errorf(
			"Error checking if volume (%q) is already attached to current node (%q). Will continue and try attach anyway. err=%v",
			volumeRef, nodeName, err)
	}

	if err == nil && attached {
		// Volume is already attached to node.
		glog.Infof("Attach operation is successful. Volume %q is already attached to node %q.", volumeRef, nodeName)
	} else {
		if err := attacher.volMgr.AttachDisk(volumeRef, nodeName, readOnly); err != nil {
			glog.Errorf("Error attaching volume %q to node %q: %+v", volumeRef, nodeName, err)
			return "", err
		}
	}

	return path.Join(volumePath, volumeRef), nil
}

func (attacher *storageosAttacher) VolumesAreAttached(specs []*volume.Spec, nodeName types.NodeName) (map[*volume.Spec]bool, error) {
	volumesAttachedCheck := make(map[*volume.Spec]bool)
	// volumeSpecMap := make(map[aws.KubernetesVolumeID]*volume.Spec)
	// volumeIDList := []aws.KubernetesVolumeID{}
	for _, spec := range specs {
		// volumeSource, _, err := getVolumeSource(spec)
		// if err != nil {
		// 	glog.Errorf("Error getting volume (%q) source : %v", spec.Name(), err)
		// 	continue
		// }

		// name := aws.KubernetesVolumeID(volumeSource.VolumeID)
		// volumeIDList = append(volumeIDList, name)
		volumesAttachedCheck[spec] = true
		// volumeSpecMap[name] = spec
	}
	// attachedResult, err := attacher.awsVolumes.DisksAreAttached(volumeIDList, nodeName)
	// if err != nil {
	// 	// Log error and continue with attach
	// 	glog.Errorf(
	// 		"Error checking if volumes (%v) is already attached to current node (%q). err=%v",
	// 		volumeIDList, nodeName, err)
	// 	return volumesAttachedCheck, err
	// }
	//
	// for volumeID, attached := range attachedResult {
	// 	if !attached {
	// 		spec := volumeSpecMap[volumeID]
	// 		volumesAttachedCheck[spec] = false
	// 		glog.V(2).Infof("VolumesAreAttached: check volume %q (specName: %q) is no longer attached", volumeID, spec.Name())
	// 	}
	// }
	return volumesAttachedCheck, nil
}

func (attacher *storageosAttacher) WaitForAttach(spec *volume.Spec, devicePath string, timeout time.Duration) (string, error) {
	ticker := time.NewTicker(checkSleepDuration)
	defer ticker.Stop()
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	volumeSource, _, err := getVolumeSource(spec)
	if err != nil {
		return "", err
	}

	volumeRef := volumeSource.VolumeRef
	glog.Errorf("Dummy log enty (%q)", volumeRef)

	return "", nil

	// partition := ""
	// if volumeSource.Partition != 0 {
	// 	partition = strconv.Itoa(int(volumeSource.Partition))
	// }
	//
	// sdBefore, err := filepath.Glob(diskSDPattern)
	// if err != nil {
	// 	glog.Errorf("Error filepath.Glob(\"%s\"): %v\r\n", diskSDPattern, err)
	// }
	// sdBeforeSet := sets.NewString(sdBefore...)
	//
	// devicePaths := getDiskByIdPaths(pdName, partition)
	// for {
	// 	select {
	// 	case <-ticker.C:
	// 		glog.V(5).Infof("Checking GCE PD %q is attached.", pdName)
	// 		path, err := verifyDevicePath(devicePaths, sdBeforeSet)
	// 		if err != nil {
	// 			// Log error, if any, and continue checking periodically. See issue #11321
	// 			glog.Errorf("Error verifying GCE PD (%q) is attached: %v", pdName, err)
	// 		} else if path != "" {
	// 			// A device path has successfully been created for the PD
	// 			glog.Infof("Successfully found attached GCE PD %q.", pdName)
	// 			return path, nil
	// 		}
	// 	case <-timer.C:
	// 		return "", fmt.Errorf("Could not find attached GCE PD %q. Timeout waiting for mount paths to be created.", pdName)
	// 	}
	// }
}

func (attacher *storageosAttacher) GetDeviceMountPath(
	spec *volume.Spec) (string, error) {
	volumeSource, _, err := getVolumeSource(spec)
	if err != nil {
		return "", err
	}

	// TODO: get ID from volumeRef

	return getDevicePath(attacher.host, volumeSource.VolumeRef), nil
}

func (attacher *storageosAttacher) MountDevice(spec *volume.Spec, devicePath string, deviceMountPath string) error {
	// Only mount the PD globally once.
	mounter := attacher.host.GetMounter()
	notMnt, err := mounter.IsLikelyNotMountPoint(deviceMountPath)
	if err != nil {
		if os.IsNotExist(err) {
			if err = os.MkdirAll(deviceMountPath, 0750); err != nil {
				return err
			}
			notMnt = true
		} else {
			return err
		}
	}

	volumeSource, readOnly, err := getVolumeSource(spec)
	if err != nil {
		return err
	}

	options := []string{}
	if readOnly {
		options = append(options, "ro")
	}
	if notMnt {
		diskMounter := &mount.SafeFormatAndMount{Interface: mounter, Runner: exec.New()}
		err = diskMounter.FormatAndMount(devicePath, deviceMountPath, *volumeSource.FSType, options)
		if err != nil {
			os.Remove(deviceMountPath)
			return err
		}
		glog.V(4).Infof("formatting spec %v devicePath %v deviceMountPath %v fs %v with options %+v", spec.Name(), devicePath, deviceMountPath, volumeSource.FSType, options)
	}
	return nil
}

type storageosDetacher struct {
	host       volume.VolumeHost
	apiVersion string
	volMgr     VolumeManager
	// gceDisks gce.Disks
}

var _ volume.Detacher = &storageosDetacher{}

func (plugin *storageosPlugin) NewDetacher() (volume.Detacher, error) {
	// gceCloud, err := getCloudProvider(plugin.host.GetCloudProvider())
	// if err != nil {
	// 	return nil, err
	// }
	apiVersion := "1"
	return &storageosDetacher{
		host:       plugin.host,
		apiVersion: apiVersion,
		volMgr:     NewVolumeManager(apiVersion),
		// gceDisks: gceCloud,
	}, nil
}

// Detach checks with the GCE cloud provider if the specified volume is already
// attached to the specified node. If the volume is not attached, it succeeds
// (returns nil). If it is attached, Detach issues a call to the GCE cloud
// provider to attach it.
// Callers are responsible for retryinging on failure.
// Callers are responsible for thread safety between concurrent attach and detach
// operations.
func (detacher *storageosDetacher) Detach(deviceMountPath string, nodeName types.NodeName) error {
	pdName := path.Base(deviceMountPath)

	attached, err := detacher.volMgr.DiskIsAttached(pdName, nodeName)
	if err != nil {
		// Log error and continue with detach
		glog.Errorf(
			"Error checking if PD (%q) is already attached to current node (%q). Will continue and try detach anyway. err=%v",
			pdName, nodeName, err)
	}

	if err == nil && !attached {
		// Volume is not attached to node. Success!
		glog.Infof("Detach operation is successful. PD %q was not attached to node %q.", pdName, nodeName)
		return nil
	}

	if err = detacher.volMgr.DetachDisk(pdName, nodeName); err != nil {
		glog.Errorf("Error detaching PD %q from node %q: %v", pdName, nodeName, err)
		return err
	}

	return nil
}

func (detacher *storageosDetacher) WaitForDetach(devicePath string, timeout time.Duration) error {
	ticker := time.NewTicker(checkSleepDuration)
	defer ticker.Stop()
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case <-ticker.C:
			glog.V(5).Infof("Checking device %q is detached.", devicePath)
			if pathExists, err := volumeutil.PathExists(devicePath); err != nil {
				return fmt.Errorf("Error checking if device path exists: %v", err)
			} else if !pathExists {
				return nil
			}
		case <-timer.C:
			return fmt.Errorf("Timeout reached; PD Device %v is still attached", devicePath)
		}
	}
}

func (detacher *storageosDetacher) UnmountDevice(deviceMountPath string) error {
	return volumeutil.UnmountPath(deviceMountPath, detacher.host.GetMounter())
}
