package main

import (
	"fmt"
	"os"
	"path/filepath"
	"context"
	"sort"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	api "k8s.io/client-go/tools/clientcmd/api"
)

type MonitoringTarget struct {
	Cluster   string
	Namespace string
	Ingress   string
	Host      string
}

func main() {
	home, _ := os.UserHomeDir()
	kubeconfig := filepath.Join(home, ".kube", "desenvolvimento.yaml")

	rawConfig, err := clientcmd.LoadFromFile(kubeconfig)
	if err != nil {
		panic(err)
	}

	ctx := context.Background()



	targets, err := discoverTargets(rawConfig, ctx)
		if err != nil {
			panic(err)
		}
	 
	for _, target := range targets {
		fmt.Printf(
			"%s/%s/%s → %s\n",
			target.Cluster,
			target.Namespace,
			target.Ingress,
			target.Host,
		)
	}
	
}

func discoverTargets(rawConfig *api.Config,ctx context.Context,) ([]MonitoringTarget, error) {
	var allTargets []MonitoringTarget

	for contextName := range rawConfig.Contexts {
		clientset, err := clientForContext(rawConfig, contextName)
		if err != nil {
			return nil, err
		}

		cluster := rawConfig.Contexts[contextName].Cluster

		targets, err := findMonitoredIngresses(clientset, ctx, cluster)
		if err != nil {
			return nil, err
		}

		allTargets = append(allTargets, targets...)
	}

	return normalizeTargets(allTargets), nil
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
				Cluster:   cluster,
				Namespace: ingress.Namespace,
				Ingress:   ingress.Name,
				Host:      rule.Host,
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

func deduplicateTargets(targets []MonitoringTarget) []MonitoringTarget {
	seen := make(map[string]struct{})
	result := make([]MonitoringTarget, 0, len(targets))

	for _, target := range targets {
		if _, exists := seen[target.Host]; exists {
			continue
		}

		seen[target.Host] = struct{}{}
		result = append(result, target)
	}

	return result
}

func normalizeTargets(targets []MonitoringTarget) []MonitoringTarget {
	targets = deduplicateTargets(targets)

	sort.Slice(targets, func(i, j int) bool {
		return targets[i].Host < targets[j].Host
	})

	return targets
}


