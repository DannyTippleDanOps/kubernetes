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
	"testing"

	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/client/clientset_generated/clientset/fake"
	"k8s.io/kubernetes/pkg/types"
	"k8s.io/kubernetes/pkg/util/mount"
	utiltesting "k8s.io/kubernetes/pkg/util/testing"
	"k8s.io/kubernetes/pkg/volume"
	volumetest "k8s.io/kubernetes/pkg/volume/testing"
)

func TestCanSupport(t *testing.T) {
	tmpDir, err := utiltesting.MkTmpdir("storageos_test")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	plugMgr := volume.VolumePluginMgr{}
	plugMgr.InitPlugins(ProbeVolumePlugins(), volumetest.NewFakeVolumeHost(tmpDir, nil, nil))

	plug, err := plugMgr.FindPluginByName("kubernetes.io/storageos")
	if err != nil {
		t.Errorf("Can't find the plugin by name")
	}
	if plug.GetPluginName() != "kubernetes.io/storageos" {
		t.Errorf("Wrong name: %s", plug.GetPluginName())
	}
	if !plug.CanSupport(&volume.Spec{Volume: &v1.Volume{VolumeSource: v1.VolumeSource{StorageOS: &v1.StorageOSVolumeSource{}}}}) {
		t.Errorf("Expected true")
	}
	if !plug.CanSupport(&volume.Spec{PersistentVolume: &v1.PersistentVolume{Spec: v1.PersistentVolumeSpec{PersistentVolumeSource: v1.PersistentVolumeSource{StorageOS: &v1.StorageOSVolumeSource{}}}}}) {
		t.Errorf("Expected true")
	}
}

func TestGetAccessModes(t *testing.T) {
	tmpDir, err := utiltesting.MkTmpdir("storageos_test")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	plugMgr := volume.VolumePluginMgr{}
	plugMgr.InitPlugins(ProbeVolumePlugins(), volumetest.NewFakeVolumeHost(tmpDir, nil, nil))

	plug, err := plugMgr.FindPersistentPluginByName("kubernetes.io/storageos")
	if err != nil {
		t.Errorf("Can't find the plugin by name")
	}
	if !contains(plug.GetAccessModes(), v1.ReadWriteOnce) || !contains(plug.GetAccessModes(), v1.ReadOnlyMany) {
		t.Errorf("Expected two AccessModeTypes:  %s and %s", v1.ReadWriteOnce, v1.ReadOnlyMany)
	}
}

type fakePDManager struct {
}

func (fake *fakePDManager) create(p *storageosProvisioner) (volID string, sizeGB int, labels map[string]string, err error) {
	labels = make(map[string]string)
	labels["fakepdmanager"] = "yes"
	return "test-storageos-volume-name", 100, labels, nil
}

func (fake *fakePDManager) attach(b *storageosMounter) (string, error) {
	return "", nil
}

func (fake *fakePDManager) detach(b *storageosUnmounter, loopDevice string) error {
	return nil
}

func (fake *fakePDManager) mount(b *storageosMounter, mntDevice, deviceMountPath string) error {
	return nil
}

func (fake *fakePDManager) unmount(b *storageosUnmounter, mountPath string) error {
	return nil
}

func (fake *fakePDManager) delete(d *storageosDeleter) error {
	if d.volName != "test-storageos-volume-name" {
		return fmt.Errorf("Deleter got unexpected volume name: %s", d.volName)
	}
	return nil
}

