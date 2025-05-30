// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"fmt"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/resource_info"
)

const (
	NodePodNumberExceeded = "node(s) pod number exceeded"
)

func GetBuildOverCapacityMessageForQueue(queueName string, resourceName string, deserved, used float64,
	requiredResources *podgroup_info.JobRequirement) string {
	details := getOverCapacityMessageDetails(queueName, resourceName, deserved, used, requiredResources)
	return fmt.Sprintf("Non-preemptible workload is over quota. "+
		"%v Use a preemptible workload to go over quota.", details)
}

func getOverCapacityMessageDetails(queueName, resourceName string, deserved, used float64,
	requiredResources *podgroup_info.JobRequirement) string {
	switch resourceName {
	case GpuResource:
		return fmt.Sprintf("Workload requested %v GPUs, but %s quota is %v GPUs, "+
			"while %v GPUs are already allocated for non-preemptible pods.",
			requiredResources.GPU,
			queueName,
			resource_info.HumanizeResource(deserved, 1),
			resource_info.HumanizeResource(used, 1),
		)
	case CpuResource:
		return fmt.Sprintf("Workload requested %v CPU cores, but %s quota is %v cores, "+
			"while %v cores are already allocated for non-preemptible pods.",
			requiredResources.MilliCPU,
			queueName,
			resource_info.HumanizeResource(deserved, resource_info.MilliCPUToCores),
			resource_info.HumanizeResource(used, resource_info.MilliCPUToCores),
		)
	case MemoryResource:
		return fmt.Sprintf("Workload requested %v GB memory, but %s quota is %v GB, "+
			"while %v GB are already allocated for non-preemptible pods.",
			requiredResources.Memory,
			queueName,
			resource_info.HumanizeResource(deserved, resource_info.MemoryToGB),
			resource_info.HumanizeResource(used, resource_info.MemoryToGB),
		)
	default:
		return ""
	}
}

func GetJobOverMaxAllowedMessageForQueue(
	queueName string, resourceName string, maxAllowed, used, requested float64,
) string {
	var resourceNameStr, details string
	switch resourceName {
	case GpuResource:
		resourceNameStr = "GPUs"
		details = fmt.Sprintf("Limit is %s GPUs, currently %s GPUs allocated and workload requested %s GPUs",
			resource_info.HumanizeResource(maxAllowed, 1),
			resource_info.HumanizeResource(used, 1),
			resource_info.HumanizeResource(requested, 1))
	case CpuResource:
		resourceNameStr = "CPU cores"
		details = fmt.Sprintf("Limit is %s cores, currently %s cores allocated and workload requested %s cores",
			resource_info.HumanizeResource(maxAllowed, resource_info.MilliCPUToCores),
			resource_info.HumanizeResource(used, resource_info.MilliCPUToCores),
			resource_info.HumanizeResource(requested, resource_info.MilliCPUToCores))
	case MemoryResource:
		resourceNameStr = "memory"
		details = fmt.Sprintf("Limit is %s GB, currently %s GB allocated and workload requested %s GB",
			resource_info.HumanizeResource(maxAllowed, resource_info.MemoryToGB),
			resource_info.HumanizeResource(used, resource_info.MemoryToGB),
			resource_info.HumanizeResource(requested, resource_info.MemoryToGB))
	}
	return fmt.Sprintf("%s quota has reached the allowable limit of %s. %s",
		queueName, resourceNameStr, details)
}

func GetGangEvictionMessage(taskNamespace, taskName string, minimum int32) string {
	return fmt.Sprintf(
		"Workload doesn't have the minimum required number of pods (%d), evicting remaining pod: %s/%s",
		minimum, taskNamespace, taskName)
}

func GetPreemptMessage(preemptorJob *podgroup_info.PodGroupInfo, preempteeTask *pod_info.PodInfo) string {
	return fmt.Sprintf("Pod %s/%s was preempted by higher priority workload %s/%s", preempteeTask.Namespace,
		preempteeTask.Name, preemptorJob.Namespace, preemptorJob.Name)
}

func GetReclaimMessage(reclaimeeTask *pod_info.PodInfo, reclaimerJob *podgroup_info.PodGroupInfo) string {
	return fmt.Sprintf("Pod %s/%s was preempted by workload %s/%s.",
		reclaimeeTask.Namespace, reclaimeeTask.Name, reclaimerJob.Namespace, reclaimerJob.Name)
}

func GetReclaimQueueDetailsMessage(queueName string, queueAllocated *resource_info.ResourceRequirements,
	queueQuota *resource_info.ResourceRequirements, queueFairShare *resource_info.ResourceRequirements, queuePriority int) string {
	return fmt.Sprintf("%s uses <%v> with a quota of <%v>, fair-share of <%v> and queue priority of <%d>.",
		queueName, queueAllocated.String(), queueQuota.String(), queueFairShare.String(), queuePriority)
}

func GetConsolidateMessage(preempteeTask *pod_info.PodInfo) string {
	return fmt.Sprintf(
		"Pod %s/%s was preempted and rescheduled due to bin packing (resource consolidation) procedure",
		preempteeTask.Namespace, preempteeTask.Name)
}
