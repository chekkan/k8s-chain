package main

import (
	"flag"
	"k8s-sniffer/pkg/config"
	"k8s-sniffer/pkg/jobs"
	"path/filepath"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func main() {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		// try to load the kubeconfig from home .kube/config folder
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		// try to load the kubeconfig from command line arg?
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}

	flag.Parse()

	restClientConfig, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)

	if err != nil {
		panic(err)
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(restClientConfig)
	if err != nil {
		panic(err.Error())
	}

	// read the configuration file
	c := config.SnifferConfig{}
	c.ParseConfiguration(clientset.CoreV1().Secrets(v1.NamespaceDefault))

	_, jobsController := jobs.Controller(c, clientset)

	stop := make(chan struct{})
	go jobsController.Run(stop)
	for {
		time.Sleep(time.Second)
	}
}
