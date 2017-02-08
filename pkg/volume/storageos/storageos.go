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

	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/types"
	"k8s.io/kubernetes/pkg/util/exec"
	"k8s.io/kubernetes/pkg/util/mount"
	"k8s.io/kubernetes/pkg/util/strings"
	"k8s.io/kubernetes/pkg/volume"
)

// ProbeVolumePlugins is the primary entrypoint for volume plugins.
func ProbeVolumePlugins() []volume.VolumePlugin {
	return []volume.VolumePlugin{&storageosPlugin{nil}}
}

type storageosPlugin struct {
	host volume.VolumeHost
}

// // This user is used to configure the connection to the StorageOS API service.
// type storageosAPIConfig struct {
// 	apiAddress  string
// 	apiUser     string
// 	apiPassword string
// 	apiVersion  string
// }

var _ volume.VolumePlugin = &storageosPlugin{}
var _ volume.PersistentVolumePlugin = &storageosPlugin{}

// var _ volume.DeletableVolumePlugin = &storageosPlugin{}
var _ volume.ProvisionableVolumePlugin = &storageosPlugin{}

const (
	storageosPluginName = "kubernetes.io/storageos"

	defaultAPIAddress  = "tcp://localhost:8000"
	defaultAPIUser     = "storageos"
	defaultAPIPassword = "storageos"
	defaultAPIVersion  = "1"
	defaultFSType      = "ext2"
	defaultSizeGB      = 1
	defaultPool        = "default"
	defaultNamespace   = "default"

	checkSleepDuration = time.Second
)

func getPath(uid types.UID, volName string, host volume.VolumeHost) string {
	return host.GetPodVolumeDir(uid, strings.EscapeQualifiedNameForDisk(storageosPluginName), volName)
}

func (plugin *storageosPlugin) Init(host volume.VolumeHost) error {
	plugin.host = host
	return nil
}

func (plugin *storageosPlugin) GetPluginName() string {
	return storageosPluginName
}

func (plugin *storageosPlugin) GetVolumeName(spec *volume.Spec) (string, error) {
	volumeSource, _, err := getVolumeSource(spec)
	if err != nil {
		return "", err
	}
	return volumeSource.VolumeID, nil
}

func (plugin *storageosPlugin) CanSupport(spec *volume.Spec) bool {
	return (spec.PersistentVolume != nil && spec.PersistentVolume.Spec.StorageOS != nil) ||
		(spec.Volume != nil && spec.Volume.StorageOS != nil)
}

func (plugin *storageosPlugin) RequiresRemount() bool {
	return false
}

func (plugin *storageosPlugin) GetAccessModes() []v1.PersistentVolumeAccessMode {
	return []v1.PersistentVolumeAccessMode{
		v1.ReadWriteOnce,
		v1.ReadOnlyMany,
	}
}

func (plugin *storageosPlugin) NewMounter(spec *volume.Spec, pod *v1.Pod, _ volume.VolumeOptions) (volume.Mounter, error) {
	return plugin.newMounterInternal(spec, pod, &storageosUtil{}, plugin.host.GetMounter())
}

func (plugin *storageosPlugin) newMounterInternal(spec *volume.Spec, pod *v1.Pod, manager storageosManager, mounter mount.Interface) (volume.Mounter, error) {
	source, readOnly, err := getVolumeSource(spec)
	if err != nil {
		return nil, err
	}

	return &storageosMounter{
		storageos: &storageos{
			podUID:       pod.UID,
			podNamespace: pod.Namespace,
			podName:      pod.Name,
			volName:      spec.Name(),
			fsType:       source.FSType,
			manager:      manager,
			mounter:      mounter,
			plugin:       plugin,
		},

		readOnly: readOnly,
		// options:  source.Options,
		// manager:            manager,
		blockDeviceMounter: &mount.SafeFormatAndMount{Interface: mounter, Runner: exec.New()},
	}, nil
}

func (plugin *storageosPlugin) NewUnmounter(volName string, podUID types.UID) (volume.Unmounter, error) {
	return plugin.newUnmounterInternal(volName, podUID, &storageosUtil{}, plugin.host.GetMounter())
}

func (plugin *storageosPlugin) newUnmounterInternal(volName string, podUID types.UID, manager storageosManager, mounter mount.Interface) (volume.Unmounter, error) {
	return &storageosUnmounter{
		storageos: &storageos{
			podUID:  podUID,
			volName: volName,
			manager: manager,
			mounter: mounter,
			plugin:  plugin,
		},
	}, nil
}

