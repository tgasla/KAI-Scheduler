package reclaimjoborder

import (
	"testing"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/resource_info"
	rs "github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/proportion/resource_share"
	"github.com/stretchr/testify/assert"
)

func TestJobsComparatorByNodeStarvation_queue1_higher_allocated(t *testing.T) {
	// Create test queues - queue1 has higher over allocation score (0.4/0.5 = 0.8) than queue2 (0.2/0.5 = 0.4)
	queues := map[common_info.QueueID]*rs.QueueAttributes{
		"queue1": {
			UID: "queue1",
			QueueResourceShare: rs.QueueResourceShare{
				GPU: rs.ResourceShare{
					FairShare: 0.5,
					Allocated: 0.4,
				},
				CPU:    rs.EmptyResource(),
				Memory: rs.EmptyResource(),
			},
		},
		"queue2": {
			UID: "queue2",
			QueueResourceShare: rs.QueueResourceShare{
				GPU: rs.ResourceShare{
					FairShare: 0.5,
					Allocated: 0.2,
				},
				CPU:    rs.EmptyResource(),
				Memory: rs.EmptyResource(),
			},
		},
	}

	// Create test jobs
	jobs := map[common_info.PodGroupID]*podgroup_info.PodGroupInfo{
		"job1": {
			UID:   "job1",
			Queue: "queue1",
			PodInfos: map[common_info.PodID]*pod_info.PodInfo{
				"pod1": {
					UID:      "pod1",
					Job:      "job1",
					NodeName: "node1",
					ResReq:   resource_info.NewResourceRequirementsWithGpus(2),
				},
			},
		},
		"job2": {
			UID:   "job2",
			Queue: "queue2",
			PodInfos: map[common_info.PodID]*pod_info.PodInfo{
				"pod2": {
					UID:      "pod2",
					Job:      "job2",
					NodeName: "node2",
					ResReq:   resource_info.NewResourceRequirementsWithGpus(2),
				},
			},
		},
	}

	// Create test nodes
	nodes := map[string]*node_info.NodeInfo{
		"node1": {
			Name: "node1",
			PodInfos: map[common_info.PodID]*pod_info.PodInfo{
				"pod1": jobs["job1"].PodInfos["pod1"],
			},
			Allocatable: resource_info.NewResource(0, 0, 2),
		},
		"node2": {
			Name: "node2",
			PodInfos: map[common_info.PodID]*pod_info.PodInfo{
				"pod2": jobs["job2"].PodInfos["pod2"],
			},
			Allocatable: resource_info.NewResource(0, 0, 2),
		},
	}

	// Get the comparator function
	comparator := JobsComparatorByNodeStarvation(queues, nodes, jobs)

	// Test case 1: Compare jobs with different starvation scores
	// queue1 has higher starvation score (0.5/0.2 = 2.5) than queue2 (0.5/0.4 = 1.25)
	// Therefore job1 should be preferred over job2
	result := comparator(jobs["job1"], jobs["job2"])
	assert.Equal(t, 1, result, "Job1 should be preferred over Job2 due to higher starvation score")

	// Test case 2: Reverse comparison
	result = comparator(jobs["job2"], jobs["job1"])
	assert.Equal(t, -1, result, "Job2 should be less preferred than Job1")

	// Test case 3: Compare same job (should be equal)
	result = comparator(jobs["job1"], jobs["job1"])
	assert.Equal(t, 0, result, "Same job should be equal")

	// Test case 4: Compare jobs with no assigned nodes
	job3 := &podgroup_info.PodGroupInfo{
		UID:   "job3",
		Queue: "queue1",
		PodInfos: map[common_info.PodID]*pod_info.PodInfo{
			"pod3": {
				UID:      "pod3",
				Job:      "job3",
				NodeName: "",
				ResReq:   resource_info.NewResourceRequirementsWithGpus(2),
			},
		},
	}
	jobs["job3"] = job3
	result = comparator(job3, jobs["job1"])
	assert.Equal(t, -1, result, "Job with no assigned nodes should be less preferred")
}

