// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package podgroup

import (
	"context"

	enginev2alpha2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2alpha2"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/constants"
)

type Handler struct {
	client      client.Client
	nodePoolKey string
}

func NewHandler(client client.Client, nodePoolKey string) *Handler {
	return &Handler{client: client, nodePoolKey: nodePoolKey}
}

func (h *Handler) ApplyToCluster(ctx context.Context, pgMetadata Metadata) error {
	newPodGroup := h.createPodGroupForMetadata(pgMetadata)

	var err error
	oldPodGroup := &enginev2alpha2.PodGroup{}
	key := types.NamespacedName{
		Namespace: pgMetadata.Namespace,
		Name:      pgMetadata.Name,
	}
	err = h.client.Get(ctx, key, oldPodGroup)
	if err != nil {
		if errors.IsNotFound(err) {
			err = h.client.Create(ctx, newPodGroup)
			return err
		}
		return err
	}

	newPodGroup = h.ignoreFields(oldPodGroup, newPodGroup)

	// If we got here then oldPodGroup exists - update if necessary
	if podGroupsEqual(oldPodGroup, newPodGroup) {
		// The objects are equal - no need to update.
		return nil
	}

	updatePodGroup(oldPodGroup, newPodGroup)

	err = h.client.Update(ctx, oldPodGroup)
	return err
}

func (h *Handler) ignoreFields(oldPodGroup, newPodGroup *enginev2alpha2.PodGroup) *enginev2alpha2.PodGroup {
	// to avoid overriding the fields that the pod-group-assigner is responsible for
	newPodGroupCopy := newPodGroup.DeepCopy()

	newPodGroupCopy.Spec.MarkUnschedulable = oldPodGroup.Spec.MarkUnschedulable
	newPodGroupCopy.Spec.SchedulingBackoff = oldPodGroup.Spec.SchedulingBackoff
	newPodGroupCopy.Spec.Queue = oldPodGroup.Spec.Queue

	if newPodGroupCopy.Labels == nil {
		newPodGroupCopy.Labels = map[string]string{}
	}
	nodePoolName, found := oldPodGroup.Labels[h.nodePoolKey]
	if found {
		newPodGroupCopy.Labels[h.nodePoolKey] = nodePoolName
	} else {
		delete(newPodGroupCopy.Labels, h.nodePoolKey)
	}

	queueName, found := oldPodGroup.Labels[constants.QueueLabelKey]
	if found {
		newPodGroupCopy.Labels[constants.QueueLabelKey] = queueName
	}

	return newPodGroupCopy
}

func (h *Handler) createPodGroupForMetadata(podGroupMetadata Metadata) *enginev2alpha2.PodGroup {
	return &enginev2alpha2.PodGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:        podGroupMetadata.Name,
			Namespace:   podGroupMetadata.Namespace,
			Labels:      podGroupMetadata.Labels,
			Annotations: podGroupMetadata.Annotations,
			OwnerReferences: []metav1.OwnerReference{
				podGroupMetadata.Owner,
			},
		},
		Spec: enginev2alpha2.PodGroupSpec{
			MinMember:         podGroupMetadata.MinAvailable,
			Queue:             podGroupMetadata.Queue,
			PriorityClassName: podGroupMetadata.PriorityClassName,
		},
	}
}