func TestPlugin(t *testing.T) {
	tmpDir, err := utiltesting.MkTmpdir("storageos_test")
	if err != nil {
		t.Fatalf("can't make a temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	plugMgr := volume.VolumePluginMgr{}
	plugMgr.InitPlugins(ProbeVolumePlugins(), volumetest.NewFakeVolumeHost(tmpDir, nil, nil))

	plug, err := plugMgr.FindPluginByName("kubernetes.io/storageos")
	if err != nil {
		t.Errorf("Can't find the plugin by name")
	}
	spec := &v1.Volume{
		Name: "vol1",
		VolumeSource: v1.VolumeSource{
			StorageOS: &v1.StorageOSVolumeSource{
				VolumeID: "base64hash",
				FSType:   "ext4",
			},
		},
	}
	fakeManager := &fakePDManager{}
	fakeMounter := &mount.FakeMounter{}
	pod := &v1.Pod{ObjectMeta: v1.ObjectMeta{UID: types.UID("poduid")}}
	mounter, err := plug.(*storageosPlugin).newMounterInternal(volume.NewSpecFromVolume(spec), pod, fakeManager, fakeMounter)
	if err != nil {
		t.Errorf("Failed to make a new Mounter: %v", err)
	}
	if mounter == nil {
		t.Errorf("Got a nil Mounter")
	}

	volPath := path.Join(tmpDir, "pods/poduid/volumes/kubernetes.io~storageos/vol1")
	path := mounter.GetPath()
	if path != volPath {
		t.Errorf("Got unexpected path: %s", path)
	}

	if err := mounter.SetUp(nil); err != nil {
		t.Errorf("Expected success, got: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			t.Errorf("SetUp() failed, volume path not created: %s", path)
		} else {
			t.Errorf("SetUp() failed: %v", err)
		}
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			t.Errorf("SetUp() failed, volume path not created: %s", path)
		} else {
			t.Errorf("SetUp() failed: %v", err)
		}
	}

	fakeManager = &fakePDManager{}
	unmounter, err := plug.(*storageosPlugin).newUnmounterInternal("vol1", types.UID("poduid"), fakeManager, fakeMounter)
	if err != nil {
		t.Errorf("Failed to make a new Unmounter: %v", err)
	}
	if unmounter == nil {
		t.Errorf("Got a nil Unmounter")
	}

	if err := unmounter.TearDown(); err != nil {
		t.Errorf("Expected success, got: %v", err)
	}
	if _, err := os.Stat(path); err == nil {
		t.Errorf("TearDown() failed, volume path still exists: %s", path)
	} else if !os.IsNotExist(err) {
		t.Errorf("SetUp() failed: %v", err)
	}

	// Test Provisioner
	options := volume.VolumeOptions{
		PVC: volumetest.CreateTestPVC("100Mi", []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce}),
		PersistentVolumeReclaimPolicy: v1.PersistentVolumeReclaimDelete,
	}
	provisioner, err := plug.(*storageosPlugin).newProvisionerInternal(options, &fakePDManager{})
	persistentSpec, err := provisioner.Provision()
	if err != nil {
		t.Errorf("Provision() failed: %v", err)
	}

	if persistentSpec.Spec.PersistentVolumeSource.StorageOS.VolumeID != "test-storageos-volume-name" {
		t.Errorf("Provision() returned unexpected volume ID: %s", persistentSpec.Spec.PersistentVolumeSource.StorageOS.VolumeID)
	}
	cap := persistentSpec.Spec.Capacity[v1.ResourceStorage]
	size := cap.Value()
	if size != 100*1024*1024*1024 {
		t.Errorf("Provision() returned unexpected volume size: %v", size)
	}

	if persistentSpec.Labels["fakepdmanager"] != "yes" {
		t.Errorf("Provision() returned unexpected labels: %v", persistentSpec.Labels)
	}

	// Test Deleter
	volSpec := &volume.Spec{
		PersistentVolume: persistentSpec,
	}
	deleter, err := plug.(*storageosPlugin).newDeleterInternal(volSpec, &fakePDManager{})
	err = deleter.Delete()
	if err != nil {
		t.Errorf("Deleter() failed: %v", err)
	}
}

func contains(modes []v1.PersistentVolumeAccessMode, mode v1.PersistentVolumeAccessMode) bool {
	for _, m := range modes {
		if m == mode {
			return true
		}
	}
	return false
}

