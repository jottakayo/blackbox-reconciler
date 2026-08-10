package main

import (
	"fmt"
	"os"
	"path/filepath"
	"context"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type MonitoringTarget struct {
		Host string
		Cluster string
	}

func main() {
	home, _ := os.UserHomeDir()
	kubeconfig := filepath.Join(home, ".kube", "desenvolvimento.yaml")

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err)
	}

	rawConfig, err := clientcmd.LoadFromFile(kubeconfig)
	if err != nil {
		panic(err)
	}

	contextConfig := rawConfig.Contexts[rawConfig.CurrentContext]

	for contextName, contextConfig := range rawConfig.Contexts {
		fmt.Println("Context:", contextName)
		fmt.Println("Cluster:", contextConfig.Cluster)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}
	ctx := context.Background()

	targets, err := findMonitoredIngresses(clientset, ctx, contextConfig.Cluster)
	if err != nil {
		panic(err)
	}

	for _, target := range targets {
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
