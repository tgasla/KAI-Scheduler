// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package podgroup_info

import (
	"crypto/sha256"
	"fmt"
	"time"

	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	enginev2alpha2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2alpha2"
	commonconstants "github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_status"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/resource_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/log"
)

const (
	OverCapacity        = "OverCapacity"
	PodSchedulingErrors = "PodSchedulingErrors"
)

type JobRequirement struct {
	GPU      float64 `json:"gpu,omitempty"`
	MilliCPU float64 `json:"milliCpu,omitempty"`
	Memory   float64 `json:"memory,omitempty"`
}

type StalenessInfo struct {
	TimeStamp *time.Time `json:"timeStamp,omitempty"`
	Stale     bool       `json:"stale,omitempty"`
}

type PodGroupInfo struct {
	UID common_info.PodGroupID `json:"uid,omitempty"`

	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`

	Queue common_info.QueueID `json:"queue,omitempty"`

	Priority int32 `json:"priority,omitempty"`

	MinAvailable int32 `json:"minAvailable,omitempty"`

	JobFitErrors   enginev2alpha2.UnschedulableExplanations `json:"jobFitErrors,omitempty"`
	NodesFitErrors map[common_info.PodID]*common_info.FitErrors `json:"nodesFitErrors,omitempty"`

	// All tasks of the Job.
	PodStatusIndex map[pod_status.PodStatus]pod_info.PodsMap `json:"podStatusIndex,omitempty"`
	PodInfos       pod_info.PodsMap `json:"podInfos,omitempty"`

	Allocated *resource_info.Resource `json:"allocated,omitempty"`

	CreationTimestamp metav1.Time `json:"creationTimestamp,omitempty"`
	PodGroup          *enginev2alpha2.PodGroup `json:"podGroup,omitempty"`
	PodGroupUID       types.UID `json:"podGroupUid,omitempty"`

	// TODO(k82cn): keep backward compatibility, removed it when v1alpha1 finalized.
	PDB *policyv1.PodDisruptionBudget `json:"pdb,omitempty"`

	StalenessInfo `json:"stalenessInfo,omitempty"`

	schedulingConstraintsSignature common_info.SchedulingConstraintsSignature `json:"schedulingConstraintsSignature,omitempty"`
}

func NewPodGroupInfo(uid common_info.PodGroupID, tasks ...*pod_info.PodInfo) *PodGroupInfo {
	podGroupInfo := &PodGroupInfo{
		UID:          uid,
		MinAvailable: 0,
		Allocated:    resource_info.EmptyResource(),

		JobFitErrors:   make(enginev2alpha2.UnschedulableExplanations, 0),
		NodesFitErrors: make(map[common_info.PodID]*common_info.FitErrors),

		PodStatusIndex: map[pod_status.PodStatus]pod_info.PodsMap{},
		PodInfos:       pod_info.PodsMap{},

		StalenessInfo: StalenessInfo{
			TimeStamp: nil,
			Stale:     false,
		},
	}

	for _, task := range tasks {
		podGroupInfo.AddTaskInfo(task)
	}

	return podGroupInfo
}

func (podGroupInfo *PodGroupInfo) IsPreemptibleJob(isInferencePreemptible bool) bool {
	if isInferencePreemptible {
		if podGroupInfo.Priority == constants.PriorityInferenceNumber {
			return true
		}
	}

	return podGroupInfo.Priority < constants.PriorityBuildNumber
}

func (podGroupInfo *PodGroupInfo) SetPodGroup(pg *enginev2alpha2.PodGroup) {
	podGroupInfo.Name = pg.Name
	podGroupInfo.Namespace = pg.Namespace
	podGroupInfo.MinAvailable = pg.Spec.MinMember
	podGroupInfo.Queue = common_info.QueueID(pg.Spec.Queue)
	podGroupInfo.CreationTimestamp = pg.GetCreationTimestamp()
	podGroupInfo.PodGroup = pg
	podGroupInfo.PodGroupUID = pg.UID

	if pg.Annotations[commonconstants.StalePodgroupTimeStamp] != "" {
		staleTimeStamp, err := time.Parse(time.RFC3339, pg.Annotations[commonconstants.StalePodgroupTimeStamp])
		if err != nil {
			log.InfraLogger.V(7).Warnf("Failed to parse stale timestamp for podgroup <%s/%s> err: %v",
				podGroupInfo.Namespace, podGroupInfo.Name, err)
		} else {
			podGroupInfo.StalenessInfo.TimeStamp = &staleTimeStamp
			podGroupInfo.StalenessInfo.Stale = true
		}
	}

	log.InfraLogger.V(7).Infof(
		"SetPodGroup. podGroupName=<%s>, PodGroupUID=<%s> podGroupInfo.PodGroupIndex=<%d>",
		podGroupInfo.Name, podGroupInfo.PodGroupUID)
}

func (podGroupInfo *PodGroupInfo) SetPDB(pdb *policyv1.PodDisruptionBudget) {
	podGroupInfo.Name = pdb.Name
	if pdb.Spec.MinAvailable != nil {
		podGroupInfo.MinAvailable = pdb.Spec.MinAvailable.IntVal
	}
	podGroupInfo.Namespace = pdb.Namespace

	podGroupInfo.CreationTimestamp = pdb.GetCreationTimestamp()
	podGroupInfo.PDB = pdb
}

func (podGroupInfo *PodGroupInfo) addTaskIndex(ti *pod_info.PodInfo) {
	if _, found := podGroupInfo.PodStatusIndex[ti.Status]; !found {
		podGroupInfo.PodStatusIndex[ti.Status] = pod_info.PodsMap{}
	}

	podGroupInfo.PodStatusIndex[ti.Status][ti.UID] = ti
}

func (podGroupInfo *PodGroupInfo) AddTaskInfo(ti *pod_info.PodInfo) {
	podGroupInfo.PodInfos[ti.UID] = ti
	podGroupInfo.addTaskIndex(ti)

	if pod_status.AllocatedStatus(ti.Status) {
		podGroupInfo.Allocated.AddResourceRequirements(ti.ResReq)
	}
}

func (podGroupInfo *PodGroupInfo) UpdateTaskStatus(task *pod_info.PodInfo, status pod_status.PodStatus) error {
	// Remove the task from the task list firstly
	if err := podGroupInfo.DeleteTaskInfo(task); err != nil {
		return err
	}

	// Update task's status to the target status
	task.Status = status
	podGroupInfo.AddTaskInfo(task)

	return nil
}

func (podGroupInfo *PodGroupInfo) deleteTaskIndex(ti *pod_info.PodInfo) {
	if tasks, found := podGroupInfo.PodStatusIndex[ti.Status]; found {
		delete(tasks, ti.UID)

		if len(tasks) == 0 {
			delete(podGroupInfo.PodStatusIndex, ti.Status)
		}
	}
}

func (podGroupInfo *PodGroupInfo) GetActiveAllocatedTasks() []*pod_info.PodInfo {
	var tasksToAllocate []*pod_info.PodInfo
	for _, task := range podGroupInfo.PodInfos {
		if pod_status.IsActiveAllocatedStatus(task.Status) {
			tasksToAllocate = append(tasksToAllocate, task)
		}
	}
	return tasksToAllocate
}

func (podGroupInfo *PodGroupInfo) GetActivelyRunningTasks() []*pod_info.PodInfo {
	var tasks []*pod_info.PodInfo
	for _, task := range podGroupInfo.PodInfos {
		if pod_status.IsActiveUsedStatus(task.Status) {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

func (podGroupInfo *PodGroupInfo) DeleteTaskInfo(ti *pod_info.PodInfo) error {
	task, found := podGroupInfo.PodInfos[ti.UID]
	if !found {
		return fmt.Errorf("failed to find task <%v/%v> in job <%v/%v>",
			ti.Namespace, ti.Name, podGroupInfo.Namespace, podGroupInfo.Name)
	}

	if pod_status.AllocatedStatus(task.Status) {
		podGroupInfo.Allocated.SubResourceRequirements(task.ResReq)
	}

	podGroupInfo.deleteTaskIndex(ti)
	taskClone := task.Clone()
	taskClone.Status = pod_status.Deleted
	podGroupInfo.PodInfos[taskClone.UID] = taskClone
	return nil

}

func (podGroupInfo *PodGroupInfo) GetNumAliveTasks() int {
	numTasks := 0
	for _, task := range podGroupInfo.PodInfos {
		if pod_status.IsAliveStatus(task.Status) {
			numTasks += 1
		}
	}
	return numTasks
}

func (podGroupInfo *PodGroupInfo) GetNumActiveUsedTasks() int {
	numTasks := 0
	for _, task := range podGroupInfo.PodInfos {
		if pod_status.IsActiveUsedStatus(task.Status) {
			numTasks += 1
		}
	}
	return numTasks
}

func (podGroupInfo *PodGroupInfo) GetPendingTasks() []*pod_info.PodInfo {
	var pendingTasks []*pod_info.PodInfo
	for _, task := range podGroupInfo.PodInfos {
		if task.Status == pod_status.Pending {
			pendingTasks = append(pendingTasks, task)
		}
	}
	return pendingTasks

}

func (podGroupInfo *PodGroupInfo) GetNumPendingTasks() int {
	return len(podGroupInfo.PodStatusIndex[pod_status.Pending])
}

func (podGroupInfo *PodGroupInfo) GetAliveTasksRequestedGPUs() float64 {
	tasksTotalRequestedGPUs := float64(0)
	for _, task := range podGroupInfo.PodInfos {
		if pod_status.IsAliveStatus(task.Status) {
			tasksTotalRequestedGPUs += task.ResReq.GPUs()
		}
	}

	return tasksTotalRequestedGPUs
}

func (podGroupInfo *PodGroupInfo) GetTasksActiveAllocatedReqResource() *resource_info.Resource {
	tasksTotalRequestedResource := resource_info.EmptyResource()
	for _, task := range podGroupInfo.PodInfos {
		if pod_status.IsActiveAllocatedStatus(task.Status) {
			tasksTotalRequestedResource.AddResourceRequirements(task.ResReq)
		}
	}

	return tasksTotalRequestedResource
}

func (podGroupInfo *PodGroupInfo) IsReadyForScheduling() bool {
	numAliveTasks := podGroupInfo.GetNumAliveTasks()
	return int32(numAliveTasks) >= podGroupInfo.MinAvailable
}

func (podGroupInfo *PodGroupInfo) IsPodGroupStale() bool {
	if podGroupInfo.PodStatusIndex[pod_status.Succeeded] != nil {
		return false
	}

	activeUsedTasks := int32(podGroupInfo.GetNumActiveUsedTasks())
	return activeUsedTasks > 0 && activeUsedTasks < podGroupInfo.MinAvailable
}

func (podGroupInfo *PodGroupInfo) ShouldPipelineJob() bool {
	hasPipelinedTask := false
	activeAllocatedTasksCount := 0
	for _, task := range podGroupInfo.PodInfos {
		if task.Status == pod_status.Pipelined {
			log.InfraLogger.V(7).Infof("task: <%v/%v> was pipelined to node: <%v>",
				task.Namespace, task.Name, task.NodeName)
			hasPipelinedTask = true
		} else if pod_status.IsActiveAllocatedStatus(task.Status) {
			activeAllocatedTasksCount += 1
		}
	}
	// If the job has already MinAvailable tasks active allocated (but not pipelined),
	//  then we shouldn't convert non-pipelined tasks to pipeline.
	return hasPipelinedTask && activeAllocatedTasksCount < int(podGroupInfo.MinAvailable)
}

func (podGroupInfo *PodGroupInfo) Clone() *PodGroupInfo {
	return podGroupInfo.CloneWithTasks(maps.Values(podGroupInfo.PodInfos))
}

func (podGroupInfo *PodGroupInfo) CloneWithTasks(tasks []*pod_info.PodInfo) *PodGroupInfo {
	info := &PodGroupInfo{
		UID:       podGroupInfo.UID,
		Name:      podGroupInfo.Name,
		Namespace: podGroupInfo.Namespace,
		Queue:     podGroupInfo.Queue,
		Priority:  podGroupInfo.Priority,

		MinAvailable: podGroupInfo.MinAvailable,
		Allocated:    resource_info.EmptyResource(),

		JobFitErrors:   make(enginev2alpha2.UnschedulableExplanations, 0),
		NodesFitErrors: make(map[common_info.PodID]*common_info.FitErrors),

		PDB:         podGroupInfo.PDB,
		PodGroup:    podGroupInfo.PodGroup,
		PodGroupUID: podGroupInfo.PodGroupUID,

		PodStatusIndex: map[pod_status.PodStatus]pod_info.PodsMap{},
		PodInfos:       pod_info.PodsMap{},
	}

	podGroupInfo.CreationTimestamp.DeepCopyInto(&info.CreationTimestamp)

	for _, task := range tasks {
		info.AddTaskInfo(task.Clone())
	}

	return info
}

func (podGroupInfo PodGroupInfo) String() string {
	res := ""

	i := 0
	for _, task := range podGroupInfo.PodInfos {
		res = res + fmt.Sprintf("\n\t %d: %v", i, task)
		i++
	}

	return fmt.Sprintf("Job (%v): namespace %v (%v), name %v, minAvailable %d, podGroup %+v",
		podGroupInfo.UID, podGroupInfo.Namespace, podGroupInfo.Queue, podGroupInfo.Name, podGroupInfo.MinAvailable, podGroupInfo.PodGroup) + res
}

func (podGroupInfo *PodGroupInfo) SetTaskFitError(task *pod_info.PodInfo, fitErrors *common_info.FitErrors) {
	existingFitErrors, found := podGroupInfo.NodesFitErrors[task.UID]
	if found {
		existingFitErrors.AddNodeErrors(fitErrors)
	} else {
		podGroupInfo.NodesFitErrors[task.UID] = fitErrors
	}
}

func (podGroupInfo *PodGroupInfo) SetJobFitError(reason enginev2alpha2.UnschedulableReason, message string, details *enginev2alpha2.UnschedulableExplanationDetails) {
	podGroupInfo.JobFitErrors = append(podGroupInfo.JobFitErrors, enginev2alpha2.UnschedulableExplanation{
		Reason:  reason,
		Message: message,
		Details: details,
	})
}

func (podGroupInfo *PodGroupInfo) GetSchedulingConstraintsSignature() common_info.SchedulingConstraintsSignature {
	if podGroupInfo.schedulingConstraintsSignature != "" {
		return podGroupInfo.schedulingConstraintsSignature
	}

	key := podGroupInfo.generateSchedulingConstraintsSignature()

	podGroupInfo.schedulingConstraintsSignature = key
	return key
}

func (podGroupInfo *PodGroupInfo) generateSchedulingConstraintsSignature() common_info.SchedulingConstraintsSignature {
	hash := sha256.New()
	var signatures []common_info.SchedulingConstraintsSignature

	for _, pod := range podGroupInfo.PodInfos {
		if pod_status.IsActiveAllocatedStatus(pod.Status) {
			continue
		}

		key := pod.GetSchedulingConstraintsSignature()
		signatures = append(signatures, key)
	}
	slices.Sort(signatures)

	for _, signature := range signatures {
		hash.Write([]byte(signature))
	}

	return common_info.SchedulingConstraintsSignature(fmt.Sprintf("%x", hash.Sum(nil)))
}

func (jr *JobRequirement) Get(resourceName v1.ResourceName) float64 {
	switch resourceName {
	case v1.ResourceCPU:
		return jr.MilliCPU
	case v1.ResourceMemory:
		return jr.Memory
	case resource_info.GPUResourceName:
		return jr.GPU
	default:
		return 0
	}
}
