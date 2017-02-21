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

	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/util/exec"
	"k8s.io/kubernetes/pkg/volume"

	"github.com/golang/glog"
	storageosapi "github.com/storageos/go-api"
	storageostypes "github.com/storageos/go-api/types"
)

const (
	volumePath  = "/var/lib/storageos/volumes"
	losetupPath = "losetup"

	optionFSType    = "kubernetes.io/fsType"
	optionReadWrite = "kubernetes.io/readwrite"
	optionKeySecret = "kubernetes.io/secret"

	modeBlock deviceType = iota
	modeFile
	modeUnsupported

	ErrDeviceNotFound     = "device not found"
	ErrDeviceNotCreated   = "device not created"
	ErrDeviceNotSupported = "device not supported"
	ErrNotAvailable       = "not available"
	ErrNoNoRef            = "no volume reference"
	ErrNoInvalidPVName    = "pv name could not be parsed"
)

type deviceType int

// storageosVolume describes a provisioned volume
type storageosVolume struct {
	ID        string
	Name      string
	Namespace string
	Pool      string
	Size      int
	Labels    map[string]string
	FSType    string
}

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
func (u *storageosUtil) CreateVolume(p *storageosProvisioner) (*storageosVolume, error) {
	fmt.Printf("XXX: CreateVolume: namespace: %s, volName: %s\n", p.options.PVC.Namespace, p.options.PVName)

	var labels = make(map[string]string)
	for k, v := range p.options.PVC.Labels {
		labels[k] = v
	}

	capacity := p.options.PVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)]
	requestGB := int(volume.RoundUpSize(capacity.Value(), 1024*1024*1024))

	opts := storageostypes.VolumeCreateOptions{
		Name:        p.options.PVName,
		Description: "Kubernetes volume",
		Size:        requestGB,
		Pool:        defaultPool,
		Namespace:   p.options.PVC.Namespace,
		Labels:      labels,
	}

	glog.V(4).Infof("storageos: create opts: %#v", opts)

	// fetch the volName details from the StorageOS API
	vol, err := u.client().VolumeCreate(opts)
	if err != nil {
		glog.Errorf("volume create failed for volume %q (%v)", opts.Name, err)
	}
	return &storageosVolume{
		ID:        vol.ID,
		Name:      vol.Name,
		Namespace: vol.Namespace,
		Pool:      vol.Pool,
		Size:      int(vol.Size),
		Labels:    vol.Labels,
		FSType:    vol.FSType,
	}, nil
}

// Attach exposes a volume on the host as a block device.  StorageOS uses a
// global namespace, so if the volume exists, it should already be available as
// a device within `/var/lib/storageos/volumes/<id>`.
//
// Depending on the host capabilities, the device may be either a block device
// or a file device.  Block devices can be used directly, but file devices must
// be made accessible as a block device before using.
func (u *storageosUtil) AttachVolume(b *storageosMounter) (string, error) {
	fmt.Printf("XXX: AttachVolume: namespace: %s, volName: %s\n", b.namespace, b.volName)
	if b.namespace == "" || b.volName == "" {
		fmt.Printf("XXX PANIC: AttachVolume: namespace: %s, volName: %s\n", b.namespace, b.volName)
	}

	// var namespace string
	// switch {
	// case b.namespace != "":
	// 	namespace = b.namespace
	// case b.podNamespace != "":
	// 	namespace = b.podNamespace
	// default:
	// 	namespace = defaultNamespace
	// }
	//
	// ref := b.volName
	// if b.volID != "" {
	// 	ref = b.volID
	// }

	// fmt.Printf("XXX: attach namespace: %s\n", namespace)
	// fmt.Printf("XXX: attach ref: %s\n", ref)

	// fetch the volID from the StorageOS API
	vol, err := u.client().Volume(b.namespace, b.volName)
	if err != nil {
		glog.Warningf("volume retrieve failed for volume %q with namespace %q (%v)", b.volName, b.namespace, err)
		return "", err
	}
	srcPath := path.Join(volumePath, vol.ID)
	dt, err := pathDeviceType(srcPath)
	if err != nil {
		glog.Warningf("volume source path %q for volume %q not ready (%v)", srcPath, b.volName, err)
		return "", err
	}
	switch dt {
	case modeBlock:
		return srcPath, nil
	case modeFile:
		return attachFileDevice(srcPath)
	default:
		return "", fmt.Errorf(ErrDeviceNotSupported)
	}
}

