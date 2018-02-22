package jobs

import (
	"k8s-sniffer/pkg/config"
	"k8s-sniffer/pkg/slack"
	"log"
	"strings"
	"time"

	v1batch "k8s.io/api/batch/v1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

func getActionsForTrigger(resource string, name string, namespace string, state string, conf config.SnifferConfig) []config.SnifferTriggerAction {
	actions := []config.SnifferTriggerAction{}
	for _, elem := range conf.Triggers {
		// check if we should do a starts with comparison on the filter.name
		if strings.HasSuffix(elem.Filter.Name, "*") {
			// starts with comparison failed
			if !strings.HasPrefix(name, strings.TrimSuffix(elem.Filter.Name, "*")) {
				continue
			}
		} else if name != elem.Filter.Name {
			continue
		}
		if elem.Resource == resource &&
			elem.Filter.Namespace == namespace &&
			state == elem.State {
			actions = append(actions, elem.Actions...)
		}
	}
	return actions
}

func getJobState(eventType string, jobs ...*v1batch.Job) string {
	switch eventType {
	case "add":
		return "created"
	case "delete":
		return "deleted"
	case "update":
		// check two jobs exists
		if len(jobs) != 2 {
			log.Println("expected 2 jobs to be passed in for update action")
			return "idk"
		}
		newJobStatus := jobs[0].Status
		oldJobStatus := jobs[1].Status
		// ignore if the two jobs have same statuses
		if newJobStatus.Active == oldJobStatus.Active &&
			newJobStatus.Succeeded == oldJobStatus.Succeeded &&
			newJobStatus.Failed == oldJobStatus.Failed {
			return "idk"
		}
		if newJobStatus.Active == 1 && newJobStatus.Succeeded == 0 {
			return "started"
		}
		if newJobStatus.Active == 0 && newJobStatus.Succeeded == 1 {
			return "succeeded"
		}
		if newJobStatus.Active == 0 && newJobStatus.Failed == 1 {
			return "failed"
		}
		return "changed"
	}
	return "idk"
}

func doAction(conf config.SnifferTriggerAction, job *v1batch.Job) {
	// check if its a slack notification action
	if conf.Type == "slack-notification" {
		slack.SendNotification(conf.Data, job)
	} else {
		log.Printf("not sure what action to perform %s\n", conf.Type)
	}
}

func doActions(actionConfigs []config.SnifferTriggerAction, job *v1batch.Job) {
	for _, action := range actionConfigs {
		doAction(action, job)
	}
}

func addEventHandler(conf config.SnifferConfig) func(interface{}) {
	return func(obj interface{}) {
		if job, ok := obj.(*v1batch.Job); ok {
			// return configuration for job resource with the current job's name in the job's namespace
			state := getJobState("add", job)
			actionConfs := getActionsForTrigger("job", job.Name, job.GetNamespace(), state, conf)
			if len(actionConfs) == 0 {
				log.Printf("no actions defined for job %s with state %s\n", job.Name, state)
			} else {
				doActions(actionConfs, job)
			}
		}
	}
}

func deleteEventHandler(conf config.SnifferConfig) func(interface{}) {
	return func(obj interface{}) {
		if job, ok := obj.(*v1batch.Job); ok {
			// return configuration for job resource with the current job's name in the job's namespace
			state := getJobState("delete", job)
			actionConfs := getActionsForTrigger("job", job.Name, job.GetNamespace(), state, conf)
			if len(actionConfs) == 0 {
				log.Printf("no actions defined for job %s with state %s\n", job.Name, state)
			} else {
				doActions(actionConfs, job)
			}
		}
	}
}

func updateEventHandler(conf config.SnifferConfig) func(interface{}, interface{}) {
	return func(oldObj, newObj interface{}) {
		if oldJob, ok := oldObj.(*v1batch.Job); ok {
			if newJob, ok := newObj.(*v1batch.Job); ok {
				state := getJobState("update", newJob, oldJob)
				actionConfs := getActionsForTrigger("job", newJob.Name, newJob.GetNamespace(), state, conf)
				if len(actionConfs) == 0 {
					log.Printf("no actions defined for job %s with state %s\n", newJob.Name, state)
				} else {
					doActions(actionConfs, newJob)
				}
			}
		}
	}
}

// Controller return a new informer with event handlers setup
func Controller(config config.SnifferConfig, cs *kubernetes.Clientset) (cache.Store, cache.Controller) {
	watchlist := cache.NewListWatchFromClient(cs.BatchV1().RESTClient(), "jobs", v1.NamespaceAll, fields.Everything())

	return cache.NewInformer(
		watchlist,
		&v1batch.Job{},
		time.Second*0,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    addEventHandler(config),
			DeleteFunc: deleteEventHandler(config),
			UpdateFunc: updateEventHandler(config),
		},
	)
}
