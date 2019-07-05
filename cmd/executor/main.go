package main

import (
	"fmt"
	"github.com/G-Research/k8s-batch/internal/reporter"
	"github.com/G-Research/k8s-batch/internal/service"
	"github.com/G-Research/k8s-batch/internal/startup"
	"github.com/G-Research/k8s-batch/internal/submitter"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	v12 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"
	"os"
	"time"
)

func main() {
	kubernetesClient, err := startup.LoadDefaultKubernetesClient()
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}

	//tweakOptionsFunc := func(options *metav1.ListOptions) {
	//	options.LabelSelector = "node-role.startup.io/master"
	//}
	//tweakOptions := informers.WithTweakListOptions(tweakOptionsFunc)
	//factory := informers.NewSharedInformerFactoryWithOptions(clientset, 0, tweakOptions)

	podEventReporter := reporter.PodEventReporter{ KubernetesClient: kubernetesClient }

	factory := informers.NewSharedInformerFactoryWithOptions(kubernetesClient, 0)
	podWatcher := initializePodWatcher(factory, podEventReporter)

	nodeWatcher := factory.Core().V1().Nodes()
	nodeLister := nodeWatcher.Lister()

	defer runtime.HandleCrash()
	stopper := make(chan struct{})
	defer close(stopper)
	factory.Start(stopper)

	jobSubmitter := submitter.JobSubmitter{KubernetesClient:kubernetesClient}

	kubernetesAllocationService := service.KubernetesAllocationService{
		PodLister: podWatcher.Lister(),
		NodeLister: nodeLister,
		JobSubmitter: jobSubmitter,
	}

	for {
		time.Sleep(5 * time.Second)

		kubernetesAllocationService.FillInSpareClusterCapacity()
	}
}

func initializePodWatcher(factory informers.SharedInformerFactory, eventReporter reporter.PodEventReporter) v12.PodInformer {
	podWatcher := factory.Core().V1().Pods()
	podWatcher.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod, ok := obj.(*v1.Pod)
			if !ok {
				return
			}
			eventReporter.ReportAddEvent(pod)
		},

		UpdateFunc: func(oldObj, newObj interface{}) {
			oldPod, ok := oldObj.(*v1.Pod)
			if !ok {
				return
			}
			newPod, ok := newObj.(*v1.Pod)
			if !ok {
				return
			}
			eventReporter.ReportUpdateEvent(oldPod, newPod)
		},
	})
	return podWatcher
}
