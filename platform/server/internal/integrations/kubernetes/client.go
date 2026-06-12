package kubernetes

import (
	"context"
	"fmt"
	"sort"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	client kubernetes.Interface
}

func NewClient(kubeconfig string) (*Client, error) {
	cfg, err := buildRestConfig(kubeconfig)
	if err != nil {
		return nil, err
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("create Kubernetes client: %w", err)
	}
	return &Client{client: client}, nil
}

func (c *Client) GetDeploymentStatus(ctx context.Context, namespace, name string) (DeploymentStatus, error) {
	deployment, err := c.client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return DeploymentStatus{}, fmt.Errorf("get deployment %s/%s: %w", namespace, name, err)
	}
	status := DeploymentStatusFrom(deployment)
	pods, err := c.ListPodsForDeployment(ctx, namespace, name)
	if err != nil {
		return DeploymentStatus{}, err
	}
	events, err := c.GetRecentEvents(ctx, namespace)
	if err != nil {
		return DeploymentStatus{}, err
	}
	status.Pods = pods
	status.Events = events
	return status, nil
}

func (c *Client) ListPodsForDeployment(ctx context.Context, namespace, deploymentName string) ([]PodStatus, error) {
	deployment, err := c.client.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get deployment %s/%s for pods: %w", namespace, deploymentName, err)
	}
	replicaSets, err := c.client.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list replicasets in %s: %w", namespace, err)
	}
	ownedReplicaSets := map[string]struct{}{}
	for _, rs := range replicaSets.Items {
		if ownedByDeployment(rs, deployment) {
			ownedReplicaSets[rs.Name] = struct{}{}
		}
	}

	pods, err := c.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list pods in %s: %w", namespace, err)
	}
	var result []PodStatus
	for _, pod := range pods.Items {
		if ownedByReplicaSet(pod, ownedReplicaSets) {
			result = append(result, podStatusFrom(pod))
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result, nil
}

func (c *Client) GetRecentEvents(ctx context.Context, namespace string) ([]EventRecord, error) {
	events, err := c.client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list events in %s: %w", namespace, err)
	}
	cutoff := time.Now().Add(-30 * time.Minute)
	var records []EventRecord
	for _, event := range events.Items {
		when := event.LastTimestamp
		if when.IsZero() {
			when = metav1.NewTime(event.EventTime.Time)
		}
		if !when.IsZero() && when.Time.Before(cutoff) {
			continue
		}
		records = append(records, EventRecord{
			Type:           event.Type,
			Reason:         event.Reason,
			Message:        event.Message,
			InvolvedObject: fmt.Sprintf("%s/%s", event.InvolvedObject.Kind, event.InvolvedObject.Name),
			LastTimestamp:  when,
		})
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].LastTimestamp.Time.After(records[j].LastTimestamp.Time)
	})
	if len(records) > 30 {
		records = records[:30]
	}
	return records, nil
}

func ownedByDeployment(rs appsv1.ReplicaSet, deployment *appsv1.Deployment) bool {
	for _, owner := range rs.OwnerReferences {
		if owner.Kind == "Deployment" && owner.Name == deployment.Name && owner.UID == deployment.UID {
			return true
		}
	}
	return false
}

func ownedByReplicaSet(pod corev1.Pod, replicaSets map[string]struct{}) bool {
	for _, owner := range pod.OwnerReferences {
		if owner.Kind == "ReplicaSet" {
			_, ok := replicaSets[owner.Name]
			return ok
		}
	}
	return false
}

func buildRestConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("build Kubernetes config from kubeconfig: %w", err)
		}
		return cfg, nil
	}
	cfg, err := rest.InClusterConfig()
	if err == nil {
		return cfg, nil
	}
	cfg, err = clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		return nil, fmt.Errorf("build Kubernetes config: %w", err)
	}
	return cfg, nil
}
