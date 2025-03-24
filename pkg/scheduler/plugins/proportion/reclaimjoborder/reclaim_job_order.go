package reclaimjoborder

import (
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	rs "github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/proportion/resource_share"
)

const (
	noAllocatedStarvationFactor = 100
)

func JobsComparatorByNodeStarvation(
	queues map[common_info.QueueID]*rs.QueueAttributes,
	nodes map[string]*node_info.NodeInfo,
	jobs map[common_info.PodGroupID]*podgroup_info.PodGroupInfo,
) common_info.CompareFn {
	queueOverAllocationScore := map[common_info.QueueID]float64{} // larger score for node means "more starved"
	for _, queue := range queues {
		allocated := queue.GetAllocatedShare()
		fairShare := queue.GetFairShare()
		// TODO: change to DRF score later?
		if fairShare[rs.GpuResource] > 0 {
			queueOverAllocationScore[queue.UID] = allocated[rs.GpuResource] / fairShare[rs.GpuResource]
		} else {
			queueOverAllocationScore[queue.UID] = allocated[rs.GpuResource] * noAllocatedStarvationFactor
		}
	}

	nodeOverAllocationScore := map[string]float64{}
	for _, node := range nodes {
		score := 0.0
		for _, podInfo := range node.PodInfos {
			podGpus := podInfo.ResReq.GPUs()
			if podGpus > 0 && podInfo.Job != "" {
				score += queueOverAllocationScore[jobs[podInfo.Job].Queue] * podGpus
			}
		}
		nodeOverAllocationScore[node.Name] = score
	}

	return func(l, r interface{}) int {
		lJob := l.(*podgroup_info.PodGroupInfo)
		rJob := r.(*podgroup_info.PodGroupInfo)

		lJobNodesScore := 0.0
		rJobNodesScore := 0.0
		for _, podInfo := range lJob.PodInfos {
			if podInfo.NodeName != "" {
				//gpuShareOnNode := podInfo.ResReq.GPUs() / nodes[podInfo.NodeName].Allocatable.GPUs()
				lJobNodesScore += nodeOverAllocationScore[podInfo.NodeName]
			}
		}
		lJobNodesScore = lJobNodesScore / float64(len(lJob.PodInfos))
		for _, podInfo := range rJob.PodInfos {
			if podInfo.NodeName != "" {
				//gpuShareOnNode := podInfo.ResReq.GPUs() / nodes[podInfo.NodeName].Allocatable.GPUs()
				rJobNodesScore += nodeOverAllocationScore[podInfo.NodeName]
			}
		}
		rJobNodesScore = rJobNodesScore / float64(len(rJob.PodInfos))
		if lJobNodesScore > rJobNodesScore {
			return 1
		} else if lJobNodesScore < rJobNodesScore {
			return -1
		} else {
			return 0
		}
	}
}
