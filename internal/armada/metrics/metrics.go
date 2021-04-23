package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"

	"github.com/G-Research/armada/internal/armada/repository"
	"github.com/G-Research/armada/internal/armada/scheduling"
	"github.com/G-Research/armada/internal/common"
	"github.com/G-Research/armada/pkg/api"
)

const MetricPrefix = "armada_"

type QueueMetricProvider interface {
	GetQueuedResources(queueName string) map[string]common.ComputeResourcesFloat
	GetQueueDurations(queueName string) map[string]*DurationMetrics
}

func ExposeDataMetrics(
	queueRepository repository.QueueRepository,
	jobRepository repository.JobRepository,
	usageRepository repository.UsageRepository,
	schedulingInfoRepository repository.SchedulingInfoRepository,
	queueMetrics QueueMetricProvider,
) *QueueInfoCollector {
	collector := &QueueInfoCollector{
		queueRepository:          queueRepository,
		jobRepository:            jobRepository,
		usageRepository:          usageRepository,
		schedulingInfoRepository: schedulingInfoRepository,
		queueMetrics:             queueMetrics}
	prometheus.MustRegister(collector)
	return collector
}

type QueueInfoCollector struct {
	queueRepository          repository.QueueRepository
	jobRepository            repository.JobRepository
	usageRepository          repository.UsageRepository
	schedulingInfoRepository repository.SchedulingInfoRepository
	queueMetrics             QueueMetricProvider
}

var queueSizeDesc = prometheus.NewDesc(
	MetricPrefix+"queue_size",
	"Number of jobs in a queue",
	[]string{"queueName"},
	nil,
)

var queuePriorityDesc = prometheus.NewDesc(
	MetricPrefix+"queue_priority",
	"Priority of a queue",
	[]string{"pool", "queueName"},
	nil,
)

var queueResourcesDesc = prometheus.NewDesc(
	MetricPrefix+"queue_resource_queued",
	"Resource required by queued jobs",
	[]string{"pool", "queueName", "resourceType"},
	nil,
)

var minQueueDurationDesc = prometheus.NewDesc(
	MetricPrefix+"job_min_queued_seconds",
	"Min queue time for Armada jobs",
	[]string{"pool", "queueName"},
	nil,
)

var maxQueueDurationDesc = prometheus.NewDesc(
	MetricPrefix+"job_max_queued_seconds",
	"Max queue time for Armada jobs",
	[]string{"pool", "queueName"},
	nil,
)

var medianQueueDurationDesc = prometheus.NewDesc(
	MetricPrefix+"job_median_queued_seconds",
	"Median queue time for Armada jobs",
	[]string{"pool", "queueName"},
	nil,
)

var queueDurationDesc = prometheus.NewDesc(
	MetricPrefix+"job_queued_seconds",
	"Queued time for Armada jobs",
	[]string{"pool", "queueName"},
	nil,
)

var minJobRunDurationDesc = prometheus.NewDesc(
	MetricPrefix+"job_min_run_time_seconds",
	"Min run time for Armada jobs",
	[]string{"pool", "queueName"},
	nil,
)

var maxJobRunDurationDesc = prometheus.NewDesc(
	MetricPrefix+"job_max_run_time_seconds",
	"Max run time for Armada jobs",
	[]string{"pool", "queueName"},
	nil,
)

var medianJobRunDurationDesc = prometheus.NewDesc(
	MetricPrefix+"job_median_run_time_seconds",
	"Median run time for Armada jobs",
	[]string{"pool", "queueName"},
	nil,
)

var jobRunDurationDesc = prometheus.NewDesc(
	MetricPrefix+"job_run_time_seconds",
	"Run time for Armada jobs",
	[]string{"pool", "queueName"},
	nil,
)

var queueAllocatedDesc = prometheus.NewDesc(
	MetricPrefix+"queue_resource_allocated",
	"Resource allocated to running jobs of a queue",
	[]string{"cluster", "pool", "queueName", "resourceType"},
	nil,
)

var queueUsedDesc = prometheus.NewDesc(
	MetricPrefix+"queue_resource_used",
	"Resource actually being used by running jobs of a queue",
	[]string{"cluster", "pool", "queueName", "resourceType"},
	nil,
)

var clusterCapacityDesc = prometheus.NewDesc(
	MetricPrefix+"cluster_capacity",
	"Cluster capacity",
	[]string{"cluster", "pool", "resourceType"},
	nil,
)

