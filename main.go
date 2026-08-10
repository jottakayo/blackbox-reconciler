package main

import (
	"fmt"
	"os"
	"path/filepath"
	"context"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	api "k8s.io/client-go/tools/clientcmd/api"
)

type MonitoringTarget struct {
		Host string
		Cluster string
	}

func main() {
	home, _ := os.UserHomeDir()
	kubeconfig := filepath.Join(home, ".kube", "desenvolvimento.yaml")

	rawConfig, err := clientcmd.LoadFromFile(kubeconfig)
	if err != nil {
		panic(err)
	}

	ctx := context.Background()

	var allTargets []MonitoringTarget

	for contextName := range rawConfig.Contexts {
		clientset, err := clientForContext(rawConfig, contextName)
		if err != nil {
			panic(err)
		}

		cluster := rawConfig.Contexts[contextName].Cluster

		targets, err := findMonitoredIngresses(clientset, ctx, cluster)
		if err != nil {
			panic(err)
		}

		allTargets = append(allTargets, targets...)
	}
	 
	for _, target := range allTargets {
		fmt.Println(target.Host, target.Cluster)
	}
}
func findMonitoredIngresses(clientset *kubernetes.Clientset, ctx context.Context,
	cluster string) ([]MonitoringTarget, error) {
	ingresses, err := clientset.NetworkingV1().Ingresses(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var targets []MonitoringTarget
	
	for _, ingress := range ingresses.Items {
		if ingress.Spec.IngressClassName == nil {
			continue
		}

		if *ingress.Spec.IngressClassName != "nginx" {
			continue
		}

		if ingress.Annotations["blackbox-reconciler.go.io/enabled"] != "true" {
			continue
		}

		for _, rule := range ingress.Spec.Rules {
			targets = append(targets, MonitoringTarget{
				Cluster: cluster,
				Host: rule.Host,
			})
		}
	}

	return targets, nil
}

func clientForContext(rawConfig *api.Config,contextName string,) (*kubernetes.Clientset, error) {
	overrides := &clientcmd.ConfigOverrides{
		CurrentContext: contextName,
	}

	configLoader := clientcmd.NewDefaultClientConfig(
		*rawConfig,
		overrides,
	)

	config, err := configLoader.ClientConfig()
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(config)
}