// Detach detaches a volume from the host.
func (u *storageosUtil) DetachVolume(b *storageosUnmounter, loopDevice string) error {
	fmt.Printf("XXX: DetachVolume: namespace: %s, volName: %s\n", b.namespace, b.volName)
	if b.namespace == "" || b.volName == "" {
		fmt.Printf("XXX PANIC: DetachVolume: namespace: %s, volName: %s\n", b.namespace, b.volName)
	}

	glog.V(2).Infof("storageos: detaching loopDevice %s", loopDevice)
	// return removeLoopDevice(loopDevice)
	if _, err := os.Stat(loopDevice); os.IsNotExist(err) {
		return nil
	}
	if err := removeLoopDevice(loopDevice); err != nil {
		return err
	}

	opts := storageostypes.VolumeUnmountOptions{
		Name:      b.volName,
		Namespace: b.namespace,
		Client:    b.plugin.host.GetHostName(),
	}

	return u.client().VolumeUnmount(opts)
}

// Mount mounts the volume on the host.
func (u *storageosUtil) MountVolume(b *storageosMounter, mntDevice, deviceMountPath string) error {
	fmt.Printf("XXX: MountVolume: mntDevice: %s, deviceMountPath: %s, namespace: %s, volName: %s\n", mntDevice, deviceMountPath, b.namespace, b.volName)
	// fmt.Printf("XXX: MountVolume: namespace: %s, volName: %s\n", b.namespace, b.volName)
	if b.namespace == "" || b.volName == "" {
		fmt.Printf("XXX PANIC: MountVolume: namespace: %s, volName: %s\n", b.namespace, b.volName)
	}
	notMnt, err := b.mounter.IsLikelyNotMountPoint(deviceMountPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("XXX: MountVolume: mkdir 1: %s\n", deviceMountPath)
			if err = os.MkdirAll(deviceMountPath, 0750); err != nil {
				return err
			}
			notMnt = true
		} else {
			return err
		}
	}
	fmt.Printf("XXX: MountVolume: mkdir 2: %s\n", deviceMountPath)
	if err = os.MkdirAll(deviceMountPath, 0750); err != nil {
		glog.Errorf("mkdir failed on disk %s (%v)", deviceMountPath, err)
		return err
	}
	options := []string{}
	if b.readOnly {
		options = append(options, "ro")
	}
	if notMnt {
		fmt.Printf("XXX: MountVolume: FormatAndMount(%s, %s, %s, %v)\n", mntDevice, deviceMountPath, b.fsType, options)
		err = b.diskMounter.FormatAndMount(mntDevice, deviceMountPath, b.fsType, options)
		if err != nil {
			fmt.Printf("XXX PANIC: MountVolume: FormatAndMount() err: %v\n", err)
			os.Remove(deviceMountPath)
			return err
		}
	}
	if err != nil {
		return err
	}
	fmt.Printf("XXX: MountVolume: done, updating kv\n")

	// var namespace string
	// switch {
	// case b.namespace != "":
	// 	namespace = b.namespace
	// case b.podNamespace != "":
	// 	namespace = b.podNamespace
	// default:
	// 	namespace = defaultNamespace
	// }
	//
	// ref := b.volName
	// if b.volID != "" {
	// 	ref = b.volID
	// }

	// fmt.Printf("XXX: mount namespace: %s\n", namespace)
	// fmt.Printf("XXX: mount ref: %s\n", ref)

	opts := storageostypes.VolumeMountOptions{
		Name:      b.volName,
		Namespace: b.namespace,
		Client:    b.plugin.host.GetHostName(),
	}
	return u.client().VolumeMount(opts)
}

// Unmount unmounts the volume on the host.
func (u *storageosUtil) UnmountVolume(b *storageosUnmounter) error {
	fmt.Printf("XXX: UnmountVolume: namespace: %s, volName: %s\n", b.namespace, b.volName)
	if b.namespace == "" || b.volName == "" {
		fmt.Printf("XXX PANIC: UnmountVolume: namespace: %s, volName: %s\n", b.namespace, b.volName)
	}

	// opts := storageostypes.VolumeUnmountOptions{
	// 	Name:      b.volName,
	// 	Namespace: b.namespace,
	// 	Client:    b.plugin.host.GetHostName(),
	// }
	//
	// return u.client().VolumeUnmount(opts)
	return nil
}

