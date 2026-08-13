package main

import (
	"fmt"
	"os"
	"path/filepath"
	"context"
	"sort"
	"os/exec"
    
	"gopkg.in/yaml.v3"
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

type BlackboxTarget struct {
        Name                      string            `yaml:"name"`
        URL                       string            `yaml:"url"`
        Module                    string            `yaml:"module"`
        Interval                  string            `yaml:"interval"`
        AdditionalMetricsRelabels map[string]string  `yaml:"additionalMetricsRelabels"`
  }

type BlackboxConfig struct {
	ServiceMonitor any              `yaml:"serviceMonitor,omitempty"`
	Targets        []BlackboxTarget `yaml:"targets"`
}

type CurrentBlackboxConfig struct {
	ServiceMonitor ServiceMonitorConfig `yaml:"serviceMonitor"`
}

type ServiceMonitorConfig struct {
	Targets []BlackboxTarget `yaml:"targets"`
}

const (
	defaultBlackboxModule   = "http_2xx"
	defaultBlackboxInterval = "60s"
)

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

	blackboxTargets := buildBlackboxTargets(targets)

	for _, target := range blackboxTargets {
		fmt.Printf("%+v\n", target)
	}
	config := BlackboxConfig{
		Targets: blackboxTargets,
	}
	data, err := yaml.Marshal(config)
	if err != nil {
		panic(err)
	}
	err = os.WriteFile("desired-values.yaml", data, 0644)
	if err != nil {
		panic(err)
	}
	fmt.Println("Desired configuration written to desired-values.yaml")

	fmt.Println(string(data))
	

	values, err := getHelmValues()
	if err != nil {
		panic(string(values))
	}

	currentConfig, err := parseHelmValues(values)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Current targets: %d\n", len(currentConfig.ServiceMonitor.Targets))
	
	currentTargets := currentConfig.ServiceMonitor.Targets

	if targetsEqual(currentTargets, blackboxTargets) {
		fmt.Println("Targets are already synchronized")
		return
	}

	fmt.Println("Targets differ, update required")
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

func getHelmValues() ([]byte, error) {
	cmd := exec.Command(
		"helm",
		"get",
		"values",
		"prometheus-blackbox-exporter",
		"-n",
		"cattle-monitoring-system",
		"--kube-context",
		"tools",
		"--all",
		"-o",
		"yaml",
	)

	return cmd.CombinedOutput()
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

func buildBlackboxTargets(targets []MonitoringTarget) []BlackboxTarget {
	result := make([]BlackboxTarget, 0, len(targets))

	for _, target := range targets {
		result = append(result, BlackboxTarget{
			Name:     target.Ingress,
			URL:      "https://" + target.Host,
			Module:   defaultBlackboxModule,
			Interval: defaultBlackboxInterval,
			AdditionalMetricsRelabels: map[string]string{
				"cluster":          target.Cluster,
				"ingressName":      target.Ingress,
				"ingressNamespace": target.Namespace,
			},
		})
	}

	return result
}

func parseHelmValues(data []byte) (CurrentBlackboxConfig, error) {
	var config CurrentBlackboxConfig

	err := yaml.Unmarshal(data, &config)
	if err != nil {
		return config, err
	}

	return config, nil
}

func targetsEqual(current, desired []BlackboxTarget) bool {
	if len(current) != len(desired) {
		return false
	}

	for i := range current {
		if current[i].Name != desired[i].Name {
			return false
		}

		if current[i].URL != desired[i].URL {
			return false
		}

		if current[i].Module != desired[i].Module {
			return false
		}

		if current[i].Interval != desired[i].Interval {
			return false
		}
	}

	return true
}