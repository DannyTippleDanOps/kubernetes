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
	"github.com/pborman/uuid"

	"k8s.io/client-go/pkg/api/resource"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/types"
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

// This user is used to configure the connection to the StorageOS API service.
type storageosAPIConfig struct {
	apiAddress  string
	apiUser     string
	apiPassword string
}

var _ volume.VolumePlugin = &storageosPlugin{}
var _ volume.PersistentVolumePlugin = &storageosPlugin{}

// var _ volume.DeletableVolumePlugin = &storageosPlugin{}
// var _ volume.ProvisionableVolumePlugin = &storageosPlugin{}

const (
	storageosPluginName = "kubernetes.io/storageos"

	defaultAPIAddress  = "localhost:8000"
	defaultAPIUser     = "storageos"
	defaultAPIPassword = "storageos"

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
	glog.Infof("storageos: get volume name: %s", spec.Volume.StorageOS.VolumeRef)
	volumeSource, _, err := getVolumeSource(spec)
	if err != nil {
		return "", err
	}

	return volumeSource.VolumeRef, nil
}

func (plugin *storageosPlugin) CanSupport(spec *volume.Spec) bool {
	glog.Infof("storageos: can support")
	if (spec.PersistentVolume != nil && spec.PersistentVolume.Spec.StorageOS == nil) ||
		(spec.Volume != nil && spec.Volume.StorageOS == nil) {
		return false
	}

	// If Quobyte is already mounted we don't need to check if the binary is installed
	// if mounter, err := plugin.newMounterInternal(spec, nil, plugin.host.GetMounter()); err == nil {
	// 	qm, _ := mounter.(*quobyteMounter)
	// 	pluginDir := plugin.host.GetPluginDir(strings.EscapeQualifiedNameForDisk(quobytePluginName))
	// 	if mounted, err := qm.pluginDirIsMounted(pluginDir); mounted && err == nil {
	// 		glog.V(4).Infof("quobyte: can support")
	// 		return true
	// 	}
	// } else {
	// 	glog.V(4).Infof("quobyte: Error: %v", err)
	// }
	//
	// if out, err := exec.New().Command("ls", "/sbin/mount.quobyte").CombinedOutput(); err == nil {
	// 	glog.V(4).Infof("quobyte: can support: %s", string(out))
	// 	return true
	// }

	return false
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

func getVolumeSource(spec *volume.Spec) (*v1.StorageOSVolumeSource, bool, error) {
	if spec.Volume != nil && spec.Volume.StorageOS != nil {
		return spec.Volume.StorageOS, spec.Volume.StorageOS.ReadOnly, nil
	} else if spec.PersistentVolume != nil &&
		spec.PersistentVolume.Spec.StorageOS != nil {
		return spec.PersistentVolume.Spec.StorageOS, spec.ReadOnly, nil
	}

	return nil, false, fmt.Errorf("Spec does not reference a StorageOS volume type")
}

func (plugin *storageosPlugin) ConstructVolumeSpec(volumeRef, mountPath string) (*volume.Spec, error) {
	glog.Infof("storageos: construct volume spec: %s, mountpoint: %s", volumeRef, mountPath)
	storageosVolume := &v1.Volume{
		Name: volumeRef,
		VolumeSource: v1.VolumeSource{
			StorageOS: &v1.StorageOSVolumeSource{
				VolumeRef: volumeRef,
			},
		},
	}
	return volume.NewSpecFromVolume(storageosVolume), nil
}

func (plugin *storageosPlugin) NewMounter(spec *volume.Spec, pod *v1.Pod, _ volume.VolumeOptions) (volume.Mounter, error) {
	return plugin.newMounterInternal(spec, pod, plugin.host.GetMounter())
}

func (plugin *storageosPlugin) newMounterInternal(spec *volume.Spec, pod *v1.Pod, mounter mount.Interface) (volume.Mounter, error) {
	source, readOnly, err := getVolumeSource(spec)
	if err != nil {
		return nil, err
	}

	return &storageosMounter{
		&storageos{
			volName:   spec.Name(),
			mounter:   mounter,
			pod:       pod,
			volumeRef: source.VolumeRef,
			plugin:    plugin,
		},
		readOnly: readOnly,
	}, nil
}

func (plugin *storageosPlugin) NewUnmounter(volName string, podUID types.UID) (volume.Unmounter, error) {
	return plugin.newUnmounterInternal(volName, podUID, plugin.host.GetMounter())
}

func (plugin *storageosPlugin) newUnmounterInternal(volName string, podUID types.UID, mounter mount.Interface) (volume.Unmounter, error) {
	return &storageosUnmounter{
		&storageos{
			volName: volName,
			mounter: mounter,
			pod:     &v1.Pod{ObjectMeta: v1.ObjectMeta{UID: podUID}},
			plugin:  plugin,
		},
	}, nil
}

func (plugin *storageosPlugin) NewProvisioner(options volume.VolumeOptions) (volume.Provisioner, error) {
	return plugin.newProvisionerInternal(options)
}

func (plugin *storageosPlugin) newProvisionerInternal(options volume.VolumeOptions) (volume.Provisioner, error) {
	return &storageosVolumeProvisioner{
		storageosMounter: &storageosMounter{
			storageos: &storageos{
				plugin: plugin,
			},
		},
		options: options,
	}, nil
}

type storageosVolumeProvisioner struct {
	*storageosMounter
	options volume.VolumeOptions
}

func (p *storageosVolumeProvisioner) Provision() (*v1.PersistentVolume, error) {
	if p.options.PVC.Spec.Selector != nil {
		return nil, fmt.Errorf("claim Selector is not supported")
	}
	var err error
	user := ""
	group := "default"
	token := ""

	for k, v := range p.options.Parameters {
		switch dstrings.ToLower(k) {
		case "user":
			p.user = v
		case "group":
			p.group = v
		case "pool":
			p.pool = v
		case "usersecretname":
			secretName = v
		default:
			return nil, fmt.Errorf("invalid option %q for volume plugin %s", k, p.plugin.GetPluginName())
		}
	}
	// sanity check
	// if p.Pool == "" {
	// 	p.Pool = "default"
	// }

	// create random image name
	image := fmt.Sprintf("kubernetes-dynamic-pvc-%s", uuid.NewUUID())
	p.storageosMounter.Image = image
	rbd, sizeMB, err := r.manager.CreateImage(r)
	if err != nil {
		glog.Errorf("rbd: create volume failed, err: %v", err)
		return nil, err
	}
	glog.Infof("successfully created rbd image %q", image)
	pv := new(v1.PersistentVolume)
	rbd.SecretRef = new(v1.LocalObjectReference)
	rbd.SecretRef.Name = secretName
	rbd.RadosUser = r.Id
	pv.Spec.PersistentVolumeSource.RBD = rbd
	pv.Spec.PersistentVolumeReclaimPolicy = r.options.PersistentVolumeReclaimPolicy
	pv.Spec.AccessModes = r.options.PVC.Spec.AccessModes
	if len(pv.Spec.AccessModes) == 0 {
		pv.Spec.AccessModes = r.plugin.GetAccessModes()
	}
	pv.Spec.Capacity = v1.ResourceList{
		v1.ResourceName(v1.ResourceStorage): resource.MustParse(fmt.Sprintf("%dMi", sizeMB)),
	}
	return pv, nil
}

// storageos volumes represent a bare host directory mount of an StorageOS export.
type storageos struct {
	volID     string
	volName   string
	pod       *v1.Pod
	user      string
	group     string
	volumeRef string
	tenant    string
	config    string
	// Mounter interface that provides system calls to mount the global path to the pod local path.
	mounter mount.Interface
	plugin  *storageosPlugin
	volume.MetricsProvider
}

type storageosMounter struct {
	*storageos
	// Specifies whether the disk will be mounted as read-only.
	readOnly bool
}

var _ volume.Mounter = &storageosMounter{}

func (m *storageosMounter) GetAttributes() volume.Attributes {
	return volume.Attributes{
		ReadOnly:        m.readOnly,
		Managed:         false,
		SupportsSELinux: false,
	}
}

// Checks prior to mount operations to verify that the required components (binaries, etc.)
// to mount the volume are available on the underlying node.
// If not, it returns an error
func (m *storageosMounter) CanMount() error {
	return nil
}

// SetUp attaches the disk and bind mounts to the volume path.
func (m *storageosMounter) SetUp(fsGroup *int64) error {
	pluginDir := m.plugin.host.GetPluginDir(strings.EscapeQualifiedNameForDisk(storageosPluginName))
	return m.SetUpAt(pluginDir, fsGroup)
}

func (m *storageosMounter) SetUpAt(dir string, fsGroup *int64) error {
	// TODO: handle failed mounts here.
	fmt.Printf("StorageOS SetUpAt: %s", dir)
	notMnt, err := m.mounter.IsLikelyNotMountPoint(dir)
	glog.V(4).Infof("StorageOS volume set up: %s %v %v, volume name %v readOnly %v", dir, !notMnt, err, m.volName, m.readOnly)
	if err != nil && !os.IsNotExist(err) {
		glog.Errorf("cannot validate mount point: %s %v", dir, err)
		return err
	}
	if !notMnt {
		return nil
	}

	if err := os.MkdirAll(dir, 0750); err != nil {
		glog.Errorf("mkdir failed on disk %s (%v)", dir, err)
		return err
	}

	options := []string{}
	if m.readOnly {
		options = append(options, "ro")
	}

	globalVolPath := getDevicePath(m.plugin.host, m.volID)
	glog.V(4).Infof("attempting to mount %s", dir)

	//if a trailing slash is missing we add it here
	if err := m.mounter.Mount(globalVolPath, dir, "", options); err != nil {
		return fmt.Errorf("storageos: mount failed: %v", err)
	}

	glog.V(4).Infof("storageos: mount set up: %s", dir)

	return nil
}

// getDevicePath returns the path to the StorageOS raw device.
func getDevicePath(host volume.VolumeHost, volID string) string {
	return path.Join(host.GetPluginDir(storageosPluginName), "volumes", volID)
}

// GetPath returns the path to the user specific mount of a Quobyte volume
// Returns a path in the format ../user#group@volume
func (storageosVolume *storageos) GetPath() string {
	user := storageosVolume.user
	if len(user) == 0 {
		user = "root"
	}

	group := storageosVolume.group
	if len(group) == 0 {
		group = "nfsnobody"
	}

	// Quobyte has only one mount in the PluginDir where all Volumes are mounted
	// The Quobyte client does a fixed-user mapping
	pluginDir := storageosVolume.plugin.host.GetPluginDir(strings.EscapeQualifiedNameForDisk(storageosPluginName))
	return path.Join(pluginDir, fmt.Sprintf("%s#%s@%s", user, group, storageosVolume.volumeRef))
}

type storageosUnmounter struct {
	*storageos
}

var _ volume.Unmounter = &storageosUnmounter{}

func (u *storageosUnmounter) GetPath() string {
	return getPath(u.pod.UID, u.volName, u.plugin.host)
}

func (u *storageosUnmounter) TearDown() error {
	return u.TearDownAt(u.GetPath())
}

// We don't need to unmount on the host because only one mount exists
func (u *storageosUnmounter) TearDownAt(dir string) error {
	return nil
}
