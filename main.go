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
	kubeconfig := filepath.Join(home, ".kube", "tools.yaml")

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	ingresses, err := clientset.NetworkingV1().Ingresses("sonarqube").List(ctx, metav1.ListOptions{})
	if err != nil {
		panic(err)
	}
    for _, ingress := range ingresses.Items {
	if ingress.Spec.IngressClassName == nil {
		continue
	}

	if *ingress.Spec.IngressClassName != "nginx" {
		continue
	}
	enabled := ingress.Annotations["blackbox-reconciler.jottakayo.io/enabled"]

	if enabled != "true" {
		continue
	}

	for _, rule := range ingress.Spec.Rules {
		fmt.Println(rule.Host)
	}
	}

	//fmt.Printf("Kubernetes client created: %v\n", clientset != nil)
  
}