func doTestPlugin(t *testing.T, spec *volume.Spec) {
	tmpDir, err := utiltesting.MkTmpdir("storageos_test")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	plugMgr := volume.VolumePluginMgr{}
	plugMgr.InitPlugins(ProbeVolumePlugins(), volumetest.NewFakeVolumeHost(tmpDir, nil, nil))

	plug, err := plugMgr.FindPluginByName("kubernetes.io/storageos")
	if err != nil {
		t.Errorf("Can't find the plugin by name")
	}

	pod := &v1.Pod{ObjectMeta: v1.ObjectMeta{UID: types.UID("poduid")}}
	mounter, err := plug.(*storageosPlugin).newMounterInternal(spec, pod, &storageosUtil{}, &mount.FakeMounter{})
	if err != nil {
		t.Errorf("Failed to make a new Mounter: %v", err)
	}
	if mounter == nil {
		t.Error("Got a nil Mounter")
	}
	volumePath := mounter.GetPath()
	t.Logf("volumePath: %s", volumePath)

	path := mounter.GetPath()
	t.Logf("path: %s", path)
	expectedPath := fmt.Sprintf("%s/plugins/kubernetes.io~storageos/vol1", tmpDir)
	if path != expectedPath {
		t.Errorf("Unexpected path, expected %q, got: %q", expectedPath, path)
	}
	if err := mounter.SetUp(nil); err != nil {
		t.Errorf("Expected success, got: %v", err)
	}
	if _, err := os.Stat(volumePath); err != nil {
		if os.IsNotExist(err) {
			t.Errorf("SetUp() failed, volume path not created: %s", volumePath)
		} else {
			t.Errorf("SetUp() failed: %v", err)
		}
	}
	unmounter, err := plug.(*storageosPlugin).newUnmounterInternal("vol1", types.UID("poduid"), &storageosUtil{}, &mount.FakeMounter{})
	if err != nil {
		t.Errorf("Failed to make a new Unmounter: %v", err)
	}
	if unmounter == nil {
		t.Error("Got a nil Unmounter")
	}
	if err := unmounter.TearDown(); err != nil {
		t.Errorf("Expected success, got: %v", err)
	}
	if _, err := os.Stat(volumePath); err == nil {
		t.Errorf("TearDown() failed, volume path still exists: %s", volumePath)
	} else if !os.IsNotExist(err) {
		t.Errorf("SetUp() failed: %v", err)
	}
}

func TestPluginVolume(t *testing.T) {
	vol := &v1.Volume{
		Name: "vol1",
		VolumeSource: v1.VolumeSource{
			StorageOS: &v1.StorageOSVolumeSource{VolumeID: "base64hash", ReadOnly: false},
		},
	}
	doTestPlugin(t, volume.NewSpecFromVolume(vol))
}

func TestPluginPersistentVolume(t *testing.T) {
	vol := &v1.PersistentVolume{
		ObjectMeta: v1.ObjectMeta{
			Name: "vol1",
		},
		Spec: v1.PersistentVolumeSpec{
			PersistentVolumeSource: v1.PersistentVolumeSource{
				StorageOS: &v1.StorageOSVolumeSource{VolumeID: "base64hash", ReadOnly: false},
			},
		},
	}

	doTestPlugin(t, volume.NewSpecFromPersistentVolume(vol, false))
}

func TestPersistentClaimReadOnlyFlag(t *testing.T) {
	tmpDir, err := utiltesting.MkTmpdir("storageos_test")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	pv := &v1.PersistentVolume{
		ObjectMeta: v1.ObjectMeta{
			Name: "pvA",
		},
		Spec: v1.PersistentVolumeSpec{
			PersistentVolumeSource: v1.PersistentVolumeSource{
				StorageOS: &v1.StorageOSVolumeSource{VolumeID: "base64hash", ReadOnly: false},
			},
			ClaimRef: &v1.ObjectReference{
				Name: "claimA",
			},
		},
	}

	claim := &v1.PersistentVolumeClaim{
		ObjectMeta: v1.ObjectMeta{
			Name:      "claimA",
			Namespace: "nsA",
		},
		Spec: v1.PersistentVolumeClaimSpec{
			VolumeName: "pvA",
		},
		Status: v1.PersistentVolumeClaimStatus{
			Phase: v1.ClaimBound,
		},
	}

	client := fake.NewSimpleClientset(pv, claim)
	plugMgr := volume.VolumePluginMgr{}
	plugMgr.InitPlugins(ProbeVolumePlugins(), volumetest.NewFakeVolumeHost(tmpDir, client, nil))
	plug, _ := plugMgr.FindPluginByName(storageosPluginName)

	// readOnly bool is supplied by persistent-claim volume source when its mounter creates other volumes
	spec := volume.NewSpecFromPersistentVolume(pv, true)
	pod := &v1.Pod{ObjectMeta: v1.ObjectMeta{Namespace: "nsA", UID: types.UID("poduid")}}
	mounter, _ := plug.NewMounter(spec, pod, volume.VolumeOptions{})

	if !mounter.GetAttributes().ReadOnly {
		t.Errorf("Expected true for mounter.IsReadOnly")
	}
}