func (plugin *storageosPlugin) NewDeleter(spec *volume.Spec) (volume.Deleter, error) {
	return plugin.newDeleterInternal(spec, &storageosUtil{})
}

func (plugin *storageosPlugin) newDeleterInternal(spec *volume.Spec, manager storageosManager) (volume.Deleter, error) {
	if spec.PersistentVolume != nil && spec.PersistentVolume.Spec.StorageOS == nil {
		return nil, fmt.Errorf("spec.PersistentVolumeSource.StorageOS is nil")
	}
	return &storageosDeleter{
		storageos: &storageos{
			volName: spec.Name(),
			volID:   spec.PersistentVolume.Spec.StorageOS.VolumeID,
			manager: manager,
			plugin:  plugin,
		}}, nil
}

func (plugin *storageosPlugin) ConstructVolumeSpec(volumeName, mountPath string) (*volume.Spec, error) {
	glog.Infof("storageos: ConstructVolumeSpec: volumeName: %s, mountPath: %s", volumeName, mountPath)
	volID := "TODO"
	storageosVolume := &v1.Volume{
		Name: volumeName,
		VolumeSource: v1.VolumeSource{
			StorageOS: &v1.StorageOSVolumeSource{
				VolumeID: volID,
			},
		},
	}
	return volume.NewSpecFromVolume(storageosVolume), nil
}

func (plugin *storageosPlugin) NewProvisioner(options volume.VolumeOptions) (volume.Provisioner, error) {
	return plugin.newProvisionerInternal(options, &storageosUtil{})
}

// func (plugin *storageosPlugin) newProvisionerInternal(options volume.VolumeOptions, manager storageosManager) (volume.Provisioner, error) {
// 	return &storageosVolumeProvisioner{
// 		storageosMounter: &storageosMounter{
// 			storageos: &storageos{
// 				manager: manager,
// 				plugin:  plugin,
// 			},
// 		},
// 		options: options,
// 	}, nil
// }

func (plugin *storageosPlugin) newProvisionerInternal(options volume.VolumeOptions, manager storageosManager) (volume.Provisioner, error) {
	return &storageosProvisioner{
		storageos: &storageos{
			manager: manager,
			plugin:  plugin,
		},
		options: options,
	}, nil
}

type storageosDeleter struct {
	*storageos
}

var _ volume.Deleter = &storageosDeleter{}

func (d *storageosDeleter) GetPath() string {
	return getPath(d.podUID, d.volName, d.plugin.host)
}

func (d *storageosDeleter) Delete() error {
	return d.manager.delete(d)
}

type storageosProvisioner struct {
	*storageos
	options volume.VolumeOptions
}

var _ volume.Provisioner = &storageosProvisioner{}

func (v *storageosProvisioner) Provision() (*v1.PersistentVolume, error) {
	// glog.Infof("storageos: Provision v: %#v", v)
	// glog.Infof("storageos: Provision v.options.PVC: %#v", v.options.PVC)
	// glog.Infof("storageos: Provision v.options.Parameters: %#v", v.options.Parameters)
	if v.options.PVC.Spec.Selector != nil {
		return nil, fmt.Errorf("claim Selector is not supported")
	}

	volID, sizeGB, labels, err := v.manager.create(v)
	if err != nil {
		return nil, err
	}

	pv := &v1.PersistentVolume{
		ObjectMeta: v1.ObjectMeta{
			Name:   v.options.PVName,
			Labels: map[string]string{},
			Annotations: map[string]string{
				"kubernetes.io/createdby": "storageos-dynamic-provisioner",
			},
		},
		Spec: v1.PersistentVolumeSpec{
			PersistentVolumeReclaimPolicy: v.options.PersistentVolumeReclaimPolicy,
			AccessModes:                   v.options.PVC.Spec.AccessModes,
			Capacity: v1.ResourceList{
				v1.ResourceName(v1.ResourceStorage): resource.MustParse(fmt.Sprintf("%dGi", sizeGB)),
			},
			PersistentVolumeSource: v1.PersistentVolumeSource{
				StorageOS: &v1.StorageOSVolumeSource{
					VolumeID: volID,
					FSType:   defaultFSType,
					ReadOnly: false,
				},
			},
		},
	}
	if len(v.options.PVC.Spec.AccessModes) == 0 {
		pv.Spec.AccessModes = v.plugin.GetAccessModes()
	}

	if len(labels) != 0 {
		if pv.Labels == nil {
			pv.Labels = make(map[string]string)
		}
		for k, v := range labels {
			pv.Labels[k] = v
		}
	}

	glog.V(3).Infof("storageos: Provision volume %#v", pv)
	return pv, nil
}