func TestJobsComparatorByNodeStarvation_queue1_lower_fair_share(t *testing.T) {
	// Create test queues - queue1 has higher over allocation score (0.2/0.5 = 0.4) than queue2 (0.2/2 = 0.1)
	queues := map[common_info.QueueID]*rs.QueueAttributes{
		"queue1": {
			UID: "queue1",
			QueueResourceShare: rs.QueueResourceShare{
				GPU: rs.ResourceShare{
					FairShare: 0.5,
					Allocated: 0.2,
				},
				CPU:    rs.EmptyResource(),
				Memory: rs.EmptyResource(),
			},
		},
		"queue2": {
			UID: "queue2",
			QueueResourceShare: rs.QueueResourceShare{
				GPU: rs.ResourceShare{
					FairShare: 2,
					Allocated: 0.2,
				},
				CPU:    rs.EmptyResource(),
				Memory: rs.EmptyResource(),
			},
		},
	}

	// Create test jobs
	jobs := map[common_info.PodGroupID]*podgroup_info.PodGroupInfo{
		"job1": {
			UID:   "job1",
			Queue: "queue1",
			PodInfos: map[common_info.PodID]*pod_info.PodInfo{
				"pod1": {
					UID:      "pod1",
					Job:      "job1",
					NodeName: "node1",
					ResReq:   resource_info.NewResourceRequirementsWithGpus(2),
				},
			},
		},
		"job2": {
			UID:   "job2",
			Queue: "queue2",
			PodInfos: map[common_info.PodID]*pod_info.PodInfo{
				"pod2": {
					UID:      "pod2",
					Job:      "job2",
					NodeName: "node2",
					ResReq:   resource_info.NewResourceRequirementsWithGpus(2),
				},
			},
		},
	}

	// Create test nodes
	nodes := map[string]*node_info.NodeInfo{
		"node1": {
			Name: "node1",
			PodInfos: map[common_info.PodID]*pod_info.PodInfo{
				"pod1": jobs["job1"].PodInfos["pod1"],
			},
			Allocatable: resource_info.NewResource(0, 0, 2),
		},
		"node2": {
			Name: "node2",
			PodInfos: map[common_info.PodID]*pod_info.PodInfo{
				"pod2": jobs["job2"].PodInfos["pod2"],
			},
			Allocatable: resource_info.NewResource(0, 0, 2),
		},
	}

	// Get the comparator function
	comparator := JobsComparatorByNodeStarvation(queues, nodes, jobs)

	// Test case 1: Compare jobs with different starvation scores
	// queue1 has higher starvation score (0.5/0.2 = 2.5) than queue2 (0.5/0.4 = 1.25)
	// Therefore job1 should be preferred over job2
	result := comparator(jobs["job1"], jobs["job2"])
	assert.Equal(t, 1, result, "Job1 should be preferred over Job2 due to higher starvation score")

	// Test case 2: Reverse comparison
	result = comparator(jobs["job2"], jobs["job1"])
	assert.Equal(t, -1, result, "Job2 should be less preferred than Job1")

	// Test case 3: Compare same job (should be equal)
	result = comparator(jobs["job1"], jobs["job1"])
	assert.Equal(t, 0, result, "Same job should be equal")

	// Test case 4: Compare jobs with no assigned nodes
	job3 := &podgroup_info.PodGroupInfo{
		UID:   "job3",
		Queue: "queue1",
		PodInfos: map[common_info.PodID]*pod_info.PodInfo{
			"pod3": {
				UID:      "pod3",
				Job:      "job3",
				NodeName: "",
				ResReq:   resource_info.NewResourceRequirementsWithGpus(2),
			},
		},
	}
	jobs["job3"] = job3
	result = comparator(job3, jobs["job1"])
	assert.Equal(t, -1, result, "Job with no assigned nodes should be less preferred")
}
