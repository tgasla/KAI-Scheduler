// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package deployment

import (
	"testing"

	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/constants"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestGetPodGroupMetadata(t *testing.T) {
	deployment := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "Deployment",
			"apiVersion": "apps/v1",
			"metadata": map[string]interface{}{
				"name":      "test_deployment",
				"namespace": "test_namespace",
				"uid":       "1",
				"labels": map[string]interface{}{
					"test_label": "test_value",
				},
				"annotations": map[string]interface{}{
					"test_annotation": "test_value",
				},
			},
			"spec": map[string]interface{}{
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"runai/queue": "test_queue",
						},
					},
					"spec": map[string]interface{}{
						"schedulerName": "kai-scheduler",
						"containers": []map[string]interface{}{{
							"name": "container",
						}},
					},
				},
			},
		},
	}

	pod1 := &v1.Pod{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-1",
			Namespace: "test_namespace",
			Labels: map[string]string{
				"runai/queue": "test_queue",
			},
			UID: "3",
		},
		Spec:   v1.PodSpec{},
		Status: v1.PodStatus{},
	}

	pod2 := &v1.Pod{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-2",
			Namespace: "test_namespace",
			Labels: map[string]string{
				"runai/queue": "test_queue",
			},
			UID: "4",
		},
		Spec:   v1.PodSpec{},
		Status: v1.PodStatus{},
	}

	metadata, err := GetPodGroupMetadata(deployment, pod1)
	assert.Nil(t, err)
	assert.Equal(t, "pg-pod-1-3", metadata.Name)
	assert.Equal(t, constants.InferencePriorityClass, metadata.PriorityClassName)
	assert.Equal(t, "test_queue", metadata.Queue)

	metadata2, err := GetPodGroupMetadata(deployment, pod2)
	assert.Nil(t, err)
	// assert that the second pod got a different podgroup
	assert.Equal(t, "pg-pod-2-4", metadata2.Name)
	assert.Equal(t, constants.InferencePriorityClass, metadata2.PriorityClassName)
	assert.Equal(t, "test_queue", metadata2.Queue)
}