var clusterAvailableCapacity = prometheus.NewDesc(
	MetricPrefix+"cluster_available_capacity",
	"Cluster capacity available for Armada jobs",
	[]string{"cluster", "pool", "resourceType"},
	nil,
)

func (c *QueueInfoCollector) Describe(desc chan<- *prometheus.Desc) {
	desc <- queueSizeDesc
	desc <- queuePriorityDesc
	desc <- queueDurationDesc
	desc <- minQueueDurationDesc
	desc <- maxQueueDurationDesc
	desc <- medianQueueDurationDesc
	desc <- jobRunDurationDesc
	desc <- minJobRunDurationDesc
	desc <- maxJobRunDurationDesc
	desc <- medianJobRunDurationDesc
}

func (c *QueueInfoCollector) Collect(metrics chan<- prometheus.Metric) {

	queues, e := c.queueRepository.GetAllQueues()
	if e != nil {
		log.Errorf("Error while getting queue metrics %s", e)
		recordInvalidMetrics(metrics, e)
		return
	}

	queueSizes, e := c.jobRepository.GetQueueSizes(queues)
	if e != nil {
		log.Errorf("Error while getting queue size metrics %s", e)
		recordInvalidMetrics(metrics, e)
		return
	}

	clusterSchedulingInfo, e := c.schedulingInfoRepository.GetClusterSchedulingInfo()
	if e != nil {
		log.Errorf("Error while getting cluster reports %s", e)
		recordInvalidMetrics(metrics, e)
		return
	}

	usageReports, e := c.usageRepository.GetClusterUsageReports()
	if e != nil {
		log.Errorf("Error while getting queue usage metrics %s", e)
		recordInvalidMetrics(metrics, e)
		return
	}

	activeClusterInfo := scheduling.FilterActiveClusterSchedulingInfoReports(clusterSchedulingInfo)
	clusterSchedulingInfoByPool := scheduling.GroupSchedulingInfoByPool(activeClusterInfo)
	runDurationsByPool := c.calculateRunningJobRunDurations(queues, clusterSchedulingInfoByPool)

	activeClusterReports := scheduling.FilterActiveClusters(usageReports)
	clusterPriorities, e := c.usageRepository.GetClusterPriorities(scheduling.GetClusterReportIds(activeClusterReports))
	if e != nil {
		log.Errorf("Error while getting queue priority metrics %s", e)
		recordInvalidMetrics(metrics, e)
		return
	}

	clustersByPool := scheduling.GroupByPool(activeClusterReports)
	for pool, poolReports := range clustersByPool {
		poolPriorities := map[string]map[string]float64{}
		for cluster := range poolReports {
			poolPriorities[cluster] = clusterPriorities[cluster]
		}
		queuePriority := scheduling.CalculateQueuesPriorityInfo(poolPriorities, poolReports, queues)
		for queue, priority := range queuePriority {
			metrics <- prometheus.MustNewConstMetric(queuePriorityDesc, prometheus.GaugeValue, priority.Priority, pool, queue.Name)
		}
	}

	for i, q := range queues {
		metrics <- prometheus.MustNewConstMetric(queueSizeDesc, prometheus.GaugeValue, float64(queueSizes[i]), q.Name)
		for pool, queueDurations := range c.queueMetrics.GetQueueDurations(q.Name) {
			if queueDurations.GetCount() > 0 {
				metrics <- prometheus.MustNewConstHistogram(queueDurationDesc, queueDurations.GetCount(),
					queueDurations.GetSum(), queueDurations.GetBuckets(), pool, q.Name)
				metrics <- prometheus.MustNewConstMetric(minQueueDurationDesc, prometheus.GaugeValue, queueDurations.GetMin(), pool, q.Name)
				metrics <- prometheus.MustNewConstMetric(maxQueueDurationDesc, prometheus.GaugeValue, queueDurations.GetMax(), pool, q.Name)
				metrics <- prometheus.MustNewConstMetric(medianQueueDurationDesc, prometheus.GaugeValue, queueDurations.GetMedian(), pool, q.Name)
			}
		}

		for pool, runningJobDurations := range runDurationsByPool[q.Name] {
			if runningJobDurations.GetCount() > 0 {
				metrics <- prometheus.MustNewConstHistogram(jobRunDurationDesc, runningJobDurations.GetCount(),
					runningJobDurations.GetSum(), runningJobDurations.GetBuckets(), pool, q.Name)
				metrics <- prometheus.MustNewConstMetric(minJobRunDurationDesc, prometheus.GaugeValue, runningJobDurations.GetMin(), pool, q.Name)
				metrics <- prometheus.MustNewConstMetric(maxJobRunDurationDesc, prometheus.GaugeValue, runningJobDurations.GetMax(), pool, q.Name)
				metrics <- prometheus.MustNewConstMetric(medianJobRunDurationDesc, prometheus.GaugeValue, runningJobDurations.GetMedian(), pool, q.Name)
			}
		}

		for pool, poolResources := range c.queueMetrics.GetQueuedResources(q.Name) {
			for resourceType, amount := range poolResources {
				metrics <- prometheus.MustNewConstMetric(queueResourcesDesc, prometheus.GaugeValue, amount, pool, q.Name, resourceType)
			}
		}
	}

	for cluster, report := range activeClusterReports {
		for _, queueReport := range report.Queues {
			for resourceType, value := range queueReport.Resources {
				metrics <- prometheus.MustNewConstMetric(
					queueAllocatedDesc,
					prometheus.GaugeValue,
					common.QuantityAsFloat64(value),
					cluster,
					report.Pool,
					queueReport.Name,
					resourceType)
			}
			for resourceType, value := range queueReport.ResourcesUsed {
				metrics <- prometheus.MustNewConstMetric(
					queueUsedDesc,
					prometheus.GaugeValue,
					common.QuantityAsFloat64(value),
					cluster,
					report.Pool,
					queueReport.Name,
					resourceType)
			}
		}
		for resourceType, value := range report.ClusterCapacity {
			metrics <- prometheus.MustNewConstMetric(
				clusterCapacityDesc,
				prometheus.GaugeValue,
				common.QuantityAsFloat64(value),
				cluster,
				report.Pool,
				resourceType)
		}

		for resourceType, value := range report.ClusterAvailableCapacity {
			metrics <- prometheus.MustNewConstMetric(
				clusterAvailableCapacity,
				prometheus.GaugeValue,
				common.QuantityAsFloat64(value),
				cluster,
				report.Pool,
				resourceType)
		}
	}
}