// Deletes a StorageOS volume.  Assumes it has already been unmounted and detached.
func (u *storageosUtil) DeleteVolume(d *storageosDeleter) error {
	fmt.Printf("XXX: DeleteVolume: namespace: %s, volName: %s\n", d.namespace, d.volName)
	if d.namespace == "" || d.volName == "" {
		fmt.Printf("XXX PANIC: DeleteVolume: namespace: %s, volName: %s\n", d.namespace, d.volName)
	}

	// namespace := defaultNamespace
	// if d.namespace != "" {
	// 	namespace = d.namespace
	// }
	// var ref string
	// switch {
	// case d.volID != "":
	// 	ref = d.volID
	// case d.volName != "":
	// 	ref = d.volName
	// case d.pvName != "":
	// 	ref = d.pvName
	// default:
	// 	return fmt.Errorf(ErrNoNoRef)
	// }

	return u.client().VolumeDelete(d.namespace, d.volName)
}

// // pathDeviceType returns
// func pathDeviceType(path string) (deviceType, error) {
// 	mode, err := pathMode(path)
// 	if err != nil {
// 		return modeUnsupported, err
// 	}
// 	switch {
// 	case isDevice(mode):
// 		return modeBlock, nil
// 	case isFile(mode):
// 		return modeFile, nil
// 	default:
// 		return modeUnsupported, nil
// 	}
// }

// pathMode returns the FileMode for a path.
func pathDeviceType(path string) (deviceType, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return modeUnsupported, err
	}
	switch mode := fi.Mode(); {
	case mode&os.ModeDevice != 0:
		return modeBlock, nil
	case mode.IsRegular():
		return modeFile, nil
	default:
		return modeUnsupported, nil
	}
}

// // isDevice returns true if the file mode is a device.
// func isDevice(m os.FileMode) bool {
// 	return m&os.ModeDevice != 0
// }
//
// // isFile returns true if the file mode is a regular file.
// func isFile(m os.FileMode) bool {
// 	return m.IsRegular()
// }

// attachFileDevice takes a path to a regular file and makes it available as an
// attached block device.
func attachFileDevice(path string) (string, error) {
	blockDevicePath, err := getLoopDevice(path)
	if err != nil && err.Error() != ErrDeviceNotFound {
		return "", err
	}

	// If no existing loop device for the path, create one
	if blockDevicePath == "" {
		glog.V(4).Infof("Creating device for path: %s", path)
		blockDevicePath, err = makeLoopDevice(path)
		if err != nil {
			return "", err
		}
	}
	return blockDevicePath, nil
}

// Returns the full path to the loop device associated with the given path.
func getLoopDevice(path string) (string, error) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return "", errors.New(ErrNotAvailable)
	}
	if err != nil {
		return "", fmt.Errorf("not attachable: %v", err)
	}

	exec := exec.New()
	args := []string{"-j", path}
	out, err := exec.Command(losetupPath, args...).CombinedOutput()
	if err != nil {
		glog.V(2).Infof("Failed device discover command for path %s: %v", path, err)
		return "", err
	}
	return parseLosetupOutputForDevice(out)
}

func makeLoopDevice(path string) (string, error) {
	exec := exec.New()
	args := []string{"-f", "--show", path}
	out, err := exec.Command(losetupPath, args...).CombinedOutput()
	if err != nil {
		glog.V(2).Infof("Failed device create command for path %s: %v", path, err)
		return "", err
	}
	return parseLosetupOutputForDevice(out)
}

func removeLoopDevice(device string) error {
	exec := exec.New()
	args := []string{"-d", device}
	out, err := exec.Command(losetupPath, args...).CombinedOutput()
	if err != nil {
		fmt.Printf("XXX: removeLoopDevice(%s) err = %v\n", device, err)
		fmt.Printf("XXX: removeLoopDevice(%s) out = %q\n", device, string(out))
		if !strings.Contains(string(out), "No such device or address") {
			return err
		}
	}
	return nil
}

func parseLosetupOutputForDevice(output []byte) (string, error) {
	if len(output) == 0 {
		return "", errors.New(ErrDeviceNotFound)
	}

	// losetup returns device in the format:
	// /dev/loop1: [0073]:148662 (/var/lib/storageos/volumes/308f14af-cf0a-08ff-c9c3-b48104318e05)
	device := strings.TrimSpace(strings.SplitN(string(output), ":", 2)[0])
	if len(device) == 0 {
		return "", errors.New(ErrDeviceNotFound)
	}
	return device, nil
}
