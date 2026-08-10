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

func main() {
	home, _ := os.UserHomeDir()
	kubeconfig := filepath.Join(home, ".kube", "desenvolvimento.yaml")

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}
	ctx := context.Background()

	hosts, err := findMonitoredIngresses(clientset, ctx)
	if err != nil {
		panic(err)
	}

	for _, host := range hosts {
		fmt.Println(host)
	}

}
func findMonitoredIngresses(clientset *kubernetes.Clientset, ctx context.Context) ([]string, error) {
	ingresses, err := clientset.NetworkingV1().Ingresses(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}


	var hosts []string
	

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
			hosts = append(hosts, rule.Host)
		}
	}

	return hosts, nil
}