// storageos volumes represent a bare host directory mount of an StorageOS export.
type storageos struct {
	// podUID is the UID of the pod.
	podUID types.UID
	// podNamespace is the namespace of the pod.
	podNamespace string
	// podName is the name of the pod.
	podName string
	// volName is the ID of the pod volume.
	volID string
	// volName is the name of the pod volume.
	volName string
	// fsType is the type of the filesystem to create on the volume.
	fsType string
	// Utility interface to provision and delete disks
	manager storageosManager
	// Mounter interface that provides system calls to mount the global path to the pod local path.
	mounter mount.Interface
	plugin  *storageosPlugin
	volume.MetricsProvider
}

type storageosMounter struct {
	*storageos
	// fsType is the type of the filesystem to create on the volume.
	// fsType string
	// readOnly specifies whether the disk will be setup as read-only.
	readOnly bool
	// options are the extra params that will be passed to the plugin
	// driverName.
	// options map[string]string

	// manager is the utility interface that provides API calls to StorageOS
	// to setup & teardown disks
	// manager storageosManager
	// blockDeviceMounter provides the interface to create filesystem if the
	// filesystem doesn't exist.
	blockDeviceMounter *mount.SafeFormatAndMount

	// volume.MetricsNil
}

var _ volume.Mounter = &storageosMounter{}

func (b *storageosMounter) GetAttributes() volume.Attributes {
	return volume.Attributes{
		ReadOnly:        b.readOnly,
		Managed:         !b.readOnly,
		SupportsSELinux: true,
	}
}

// Checks prior to mount operations to verify that the required components (binaries, etc.)
// to mount the volume are available on the underlying node.
// If not, it returns an error
func (b *storageosMounter) CanMount() error {
	return nil
}

// SetUp attaches the disk and bind mounts to the volume path.
func (b *storageosMounter) SetUp(fsGroup *int64) error {
	// Attach the StorageOS volume as a loop device
	devicePath, err := b.manager.attach(b)
	if err != nil {
		glog.Errorf("Failed to attach StorageOS volume %s: %s", b.volName, err.Error())
		return err
	}

	// Mount the loop device into the plugin's disk global mount dir.
	globalPDPath := makeGlobalPDName(b.plugin.host, b.volName)
	err = b.manager.mount(b, devicePath, globalPDPath)
	if err != nil {
		return err
	}
	glog.V(4).Infof("Successfully mounted StorageOS volume %s into global mount directory", b.volName)

	// Bind mount the volume into the pod
	return b.SetUpAt(b.GetPath(), fsGroup)
}

// SetUp bind mounts the disk global mount to the give volume path.
func (b *storageosMounter) SetUpAt(dir string, fsGroup *int64) error {
	notMnt, err := b.mounter.IsLikelyNotMountPoint(dir)

	if err != nil && !os.IsNotExist(err) {
		glog.Errorf("Could not validate mount point: %s %v", dir, err)
		return err
	}
	if !notMnt {
		return nil
	}

	if err = os.MkdirAll(dir, 0750); err != nil {
		glog.Errorf("mkdir failed on disk %s (%v)", dir, err)
		return err
	}

	// Perform a bind mount to the full path to allow duplicate mounts of the same PD.
	options := []string{"bind"}
	if b.readOnly {
		options = append(options, "ro")
	}

	globalPDPath := makeGlobalPDName(b.plugin.host, b.volName)
	glog.V(4).Infof("Attempting to bind mount to pod volume at %s", dir)

	err = b.mounter.Mount(globalPDPath, dir, "", options)
	if err != nil {
		notMnt, mntErr := b.mounter.IsLikelyNotMountPoint(dir)
		if mntErr != nil {
			glog.Errorf("IsLikelyNotMountPoint check failed: %v", mntErr)
			return err
		}
		if !notMnt {
			if mntErr = b.mounter.Unmount(dir); mntErr != nil {
				glog.Errorf("Failed to unmount: %v", mntErr)
				return err
			}
			notMnt, mntErr := b.mounter.IsLikelyNotMountPoint(dir)
			if mntErr != nil {
				glog.Errorf("IsLikelyNotMountPoint check failed: %v", mntErr)
				return err
			}
			if !notMnt {
				// This is very odd, we don't expect it.  We'll try again next sync loop.
				glog.Errorf("%s is still mounted, despite call to unmount().  Will try again next sync loop.", dir)
				return err
			}
		}
		os.Remove(dir)
		glog.Errorf("Mount of disk %s failed: %v", dir, err)
		return err
	}

	if !b.readOnly {
		volume.SetVolumeOwnership(b, fsGroup)
	}
	glog.V(4).Infof("StorageOS volume setup complete on %s", dir)
	return nil
}

