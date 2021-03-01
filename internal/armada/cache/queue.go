package cache

import (
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/G-Research/armada/internal/armada/repository"
	"github.com/G-Research/armada/internal/armada/scheduling"
	"github.com/G-Research/armada/internal/common"
	"github.com/G-Research/armada/pkg/api"
)

type empty struct{}
type stringSet map[string]empty

type QueueCache struct {
	queueRepository          repository.QueueRepository
	jobRepository            repository.JobRepository
	schedulingInfoRepository repository.SchedulingInfoRepository

	refreshMutex           sync.Mutex
	queuedResources        map[string]map[string]common.ComputeResourcesFloat
	queueNonMatchingJobIds map[string]map[string]stringSet
}

func NewQueueCache(
	queueRepository repository.QueueRepository,
	jobRepository repository.JobRepository,
	schedulingInfoRepository repository.SchedulingInfoRepository,
) *QueueCache {
	collector := &QueueCache{
		queueRepository:          queueRepository,
		jobRepository:            jobRepository,
		schedulingInfoRepository: schedulingInfoRepository,
		queuedResources:          map[string]map[string]common.ComputeResourcesFloat{},
		queueNonMatchingJobIds:   map[string]map[string]stringSet{}}

	return collector
}

func (c *QueueCache) Refresh() {
	queues, e := c.queueRepository.GetAllQueues()
	if e != nil {
		log.Errorf("Error while getting queue metrics %s", e)
		return
	}

	clusterInfo, e := c.schedulingInfoRepository.GetClusterSchedulingInfo()
	if e != nil {
		log.Errorf("Error while getting cluster reports %s", e)
		return
	}

	activeClusterInfo := scheduling.FilterActiveClusterSchedulingInfoReports(clusterInfo)
	clusterInfoByPool := scheduling.GroupSchedulingInfoByPool(activeClusterInfo)

	for _, queue := range queues {
		resourceUsageByPool := map[string]common.ComputeResources{}
		nonMatchingJobs := map[string]stringSet{}

		err := c.jobRepository.IterateQueueJobs(queue.Name, func(job *api.Job) {
			jobResources := common.TotalJobResourceRequest(job)
			nonMatchingClusters := stringSet{}

			for pool, infos := range clusterInfoByPool {
				matches := false
				for _, schedulingInfo := range infos {
					if scheduling.MatchSchedulingRequirements(job, schedulingInfo) {
						matches = true
					} else {
						nonMatchingClusters[schedulingInfo.ClusterId] = empty{}
					}
				}

				if matches {
					r, exists := resourceUsageByPool[pool]
					if !exists {
						r = common.ComputeResources{}
						resourceUsageByPool[pool] = r
					}
					r.Add(jobResources)
				}
			}
			nonMatchingJobs[job.Id] = nonMatchingClusters
		})

		if err != nil {
			log.Errorf("Error while getting queue %s resources %s", queue.Name, err)
		}

		c.updateQueuedNonMatchingJobs(queue.Name, nonMatchingJobs)
		c.updateQueuedResource(queue.Name, resourceUsageByPool)
	}
}

func (c *QueueCache) updateQueuedResource(queueName string, resourcesByPool map[string]common.ComputeResources) {
	c.refreshMutex.Lock()
	defer c.refreshMutex.Unlock()
	floatResourcesByPool := map[string]common.ComputeResourcesFloat{}
	for pool, res := range resourcesByPool {
		floatResourcesByPool[pool] = res.AsFloat()
	}
	c.queuedResources[queueName] = floatResourcesByPool
}

func (c *QueueCache) updateQueuedNonMatchingJobs(queueName string, nonMatchingClustersById map[string]stringSet) {
	c.refreshMutex.Lock()
	defer c.refreshMutex.Unlock()
	c.queueNonMatchingJobIds[queueName] = nonMatchingClustersById
}

func (c *QueueCache) GetQueuedResources(queueName string) map[string]common.ComputeResourcesFloat {
	c.refreshMutex.Lock()
	defer c.refreshMutex.Unlock()
	return c.queuedResources[queueName]
}

func (c *QueueCache) getNonSchedulableJobIds(queueName string) map[string]stringSet {
	c.refreshMutex.Lock()
	defer c.refreshMutex.Unlock()
	return c.queueNonMatchingJobIds[queueName]
}

func (c *QueueCache) PeekClusterQueue(clusterId, queue string, limit int64) ([]*api.Job, error) {
	ids, e := c.jobRepository.GetQueueJobIds(queue)
	if e != nil {
		return nil, e
	}
	nonMatchingJobs := c.getNonSchedulableJobIds(queue)

	filtered := []string{}
	for _, id := range ids {
		if matches(nonMatchingJobs, clusterId, id) {
			filtered = append(filtered, id)
		}
		if len(filtered) == int(limit) {
			break
		}
	}
	return c.jobRepository.GetExistingJobsByIds(filtered)
}

func matches(nonMatchingJobs map[string]stringSet, clusterId, jobId string) bool {
	nonMatchingClusters, ok := nonMatchingJobs[jobId]
	if !ok {
		return true
	}
	_, exists := nonMatchingClusters[clusterId]
	return !exists
}

func (c *QueueCache) TryLeaseJobs(clusterId string, queue string, jobs []*api.Job) ([]*api.Job, error) {
	return c.jobRepository.TryLeaseJobs(clusterId, queue, jobs)
}