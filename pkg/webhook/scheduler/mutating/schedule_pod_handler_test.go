/*
Copyright 2021 The Fluid Authors.

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

package mutating

import (
	"context"
	"fmt"
	"testing"

	datav1alpha1 "github.com/fluid-cloudnative/fluid/api/v1alpha1"
	"github.com/fluid-cloudnative/fluid/pkg/common"
	"github.com/fluid-cloudnative/fluid/pkg/utils/fake"
	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func TestAddScheduleInfoToPod(t *testing.T) {

	type testCase struct {
		name    string
		in      *corev1.Pod
		dataset *datav1alpha1.Dataset
		pv      *corev1.PersistentVolume
		pvc     *corev1.PersistentVolumeClaim
		fuse    *appsv1.DaemonSet
		wantErr bool
	}

	hostPathCharDev := corev1.HostPathCharDev
	hostPathDirectoryOrCreate := corev1.HostPathDirectoryOrCreate
	bTrue := true

	testcases := []testCase{
		{
			name: "serverless_pod_without_dataset",
			dataset: &datav1alpha1.Dataset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "noexist",
					Namespace: "big-data",
				},
				Status: datav1alpha1.DatasetStatus{
					Phase: datav1alpha1.BoundDatasetPhase,
				},
			},
			in: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "big-data",
					Labels: map[string]string{
						common.InjectServerless: common.True,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image: "test",
							Name:  "test",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "dataset",
									MountPath: "/data",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "dataset",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "noexist",
									ReadOnly:  true,
								},
							},
						},
					},
				},
			}, pv: &corev1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: "big-data-noexist",
				},
				Spec: corev1.PersistentVolumeSpec{
					PersistentVolumeSource: corev1.PersistentVolumeSource{
						CSI: &corev1.CSIPersistentVolumeSource{
							Driver: "fuse.csi.fluid.io",
							VolumeAttributes: map[string]string{
								common.VolumeAttrFluidPath: "/runtime-mnt/jindo/big-data/noexist/jindofs-fuse",
								common.VolumeAttrMountType: common.JindoRuntime,
							},
						},
					},
				},
			},
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "big-data",
				}, Spec: corev1.PersistentVolumeClaimSpec{
					VolumeName: "big-data-noexist",
				},
			},
			fuse: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "noexist-jindofs-fuse",
					Namespace: "big-data",
				},
				Spec: appsv1.DaemonSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "fuse",
									Args: []string{
										"-oroot_ns=jindo", "-okernel_cache", "-oattr_timeout=9000", "-oentry_timeout=9000",
									},
									Command: []string{"/entrypoint.sh"},
									Image:   "test",
									SecurityContext: &corev1.SecurityContext{
										Privileged: &bTrue,
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "data",
											MountPath: "/mnt/disk1",
										}, {
											Name:      "fuse-device",
											MountPath: "/dev/fuse",
										}, {
											Name:      "jindofs-fuse-mount",
											MountPath: "/jfs",
										},
									},
								},
							},
							Volumes: []corev1.Volume{
								{
									Name: "data",
									VolumeSource: corev1.VolumeSource{
										HostPath: &corev1.HostPathVolumeSource{
											Path: "/runtime_mnt/noexist",
										},
									}},
								{
									Name: "fuse-device",
									VolumeSource: corev1.VolumeSource{
										HostPath: &corev1.HostPathVolumeSource{
											Path: "/dev/fuse",
											Type: &hostPathCharDev,
										},
									},
								},
								{
									Name: "jindofs-fuse-mount",
									VolumeSource: corev1.VolumeSource{
										HostPath: &corev1.HostPathVolumeSource{
											Path: "/runtime-mnt/jindo/big-data/noexist",
											Type: &hostPathDirectoryOrCreate,
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
		}, {
			name: "serverless_pod_done",
			dataset: &datav1alpha1.Dataset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "done",
					Namespace: "big-data",
				},
				Status: datav1alpha1.DatasetStatus{
					Phase: datav1alpha1.BoundDatasetPhase,
				},
			},
			in: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "big-data",
					Labels: map[string]string{
						common.InjectServerless:  common.True,
						common.InjectSidecarDone: common.True,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image: "test",
							Name:  "test",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "dataset",
									MountPath: "/data",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "dataset",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "done",
									ReadOnly:  true,
								},
							},
						},
					},
				},
			}, pv: &corev1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: "big-data-done",
				},
				Spec: corev1.PersistentVolumeSpec{
					PersistentVolumeSource: corev1.PersistentVolumeSource{
						CSI: &corev1.CSIPersistentVolumeSource{
							Driver: "fuse.csi.fluid.io",
							VolumeAttributes: map[string]string{
								common.VolumeAttrFluidPath: "/runtime-mnt/jindo/big-data/done/jindofs-fuse",
								common.VolumeAttrMountType: common.JindoRuntime,
							},
						},
					},
				},
			},
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "done",
					Namespace: "big-data",
				}, Spec: corev1.PersistentVolumeClaimSpec{
					VolumeName: "big-data-done",
				},
			},
			fuse: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "done-jindofs-fuse",
					Namespace: "big-data",
				},
				Spec: appsv1.DaemonSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "fuse",
									Args: []string{
										"-oroot_ns=jindo", "-okernel_cache", "-oattr_timeout=9000", "-oentry_timeout=9000",
									},
									Command: []string{"/entrypoint.sh"},
									Image:   "test",
									SecurityContext: &corev1.SecurityContext{
										Privileged: &bTrue,
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "data",
											MountPath: "/mnt/disk1",
										}, {
											Name:      "fuse-device",
											MountPath: "/dev/fuse",
										}, {
											Name:      "jindofs-fuse-mount",
											MountPath: "/jfs",
										},
									},
								},
							},
							Volumes: []corev1.Volume{
								{
									Name: "data",
									VolumeSource: corev1.VolumeSource{
										HostPath: &corev1.HostPathVolumeSource{
											Path: "/runtime_mnt/done",
										},
									}},
								{
									Name: "fuse-device",
									VolumeSource: corev1.VolumeSource{
										HostPath: &corev1.HostPathVolumeSource{
											Path: "/dev/fuse",
											Type: &hostPathCharDev,
										},
									},
								},
								{
									Name: "jindofs-fuse-mount",
									VolumeSource: corev1.VolumeSource{
										HostPath: &corev1.HostPathVolumeSource{
											Path: "/runtime-mnt/jindo/big-data/done",
											Type: &hostPathDirectoryOrCreate,
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		}, {
			name: "no_serverless_pod_without_runtime",
			dataset: &datav1alpha1.Dataset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-with-csi",
					Namespace: "big-data",
				},
				Status: datav1alpha1.DatasetStatus{
					Phase: datav1alpha1.BoundDatasetPhase,
				},
			},
			in: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "big-data",
					Labels:    map[string]string{},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image: "test",
							Name:  "test",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "dataset",
									MountPath: "/data",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "dataset",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "pod-with-csi",
									ReadOnly:  true,
								},
							},
						},
					},
				},
			}, pv: &corev1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: "big-data-pod-with-csi",
				},
				Spec: corev1.PersistentVolumeSpec{
					PersistentVolumeSource: corev1.PersistentVolumeSource{
						CSI: &corev1.CSIPersistentVolumeSource{
							Driver: "fuse.csi.fluid.io",
							VolumeAttributes: map[string]string{
								common.VolumeAttrFluidPath: "/runtime-mnt/jindo/big-data/pod-with-csi/jindofs-fuse",
								common.VolumeAttrMountType: common.JindoRuntime,
							},
						},
					},
				},
			},
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-with-csi",
					Namespace: "big-data",
					Labels: map[string]string{
						common.LabelAnnotationStorageCapacityPrefix + "big-data" + "-" + "pod-with-csi": common.True,
					},
				}, Spec: corev1.PersistentVolumeClaimSpec{
					VolumeName: "big-data-pod-with-csi",
				},
			},
			fuse: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-with-csi-jindofs-fuse",
					Namespace: "big-data",
				},
				Spec: appsv1.DaemonSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "fuse",
									Args: []string{
										"-oroot_ns=jindo", "-okernel_cache", "-oattr_timeout=9000", "-oentry_timeout=9000",
									},
									Command: []string{"/entrypoint.sh"},
									Image:   "test",
									SecurityContext: &corev1.SecurityContext{
										Privileged: &bTrue,
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "data",
											MountPath: "/mnt/disk1",
										}, {
											Name:      "fuse-device",
											MountPath: "/dev/fuse",
										}, {
											Name:      "jindofs-fuse-mount",
											MountPath: "/jfs",
										},
									},
								},
							},
							Volumes: []corev1.Volume{
								{
									Name: "data",
									VolumeSource: corev1.VolumeSource{
										HostPath: &corev1.HostPathVolumeSource{
											Path: "/runtime_mnt/pod-with-csi",
										},
									}},
								{
									Name: "fuse-device",
									VolumeSource: corev1.VolumeSource{
										HostPath: &corev1.HostPathVolumeSource{
											Path: "/dev/fuse",
											Type: &hostPathCharDev,
										},
									},
								},
								{
									Name: "jindofs-fuse-mount",
									VolumeSource: corev1.VolumeSource{
										HostPath: &corev1.HostPathVolumeSource{
											Path: "/runtime-mnt/jindo/big-data/pod-with-csi",
											Type: &hostPathDirectoryOrCreate,
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
		}, {
			name: "no_serverless_pod_with_runtime",
			dataset: &datav1alpha1.Dataset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-with-fluid",
					Namespace: "big-data",
				}, Status: datav1alpha1.DatasetStatus{
					Phase: datav1alpha1.BoundDatasetPhase,
					Runtimes: []datav1alpha1.Runtime{
						{
							Type: common.JindoRuntime,
						},
					},
				},
			},
			in: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "big-data",
					Labels:    map[string]string{},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image: "test",
							Name:  "test",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "dataset",
									MountPath: "/data",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "dataset",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "pod-with-fluid",
									ReadOnly:  true,
								},
							},
						},
					},
				},
			}, pv: &corev1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: "big-data-pod-with-fluid",
				},
				Spec: corev1.PersistentVolumeSpec{
					PersistentVolumeSource: corev1.PersistentVolumeSource{
						CSI: &corev1.CSIPersistentVolumeSource{
							Driver: "fuse.csi.fluid.io",
							VolumeAttributes: map[string]string{
								common.VolumeAttrFluidPath: "/runtime-mnt/jindo/big-data/pod-with-fluid/jindofs-fuse",
								common.VolumeAttrMountType: common.JindoRuntime,
							},
						},
					},
				},
			},
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-with-fluid",
					Namespace: "big-data",
					Labels: map[string]string{
						common.LabelAnnotationStorageCapacityPrefix + "big-data" + "-" + "pod-with-fluid": common.True,
					},
				}, Spec: corev1.PersistentVolumeClaimSpec{
					VolumeName: "big-data-pod-with-fluid",
				},
			},
			fuse: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-with-fluid-jindofs-fuse",
					Namespace: "big-data",
				},
				Spec: appsv1.DaemonSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "fuse",
									Args: []string{
										"-oroot_ns=jindo", "-okernel_cache", "-oattr_timeout=9000", "-oentry_timeout=9000",
									},
									Command: []string{"/entrypoint.sh"},
									Image:   "test",
									SecurityContext: &corev1.SecurityContext{
										Privileged: &bTrue,
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "data",
											MountPath: "/mnt/disk1",
										}, {
											Name:      "fuse-device",
											MountPath: "/dev/fuse",
										}, {
											Name:      "jindofs-fuse-mount",
											MountPath: "/jfs",
										},
									},
								},
							},
							Volumes: []corev1.Volume{
								{
									Name: "data",
									VolumeSource: corev1.VolumeSource{
										HostPath: &corev1.HostPathVolumeSource{
											Path: "/runtime_mnt/pod-with-fluid",
										},
									}},
								{
									Name: "fuse-device",
									VolumeSource: corev1.VolumeSource{
										HostPath: &corev1.HostPathVolumeSource{
											Path: "/dev/fuse",
											Type: &hostPathCharDev,
										},
									},
								},
								{
									Name: "jindofs-fuse-mount",
									VolumeSource: corev1.VolumeSource{
										HostPath: &corev1.HostPathVolumeSource{
											Path: "/runtime-mnt/jindo/big-data/pod-with-fluid",
											Type: &hostPathDirectoryOrCreate,
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	objs := []runtime.Object{}
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	_ = datav1alpha1.AddToScheme(s)
	_ = appsv1.AddToScheme(s)
	for _, testcase := range testcases {
		objs = append(objs, testcase.fuse, testcase.pv, testcase.pvc, testcase.dataset)
	}

	runtime := &datav1alpha1.JindoRuntime{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-with-fluid",
			Namespace: "big-data",
		},
	}
	objs = append(objs, runtime)

	fakeClient := fake.NewFakeClientWithScheme(s, objs...)

	for _, testcase := range testcases {
		handler := &CreateUpdatePodForSchedulingHandler{
			Client: fakeClient,
		}

		err := handler.AddScheduleInfoToPod(testcase.in, testcase.in.Namespace)
		if !((err != nil) == testcase.wantErr) {
			t.Errorf("testcase %s is failed due to error %v", testcase.name, err)
		}
	}
}

func TestHandle(t *testing.T) {
	decoder, err := admission.NewDecoder(scheme.Scheme)
	if err != nil {
		t.Errorf("test failed due to err %v", err)
	}

	type testCase struct {
		name string
		req  admission.Request
		want bool
	}

	tests := []testCase{
		{
			name: "namespace_in_pod",
			req: admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Namespace: "default",
					Object: runtime.RawExtension{
						Raw: []byte(
							`{
								"apiVersion": "v1",
								"kind": "Pod",
								"metadata": {
									"name": "foo"
								},
								"spec": {
									"containers": [
										{
											"image": "bar:v2",
											"name": "bar"
										}
									]
								}
							}`),
					},
				},
			},
			want: true,
		}, {
			name: "namespace_not_in_pod",
			req: admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Namespace: "default",
					Object: runtime.RawExtension{
						Raw: []byte(
							`{
								"apiVersion": "v1",
								"kind": "Pod",
								"metadata": {
									"name": "foo"
								},
								"spec": {
									"containers": [
										{
											"image": "bar:v2",
											"name": "bar"
										}
									]
								}
							}`),
					},
				},
			},
			want: true,
		},
	}

	objs := []runtime.Object{}
	s := runtime.NewScheme()
	fakeClient := fake.NewFakeClientWithScheme(s, objs...)

	for _, test := range tests {
		handler := &CreateUpdatePodForSchedulingHandler{
			decoder: decoder,
		}
		handler.Setup(fakeClient)

		resp := handler.Handle(context.TODO(), test.req)

		if resp.AdmissionResponse.Allowed != test.want {
			t.Errorf("test %s failed to get resp %v, want %v", test.name, resp, test.want)
		}
	}
}

func TestAddScheduleInfoToPodWithReferencedDataset(t *testing.T) {

	type testCase struct {
		name                string
		in                  *corev1.Pod
		dataset             *datav1alpha1.Dataset
		refDataset          *datav1alpha1.Dataset
		pv                  *corev1.PersistentVolume
		pvc                 *corev1.PersistentVolumeClaim
		refPv               *corev1.PersistentVolume
		refPvc              *corev1.PersistentVolumeClaim
		fuse                *appsv1.DaemonSet
		wantErr             bool
		ihjectCacheAffinity bool
	}

	hostPathCharDev := corev1.HostPathCharDev
	hostPathDirectoryOrCreate := corev1.HostPathDirectoryOrCreate
	bTrue := true

	testcases := []testCase{
		{
			name: "no_serverless_pod_with_runtime",
			dataset: &datav1alpha1.Dataset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "done",
					Namespace: "big-data",
				}, Status: datav1alpha1.DatasetStatus{
					Phase: datav1alpha1.BoundDatasetPhase,
					Runtimes: []datav1alpha1.Runtime{
						{
							Type: common.JindoRuntime,
						},
					},
				},
			},
			refDataset: &datav1alpha1.Dataset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "done",
					Namespace: "ref",
				},
				Spec: datav1alpha1.DatasetSpec{
					Mounts: []datav1alpha1.Mount{
						{
							MountPoint: "dataset://big-data/done",
						},
					},
				}, Status: datav1alpha1.DatasetStatus{
					Phase: datav1alpha1.BoundDatasetPhase,
					Runtimes: []datav1alpha1.Runtime{
						{
							Type: common.ThinRuntime,
						},
					},
				},
			},
			in: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "ref",
					Labels: map[string]string{
						common.InjectServerfulFuse:    common.True,
						"fluid.io/dataset.done.sched": "required",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image: "test",
							Name:  "test",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "dataset",
									MountPath: "/data",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "dataset",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "done",
									ReadOnly:  true,
								},
							},
						},
					},
				},
			},
			pv: &corev1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: "big-data-done",
				},
				Spec: corev1.PersistentVolumeSpec{
					PersistentVolumeSource: corev1.PersistentVolumeSource{
						CSI: &corev1.CSIPersistentVolumeSource{
							Driver: "fuse.csi.fluid.io",
							VolumeAttributes: map[string]string{
								common.VolumeAttrFluidPath: "/runtime-mnt/jindo/big-data/done/jindofs-fuse",
								common.VolumeAttrMountType: common.JindoRuntime,
							},
						},
					},
				},
			},
			refPv: &corev1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ref-done",
				},
				Spec: corev1.PersistentVolumeSpec{
					PersistentVolumeSource: corev1.PersistentVolumeSource{
						CSI: &corev1.CSIPersistentVolumeSource{
							Driver: "fuse.csi.fluid.io",
							VolumeAttributes: map[string]string{
								common.VolumeAttrFluidPath: "/runtime-mnt/jindo/big-data/done/jindofs-fuse",
								common.VolumeAttrMountType: common.JindoRuntime,
							},
						},
					},
				},
			},
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "done",
					Namespace: "big-data",
				}, Spec: corev1.PersistentVolumeClaimSpec{
					VolumeName: "big-data-done",
				},
			},
			refPvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "done",
					Namespace: "ref",
					Labels: map[string]string{
						common.LabelAnnotationStorageCapacityPrefix + "ref-done": "true",
						common.LabelAnnotationDatasetReferringName:               "done",
						common.LabelAnnotationDatasetReferringNameSpace:          "big-data",
					},
				}, Spec: corev1.PersistentVolumeClaimSpec{
					VolumeName: "big-data-done",
				},
			},
			fuse: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "done-jindofs-fuse",
					Namespace: "big-data",
				},
				Spec: appsv1.DaemonSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "fuse",
									Args: []string{
										"-oroot_ns=jindo", "-okernel_cache", "-oattr_timeout=9000", "-oentry_timeout=9000",
									},
									Command: []string{"/entrypoint.sh"},
									Image:   "test",
									SecurityContext: &corev1.SecurityContext{
										Privileged: &bTrue,
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "data",
											MountPath: "/mnt/disk1",
										}, {
											Name:      "fuse-device",
											MountPath: "/dev/fuse",
										}, {
											Name:      "jindofs-fuse-mount",
											MountPath: "/jfs",
										},
									},
								},
							},
							Volumes: []corev1.Volume{
								{
									Name: "data",
									VolumeSource: corev1.VolumeSource{
										HostPath: &corev1.HostPathVolumeSource{
											Path: "/runtime_mnt/done",
										},
									}},
								{
									Name: "fuse-device",
									VolumeSource: corev1.VolumeSource{
										HostPath: &corev1.HostPathVolumeSource{
											Path: "/dev/fuse",
											Type: &hostPathCharDev,
										},
									},
								},
								{
									Name: "jindofs-fuse-mount",
									VolumeSource: corev1.VolumeSource{
										HostPath: &corev1.HostPathVolumeSource{
											Path: "/runtime-mnt/jindo/big-data/done",
											Type: &hostPathDirectoryOrCreate,
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr:             false,
			ihjectCacheAffinity: true,
		},
		{
			name: "no_serverless_pod_without_ref_pvc",
			dataset: &datav1alpha1.Dataset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "done-without-ref-pvc",
					Namespace: "big-data",
				}, Status: datav1alpha1.DatasetStatus{
					Phase: datav1alpha1.BoundDatasetPhase,
					Runtimes: []datav1alpha1.Runtime{
						{
							Type: common.JindoRuntime,
						},
					},
				},
			},
			refDataset: &datav1alpha1.Dataset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "done-without-ref-pvc",
					Namespace: "ref",
				},
				Spec: datav1alpha1.DatasetSpec{
					Mounts: []datav1alpha1.Mount{
						{
							MountPoint: "dataset://big-data/done",
						},
					},
				}, Status: datav1alpha1.DatasetStatus{
					Phase: datav1alpha1.BoundDatasetPhase,
					Runtimes: []datav1alpha1.Runtime{
						{
							Type: common.ThinRuntime,
						},
					},
				},
			},
			in: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "ref",
					Labels: map[string]string{
						common.InjectServerfulFuse:    common.True,
						"fluid.io/dataset.done.sched": "required",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image: "test",
							Name:  "test",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "dataset",
									MountPath: "/data",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "dataset",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "done-without-ref-pvc",
									ReadOnly:  true,
								},
							},
						},
					},
				},
			},
			pv: &corev1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: "big-data-done",
				},
				Spec: corev1.PersistentVolumeSpec{
					PersistentVolumeSource: corev1.PersistentVolumeSource{
						CSI: &corev1.CSIPersistentVolumeSource{
							Driver: "fuse.csi.fluid.io",
							VolumeAttributes: map[string]string{
								common.VolumeAttrFluidPath: "/runtime-mnt/jindo/big-data/done/jindofs-fuse",
								common.VolumeAttrMountType: common.JindoRuntime,
							},
						},
					},
				},
			},
			refPv: &corev1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ref-done-without-ref-pvc",
				},
				Spec: corev1.PersistentVolumeSpec{
					PersistentVolumeSource: corev1.PersistentVolumeSource{
						CSI: &corev1.CSIPersistentVolumeSource{
							Driver: "fuse.csi.fluid.io",
							VolumeAttributes: map[string]string{
								common.VolumeAttrFluidPath: "/runtime-mnt/jindo/big-data/done-without-ref-pvc/jindofs-fuse",
								common.VolumeAttrMountType: common.JindoRuntime,
							},
						},
					},
				},
			},
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "no-existt-ref-pvc",
					Namespace: "big-data",
				}, Spec: corev1.PersistentVolumeClaimSpec{
					VolumeName: "big-data-done",
				},
			},
			refPvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "done-without-ref-pvc",
					Namespace: "ref",
					Labels: map[string]string{
						common.LabelAnnotationStorageCapacityPrefix + "ref-done-without-ref-pvc": "true",
						common.LabelAnnotationDatasetReferringName:                               "done",
						common.LabelAnnotationDatasetReferringNameSpace:                          "big-data",
					},
				}, Spec: corev1.PersistentVolumeClaimSpec{
					VolumeName: "big-data-done",
				},
			},
			fuse: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "done-without-ref-pvc-jindofs-fuse",
					Namespace: "big-data",
				},
				Spec: appsv1.DaemonSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "fuse",
									Args: []string{
										"-oroot_ns=jindo", "-okernel_cache", "-oattr_timeout=9000", "-oentry_timeout=9000",
									},
									Command: []string{"/entrypoint.sh"},
									Image:   "test",
									SecurityContext: &corev1.SecurityContext{
										Privileged: &bTrue,
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "data",
											MountPath: "/mnt/disk1",
										}, {
											Name:      "fuse-device",
											MountPath: "/dev/fuse",
										}, {
											Name:      "jindofs-fuse-mount",
											MountPath: "/jfs",
										},
									},
								},
							},
							Volumes: []corev1.Volume{
								{
									Name: "data",
									VolumeSource: corev1.VolumeSource{
										HostPath: &corev1.HostPathVolumeSource{
											Path: "/runtime_mnt/done",
										},
									}},
								{
									Name: "fuse-device",
									VolumeSource: corev1.VolumeSource{
										HostPath: &corev1.HostPathVolumeSource{
											Path: "/dev/fuse",
											Type: &hostPathCharDev,
										},
									},
								},
								{
									Name: "jindofs-fuse-mount",
									VolumeSource: corev1.VolumeSource{
										HostPath: &corev1.HostPathVolumeSource{
											Path: "/runtime-mnt/jindo/big-data/done",
											Type: &hostPathDirectoryOrCreate,
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr:             true,
			ihjectCacheAffinity: true,
		},
	}
	tieredLocalityConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.TieredLocalityConfigMapName,
			Namespace: common.NamespaceFluidSystem,
		},
		Data: map[string]string{
			"tieredLocality": "" +
				"preferred:\n" +
				"  # fluid existed node affinity, the name can not be modified.\n" +
				"  - name: fluid.io/node\n" +
				"    weight: 100\n" +
				"required:\n" +
				"  - fluid.io/node\n",
		},
	}

	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	_ = datav1alpha1.AddToScheme(s)
	_ = appsv1.AddToScheme(s)
	for _, testcase := range testcases {
		objs := []runtime.Object{}
		objs = append(objs, testcase.fuse, testcase.pv, testcase.pvc, testcase.dataset,
			testcase.refDataset, testcase.refPv, testcase.refPvc, tieredLocalityConfigMap)

		runtime := &datav1alpha1.JindoRuntime{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "done",
				Namespace: "big-data",
			},
		}
		refRuntime := &datav1alpha1.ThinRuntime{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "done",
				Namespace: "ref",
			},
		}
		objs = append(objs, runtime, refRuntime)

		fakeClient := fake.NewFakeClientWithScheme(s, objs...)

		handler := &CreateUpdatePodForSchedulingHandler{
			Client: fakeClient,
		}

		err := handler.AddScheduleInfoToPod(testcase.in, testcase.in.Namespace)
		if testcase.wantErr {
			if err == nil {
				t.Errorf("testcase %s want error but get nil", testcase.name)
			}
			continue
		}

		// expect no err
		if err != nil {
			t.Errorf("testcase %s expect no error but get %v", testcase.name, testcase.name)
			continue
		}

		// check the mutate plugin
		injectMountPropagation := testcase.in.Spec.Containers[0].VolumeMounts[0].MountPropagation
		if *injectMountPropagation != corev1.MountPropagationHostToContainer {
			t.Errorf("testcase %s is failed due to error %v", testcase.name, fmt.Errorf("mount propagation is not inject"))
		}

		if testcase.ihjectCacheAffinity {
			cacheAffinity := testcase.in.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
			if cacheAffinity == nil {
				t.Errorf("testcase %s is failed due to error %v", testcase.name, fmt.Errorf("cache affinity is not inject"))
			}
		}
	}
}