func (c *QueueInfoCollector) calculateRunningJobRunDurations(
	queues []*api.Queue, clusterSchedulingInfoByPool map[string]map[string]*api.ClusterSchedulingInfoReport) map[string]map[string]*DurationMetrics {

	runDurationMetrics := make(map[string]map[string]*DurationMetrics)

	for _, queue := range queues {
		metricsRecorderByPool := make(map[string]*DurationMetricsRecorder)
		leasedJobsIds, e := c.jobRepository.GetLeasedJobIds(queue.Name)
		if e != nil {
			log.Errorf("Error getting queue(%s) run duration metrics %s", queue.Name, e)
			continue
		}

		leasedJobs, e := c.jobRepository.GetExistingJobsByIds(leasedJobsIds)
		if e != nil {
			log.Errorf("Error getting queue(%s) run duration metrics %s", queue.Name, e)
			continue
		}

		startTimes, e := c.jobRepository.GetStartTimes(leasedJobsIds)
		if e != nil {
			log.Errorf("Error getting queue(%s) run duration metrics %s", queue.Name, e)
			continue
		}

		now := time.Now()
		for _, job := range leasedJobs {
			startTime, present := startTimes[job.Id]
			if !present {
				continue
			}
			runTime := now.Sub(startTime)

			for pool, infos := range clusterSchedulingInfoByPool {
				for _, schedulingInfo := range infos {
					if scheduling.MatchSchedulingRequirements(job, schedulingInfo) {
						r, exists := metricsRecorderByPool[pool]
						if !exists {
							r = NewDefaultJobDurationMetricsRecorder()
							metricsRecorderByPool[pool] = r
						}
						r.Record(runTime.Seconds())
					}
				}
			}
		}

		for pool, durations := range metricsRecorderByPool {
			runDurationMetrics[queue.Name][pool] = durations.GetMetrics()
		}
	}
	return runDurationMetrics
}

func recordInvalidMetrics(metrics chan<- prometheus.Metric, e error) {
	metrics <- prometheus.NewInvalidMetric(queueSizeDesc, e)
	metrics <- prometheus.NewInvalidMetric(queuePriorityDesc, e)
	metrics <- prometheus.NewInvalidMetric(queueResourcesDesc, e)
	metrics <- prometheus.NewInvalidMetric(queueAllocatedDesc, e)
}
