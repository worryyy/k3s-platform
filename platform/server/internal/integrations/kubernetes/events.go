package kubernetes

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PodStatus struct {
	Name       string                   `json:"name"`
	Phase      string                   `json:"phase"`
	Blocking   bool                     `json:"blocking"`
	Containers []ContainerStatusSummary `json:"containers,omitempty"`
}

type ContainerStatusSummary struct {
	Name    string `json:"name"`
	Ready   bool   `json:"ready"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

type EventRecord struct {
	Type           string      `json:"type"`
	Reason         string      `json:"reason"`
	Message        string      `json:"message"`
	InvolvedObject string      `json:"involved_object"`
	LastTimestamp  metav1.Time `json:"last_timestamp"`
}

var blockingReasons = map[string]struct{}{
	"ImagePullBackOff":           {},
	"ErrImagePull":               {},
	"CrashLoopBackOff":           {},
	"Pending":                    {},
	"CreateContainerConfigError": {},
}

func podStatusFrom(pod corev1.Pod) PodStatus {
	status := PodStatus{Name: pod.Name, Phase: string(pod.Status.Phase)}
	if _, ok := blockingReasons[string(pod.Status.Phase)]; ok {
		status.Blocking = true
	}
	for _, container := range pod.Status.ContainerStatuses {
		summary := ContainerStatusSummary{Name: container.Name, Ready: container.Ready}
		if container.State.Waiting != nil {
			summary.Reason = container.State.Waiting.Reason
			summary.Message = container.State.Waiting.Message
			if _, ok := blockingReasons[summary.Reason]; ok {
				status.Blocking = true
			}
		}
		status.Containers = append(status.Containers, summary)
	}
	if pod.Status.Phase == corev1.PodPending {
		status.Blocking = true
	}
	return status
}
