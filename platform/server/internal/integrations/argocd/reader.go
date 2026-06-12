package argocd

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var applicationGVR = schema.GroupVersionResource{
	Group:    "argoproj.io",
	Version:  "v1alpha1",
	Resource: "applications",
}

type Reader struct {
	client dynamic.Interface
}

func NewReader(kubeconfig string) (*Reader, error) {
	cfg, err := restConfig(kubeconfig)
	if err != nil {
		return nil, err
	}
	client, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("create dynamic Kubernetes client: %w", err)
	}
	return &Reader{client: client}, nil
}

func (r *Reader) GetApplication(ctx context.Context, namespace, name string) (Application, error) {
	object, err := r.client.Resource(applicationGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return Application{}, fmt.Errorf("get ArgoCD Application %s/%s: %w", namespace, name, err)
	}
	return applicationFromUnstructured(object), nil
}

func applicationFromUnstructured(object *unstructured.Unstructured) Application {
	syncStatus, _, _ := unstructured.NestedString(object.Object, "status", "sync", "status")
	healthStatus, _, _ := unstructured.NestedString(object.Object, "status", "health", "status")
	revision, _, _ := unstructured.NestedString(object.Object, "status", "sync", "revision")
	operationPhase, _, _ := unstructured.NestedString(object.Object, "status", "operationState", "phase")
	return Application{
		Name:           object.GetName(),
		Namespace:      object.GetNamespace(),
		SyncStatus:     syncStatus,
		HealthStatus:   healthStatus,
		Revision:       revision,
		OperationPhase: operationPhase,
	}
}

func restConfig(kubeconfig string) (*rest.Config, error) {
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
