package server

import (
	"context"
	"github.com/G-Research/k8s-batch/internal/armada/api"
	"github.com/G-Research/k8s-batch/internal/armada/repository"
	"github.com/G-Research/k8s-batch/internal/common"
	"github.com/G-Research/k8s-batch/internal/common/util"
	"github.com/gogo/protobuf/types"
	log "github.com/sirupsen/logrus"
	"math"
	"time"
)

type AggregatedQueueServer struct {
	jobRepository   repository.JobRepository
	usageRepository repository.UsageRepository
	queueRepository repository.QueueRepository
}

func NewAggregatedQueueServer(jobRepository repository.JobRepository, usageRepository repository.UsageRepository, queueRepository repository.QueueRepository) *AggregatedQueueServer {
	return &AggregatedQueueServer{jobRepository: jobRepository, usageRepository: usageRepository, queueRepository: queueRepository}
}

const minPriority = 0.5
const batchSize = 100

func (q AggregatedQueueServer) LeaseJobs(ctx context.Context, request *api.LeaseRequest) (*api.JobLease, error) {

	usageReports, e := q.usageRepository.GetClusterUsageReports()
	if e != nil {
		return nil, e
	}

	activeClusterIds := filterActiveClusters(usageReports, 10*time.Minute)
	clusterPriorities, e := q.usageRepository.GetClusterPriorities(activeClusterIds)
	if e != nil {
		return nil, e
	}

	queues, e := q.queueRepository.GetQueues()
	if e != nil {
		return nil, e
	}

	activeQueues, e := q.jobRepository.FilterActiveQueues(queues)
	if e != nil {
		return nil, e
	}
	activeQueueNames := getQueueNames(activeQueues)

	queuePriority := aggregatePriority(clusterPriorities)
	activeQueuePriority := filterMapByKeys(queuePriority, activeQueueNames, minPriority)
	slices := sliceResource(activeQueuePriority, request.Resources)
	jobs, e := q.assignJobs(request.ClusterID, slices)

	jobLease := api.JobLease{
		Job: jobs,
	}
	return &jobLease, nil
}

func (q AggregatedQueueServer) RenewLease(ctx context.Context, idList *api.IdList) (*types.Empty, error) {
	unsuccessful, e := q.jobRepository.Remove(idList.Ids)
	for _, id := range unsuccessful {
		log.WithField("jobId", id).Warnf("Failed renew lease, job id: %s", id)
	}
	return &types.Empty{}, e
}

func (q AggregatedQueueServer) ReportDone(ctx context.Context, idList *api.IdList) (*types.Empty, error) {
	unsuccessful, e := q.jobRepository.Remove(idList.Ids)
	for _, id := range unsuccessful {
		log.WithField("jobId", id).Warnf("Missing job reported done, job id: %s", id)
	}
	return &types.Empty{}, e
}

func (q AggregatedQueueServer) assignJobs(clusterId string, slices map[string]common.ComputeResourcesFloat) ([]*api.Job, error) {
	jobs := make([]*api.Job, 0)
	for queue, slice := range slices {
		leased, remainder, e := q.leaseJobs(clusterId, queue, slice)
		if e != nil {
			log.Error(e)
			continue
		}
		slices[queue] = remainder
		jobs = append(jobs, leased...)
	}
	return jobs, nil
}

func (q AggregatedQueueServer) leaseJobs(clusterId string, queue string, slice common.ComputeResourcesFloat) ([]*api.Job, common.ComputeResourcesFloat, error) {
	jobs := make([]*api.Job, 0)
	remainder := slice
	for slice.IsValid() {

		topJobs, e := q.jobRepository.PeekQueue(queue, batchSize)
		if e != nil {
			return nil, slice, e
		}

		candidates := make([]*api.Job, 0)
		for _, job := range topJobs {
			requirement := common.TotalResourceLimit(job.PodSpec).AsFloat()
			remainder = slice.DeepCopy()
			remainder.Sub(requirement)
			if remainder.IsValid() {
				slice = remainder
				candidates = append(candidates, job)
			}
		}

		leased, e := q.jobRepository.TryLeaseJobs(clusterId, queue, candidates)
		if e != nil {
			return nil, slice, e
		}

		jobs = append(jobs, leased...)

		// stop scheduling round if we leased less then batch (either the slice is too small or queue is empty)
		// TODO: should we look at next batch?
		if len(candidates) < batchSize {
			break
		}
	}
	return jobs, slice, nil
}

func sliceResource(queuePriorities map[string]float64, quantity common.ComputeResources) map[string]common.ComputeResourcesFloat {
	inversePriority := make(map[string]float64)
	for queue, priority := range queuePriorities {
		inversePriority[queue] = 1 / math.Max(priority, minPriority)
	}
	inverseSum := 0.0
	for _, inverse := range inversePriority {
		inverseSum += inverse
	}

	shares := make(map[string]common.ComputeResourcesFloat)
	for queue, inverse := range inversePriority {
		shares[queue] = quantity.Mul(inverse / inverseSum)
	}
	return shares
}

func filterActiveClusters(reports map[string]*api.ClusterUsageReport, expiry time.Duration) []string {
	var result []string
	now := time.Now()
	for id, report := range reports {
		if report.ReportTime.Add(expiry).After(now) {
			result = append(result, id)
		}
	}
	return result
}

func aggregatePriority(clusterPriorities map[string]map[string]float64) map[string]float64 {
	result := make(map[string]float64)
	for _, clusterPriority := range clusterPriorities {
		for queue, priority := range clusterPriority {
			result[queue] = priority + util.GetOrDefault(result, queue, 0)
		}
	}
	return result
}

func getQueueNames(queues []*api.Queue) []string {
	result := make([]string, 0)
	for _, q := range queues {
		result = append(result, q.Name)
	}
	return result
}

func filterMapByKeys(original map[string]float64, keys []string, defaultValue float64) map[string]float64 {
	result := make(map[string]float64)
	for _, key := range keys {
		result[key] = util.GetOrDefault(original, key, defaultValue)
	}
	return result
}
