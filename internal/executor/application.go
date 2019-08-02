package executor

import (
	"github.com/G-Research/k8s-batch/internal/armada/api"
	"github.com/G-Research/k8s-batch/internal/executor/configuration"
	"github.com/G-Research/k8s-batch/internal/executor/reporter"
	"github.com/G-Research/k8s-batch/internal/executor/service"
	"github.com/G-Research/k8s-batch/internal/executor/submitter"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	informer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"
	"os"
	"sync"
	"time"
)

func StartUp(config configuration.ExecutorConfiguration) (func(), *sync.WaitGroup) {
	kubernetesClient, err := CreateKubernetesClient(&config.Kubernetes)
	if err != nil {
		log.Errorf("Failed to connect to kubernetes because %s", err)
		os.Exit(-1)
	}

	conn, err := grpc.Dial(config.Armada.Url, grpc.WithInsecure())
	if err != nil {
		log.Errorf("Failed to connect to API because: %s", err)
		os.Exit(-1)
	}

	queueClient := api.NewAggregatedQueueClient(conn)
	usageClient := api.NewUsageClient(conn)
	eventClient := api.NewEventClient(conn)

	var eventReporter reporter.EventReporter = reporter.JobEventReporter{
		KubernetesClient: kubernetesClient,
		EventClient:      eventClient,
	}

	factory := informers.NewSharedInformerFactoryWithOptions(kubernetesClient, 0)
	podInformer := factory.Core().V1().Pods()
	nodeLister := factory.Core().V1().Nodes().Lister()
	addPodEventHandler(podInformer, eventReporter)
	informerStopper := startInformers(factory)

	jobSubmitter := submitter.JobSubmitter{KubernetesClient: kubernetesClient}

	podCleanupService := service.PodCleanupService{KubernetesClient: kubernetesClient}

	jobLeaseService := service.JobLeaseService{
		PodLister:      podInformer.Lister(),
		QueueClient:    queueClient,
		CleanupService: podCleanupService,
		ClusterId:      config.Application.ClusterId,
	}

	eventReconciliationService := service.JobEventReconciliationService{
		PodLister:     podInformer.Lister(),
		EventReporter: eventReporter,
	}

	clusterUtilisationService := service.ClusterUtilisationService{
		ClientId:    config.Application.ClusterId,
		PodLister:   podInformer.Lister(),
		NodeLister:  nodeLister,
		UsageClient: usageClient,
	}

	clusterAllocationService := service.ClusterAllocationService{
		LeaseService:       jobLeaseService,
		UtilisationService: clusterUtilisationService,
		JobSubmitter:       jobSubmitter,
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)

	tasks := make([]chan bool, 0)
	tasks = append(tasks, scheduleBackgroundTask(clusterUtilisationService.ReportClusterUtilisation, config.Task.UtilisationReportingInterval, wg))
	tasks = append(tasks, scheduleBackgroundTask(clusterAllocationService.AllocateSpareClusterCapacity, config.Task.AllocateSpareClusterCapacityInterval, wg))
	tasks = append(tasks, scheduleBackgroundTask(jobLeaseService.ManageJobLeases, config.Task.JobLeaseRenewalInterval, wg))
	tasks = append(tasks, scheduleBackgroundTask(eventReconciliationService.ReconcileMissingJobEvents, config.Task.MissingJobEventReconciliationInterval, wg))

	return func() {
		stopTasks(tasks)
		close(informerStopper)
		conn.Close()
		wg.Done()
		if waitForShutdownCompletion(wg, 2*time.Second) {
			log.Warnf("Graceful shutdown timed out")
		}
		log.Infof("Shutdown complete")
	}, wg
}

func waitForShutdownCompletion(wg *sync.WaitGroup, timeout time.Duration) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return false // completed normally
	case <-time.After(timeout):
		return true // timed out
	}
}

func scheduleBackgroundTask(task func(), interval time.Duration, wg *sync.WaitGroup) chan bool {
	stop := make(chan bool)

	wg.Add(1)
	go func() {
		task()
		for {
			select {
			case <-time.After(interval):
			case <-stop:
				wg.Done()
				return
			}
			task()
		}
	}()

	return stop
}

func stopTasks(taskChannels []chan bool) {
	for _, channel := range taskChannels {
		channel <- true
	}
}

func addPodEventHandler(podInformer informer.PodInformer, eventReporter reporter.EventReporter) {
	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod, ok := obj.(*v1.Pod)
			if !ok {
				log.Errorf("Failed to process pod event due to it being an unexpected type. Failed to process %+v", obj)
				return
			}
			go eventReporter.ReportEvent(pod)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldPod, ok := oldObj.(*v1.Pod)
			if !ok {
				log.Errorf("Failed to process pod event due to it being an unexpected type. Failed to process %+v", oldObj)
				return
			}
			newPod, ok := newObj.(*v1.Pod)
			if !ok {
				log.Errorf("Failed to process pod event due to it being an unexpected type. Failed to process %+v", newObj)
				return
			}
			go eventReporter.ReportUpdateEvent(oldPod, newPod)
		},
	})
}

func startInformers(factory informers.SharedInformerFactory) chan struct{} {
	stopper := make(chan struct{})
	factory.Start(stopper)
	factory.WaitForCacheSync(stopper)
	return stopper
}