// GetPath returns the path to the user specific mount of a StorageOS volume
func (storageosVolume *storageos) GetPath() string {
	return getPath(storageosVolume.podUID, storageosVolume.volName, storageosVolume.plugin.host)
}

type storageosUnmounter struct {
	*storageos
	// manager is the utility interface that provides API calls to StorageOS
	// to setup & teardown disks
	// manager storageosManager
}

var _ volume.Unmounter = &storageosUnmounter{}

func makeGlobalPDName(host volume.VolumeHost, devName string) string {
	return path.Join(host.GetPluginDir(storageosPluginName), mount.MountsInGlobalPDPath, devName)
}

func (b *storageosUnmounter) GetPath() string {
	return getPath(b.podUID, b.volName, b.plugin.host)
}

// Unmounts the bind mount, and detaches the disk only if the PD
// resource was the last reference to that disk on the kubelet.
func (b *storageosUnmounter) TearDown() error {

	// Unmount from pod
	mountPath := b.GetPath()
	err := b.TearDownAt(mountPath)
	if err != nil {
		glog.Errorf("storageos: Unmount from pod failed: %v", err)
		return err
	}

	// Find device name from global mount
	globalPDPath := makeGlobalPDName(b.plugin.host, b.volName)
	devicePath, _, err := mount.GetDeviceNameFromMount(b.mounter, globalPDPath)
	if err != nil {
		glog.Errorf("Detach failed when getting device from global mount: %v", err)
		return err
	}
	// Unmount from plugin's disk global mount dir.
	err = b.TearDownAt(globalPDPath)
	if err != nil {
		glog.Errorf("Detach failed during unmount: %v", err)
		return err
	}

	// Detach loop device
	err = b.manager.detach(b, devicePath)
	if err != nil {
		glog.Errorf("Detach device %s failed for volume %s: %v", devicePath, b.volName, err)
		return err
	}

	glog.V(4).Infof("Successfully unmounted StorageOS volume %s and detached devices", b.volName)

	return nil
}

// Unmounts the bind mount, and detaches the disk only if the PD
// resource was the last reference to that disk on the kubelet.
func (b *storageosUnmounter) TearDownAt(dir string) error {
	return b.manager.unmount(b, dir)
}

// storageosManager is the abstract interface to StorageOS volume ops.
type storageosManager interface {
	// Creates a StorageOS volume.
	create(provisioner *storageosProvisioner) (string, int, map[string]string, error)
	// Attaches the disk to the kubelet's host machine.
	attach(mounter *storageosMounter) (string, error)
	// Detaches the disk from the kubelet's host machine.
	detach(unmounter *storageosUnmounter, dir string) error
	// Mounts the disk on the Kubelet's host machine.
	mount(mounter *storageosMounter, mnt, dir string) error
	// Unmounts the disk from the Kubelet's host machine.
	unmount(unounter *storageosUnmounter, dir string) error
	// Deletes the storageos volume.  All data will be lost.
	delete(deleter *storageosDeleter) error
}

func getVolumeSource(spec *volume.Spec) (*v1.StorageOSVolumeSource, bool, error) {
	// glog.Infof("storageos: getVolumeSource")
	if spec.Volume != nil && spec.Volume.StorageOS != nil {
		return spec.Volume.StorageOS, spec.Volume.StorageOS.ReadOnly, nil
	} else if spec.PersistentVolume != nil &&
		spec.PersistentVolume.Spec.StorageOS != nil {
		return spec.PersistentVolume.Spec.StorageOS, spec.ReadOnly, nil
	}
	return nil, false, fmt.Errorf("Spec does not reference a StorageOS volume type")
}
