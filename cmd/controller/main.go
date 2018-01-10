package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/pflag"
	"k8s.io/client-go/rest"
	"k8s.io/helm/pkg/helm/environment"

	helmClientset "github.com/bitnami-labs/helm-crd/pkg/client/clientset/versioned"
)

var (
	settings environment.EnvSettings
)

func init() {
	settings.AddFlags(pflag.CommandLine)
}

func main2() error {
	config, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	clientset, err := helmClientset.NewForConfig(config)
	if err != nil {
		return err
	}

	controller := NewController(clientset)

	stop := make(chan struct{})
	defer close(stop)

	go controller.Run(stop)

	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGTERM)
	<-sigterm

	return nil
}

func main() {
	pflag.Parse()

	// set defaults from environment
	settings.Init(pflag.CommandLine)

	if err := main2(); err != nil {
		panic(err.Error())
	}
}
