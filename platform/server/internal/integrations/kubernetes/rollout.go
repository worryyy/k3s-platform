package kubernetes

import appsv1 "k8s.io/api/apps/v1"

type DeploymentStatus struct {
	Namespace         string                `json:"namespace"`
	Name              string                `json:"name"`
	DesiredReplicas   int32                 `json:"desired_replicas"`
	ReadyReplicas     int32                 `json:"ready_replicas"`
	UpdatedReplicas   int32                 `json:"updated_replicas"`
	AvailableReplicas int32                 `json:"available_replicas"`
	Conditions        []DeploymentCondition `json:"conditions,omitempty"`
	Pods              []PodStatus           `json:"pods,omitempty"`
	Events            []EventRecord         `json:"events,omitempty"`
}

type DeploymentCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

func DeploymentStatusFrom(deployment *appsv1.Deployment) DeploymentStatus {
	desired := int32(1)
	if deployment.Spec.Replicas != nil {
		desired = *deployment.Spec.Replicas
	}
	status := DeploymentStatus{
		Namespace:         deployment.Namespace,
		Name:              deployment.Name,
		DesiredReplicas:   desired,
		ReadyReplicas:     deployment.Status.ReadyReplicas,
		UpdatedReplicas:   deployment.Status.UpdatedReplicas,
		AvailableReplicas: deployment.Status.AvailableReplicas,
	}
	for _, condition := range deployment.Status.Conditions {
		status.Conditions = append(status.Conditions, DeploymentCondition{
			Type:    string(condition.Type),
			Status:  string(condition.Status),
			Reason:  condition.Reason,
			Message: condition.Message,
		})
	}
	return status
}

func (d DeploymentStatus) Success() bool {
	return d.ReadyReplicas == d.DesiredReplicas &&
		d.UpdatedReplicas == d.DesiredReplicas &&
		d.AvailableReplicas == d.DesiredReplicas
}

func (d DeploymentStatus) HasBlockingPodCondition() bool {
	for _, pod := range d.Pods {
		if pod.Blocking {
			return true
		}
	}
	return false
}
