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
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"k8s.io/kubernetes/pkg/util/exec"
	"k8s.io/kubernetes/pkg/volume/util"

	"github.com/golang/glog"
	storageosapi "github.com/storageos/go-api"
	storageostypes "github.com/storageos/go-api/types"
)

const (
	volumePath  = "/storageos/volumes"
	losetupPath = "losetup"

	optionFSType    = "kubernetes.io/fsType"
	optionReadWrite = "kubernetes.io/readwrite"
	optionKeySecret = "kubernetes.io/secret"

	ErrDeviceNotFound = "device not found"
	ErrNotAvailable   = "not available"
)

// storageosUtil is the utility structure to setup and teardown devices from
// the host.
type storageosUtil struct {
	api *storageosapi.Client
}

// client returns a StorageOS API Client using default settings.
func (u *storageosUtil) client() *storageosapi.Client {
	if u.api == nil {
		api, err := storageosapi.NewVersionedClient(defaultAPIAddress, defaultAPIVersion)
		if err == nil {
			u.api = api
		}
		api.SetAuth(defaultAPIUser, defaultAPIPassword)
	}
	return u.api
}

// Creates a new StorageOS volume and makes it available as a file device within
// /storageos/volumes.
func (u *storageosUtil) create(v *storageosVolumeProvisioner) error {

	opts := storageostypes.CreateVolumeOptions{
		Name: v.volName,
	}

	glog.V(4).Infof("storageos: create opts: %#v", opts)

	// fetch the volName details from the StorageOS API
	// _, err := u.client().CreateVolume(opts)
	// if err != nil {
	// 	// TODO: return better error for Not found
	// 	return err
	// }
	return nil
}

// Attach exposes a volume on the host.  StorageOS uses a global namespace, so
// if the volume exists, it should already be available as a file device within
// /storageos/volumes.  Attach creates a loop device and returns the path to it.
func (u *storageosUtil) attach(b *storageosMounter) (string, error) {
	// fetch the volName details from the StorageOS API
	vol, err := u.client().GetVolume(b.volName)
	if err != nil {
		// TODO: return better error for Not found
		return "", err
	}
	fileDevicePath := path.Join(volumePath, vol.ID)
	blockDevicePath, err := getTargetLoopDevPath(fileDevicePath)
	if err != nil && err.Error() != ErrDeviceNotFound {
		return "", err
	}

	// If no existing loop device for the volume, create one
	if blockDevicePath == "" {
		glog.V(4).Infof("Creating device for volume: %s", b.volName)
		blockDevicePath, err = makeLoopDev(fileDevicePath)
		if err != nil {
			return "", err
		}
	}
	return blockDevicePath, nil
}

// Detach detaches a volume from the host.
func (u *storageosUtil) detach(b *storageosUnmounter, loopDevice string) error {
	return removeLoopDev(loopDevice)
}

// Mount mounts the volume on the host.
func (u *storageosUtil) mount(b *storageosMounter, mntDevice, deviceMountPath string) error {
	notMnt, err := b.mounter.IsLikelyNotMountPoint(deviceMountPath)
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
	if err = os.MkdirAll(deviceMountPath, 0750); err != nil {
		glog.Errorf("mkdir failed on disk %s (%v)", deviceMountPath, err)
		return err
	}
	options := []string{}
	if b.readOnly {
		options = append(options, "ro")
	}
	if notMnt {
		err = b.blockDeviceMounter.FormatAndMount(mntDevice, deviceMountPath, b.fsType, options)
		if err != nil {
			os.Remove(deviceMountPath)
			return err
		}

	}
	return nil
}

// Unmount unmounts the volume on the host.
func (u *storageosUtil) unmount(b *storageosUnmounter, mountPath string) error {
	return util.UnmountPath(mountPath, b.mounter)
}

// Returns the full path to the loop device associated with the given file target.
func getTargetLoopDevPath(target string) (string, error) {
	_, err := os.Stat(target)
	if os.IsNotExist(err) {
		return "", errors.New(ErrNotAvailable)
	}
	if err != nil {
		return "", fmt.Errorf("not attachable: %v", err)
	}

	exec := exec.New()
	args := []string{"-j", target}
	out, err := exec.Command(losetupPath, args...).CombinedOutput()
	if err != nil {
		glog.V(2).Infof("Failed device discover command for path %s: %v", target, err)
		return "", err
	}
	return parseLosetupOutputForDevice(out)
}

func makeLoopDev(pathname string) (string, error) {
	exec := exec.New()
	args := []string{"-f", "--show", pathname}
	out, err := exec.Command(losetupPath, args...).CombinedOutput()
	if err != nil {
		glog.V(2).Infof("Failed device create command for path %s: %v", pathname, err)
		return "", err
	}
	return parseLosetupOutputForDevice(out)
}

func removeLoopDev(device string) error {
	exec := exec.New()
	args := []string{"-d", device}
	_, err := exec.Command(losetupPath, args...).CombinedOutput()
	if err != nil {
		return err
	}
	return nil
}

func parseLosetupOutputForDevice(output []byte) (string, error) {
	if len(output) == 0 {
		return "", errors.New(ErrDeviceNotFound)
	}

	// losetup returns device in the format:
	// /dev/loop1: [0073]:148662 (/storageos/volumes/308f14af-cf0a-08ff-c9c3-b48104318e05)
	device := strings.TrimSpace(strings.SplitN(string(output), ":", 2)[0])
	if len(device) == 0 {
		return "", errors.New(ErrDeviceNotFound)
	}
	return device, nil
}
