package provider

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"orzbob/internal/cloud/config"
	"orzbob/internal/scheduler"
)

// LocalKind implements Provider using a local kind cluster
type LocalKind struct {
	clientset *kubernetes.Clientset
	namespace string
}

// NewLocalKind creates a new LocalKind provider
func NewLocalKind(kubeconfig string) (*LocalKind, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return &LocalKind{
		clientset: clientset,
		namespace: "orzbob-runners",
	}, nil
}

// CreateInstance creates a new pod in the kind cluster
func (k *LocalKind) CreateInstance(ctx context.Context, tier string) (*Instance, error) {
	return k.CreateInstanceWithConfig(ctx, tier, nil)
}

// CreateInstanceWithConfig creates a new pod with optional cloud config
func (k *LocalKind) CreateInstanceWithConfig(ctx context.Context, tier string, cloudConfig *config.CloudConfig) (*Instance, error) {
	// Generate unique instance ID
	instanceID := fmt.Sprintf("runner-%d", time.Now().Unix())
	
	// Build pod configuration
	podConfig := scheduler.PodConfig{
		Name:      instanceID,
		Namespace: k.namespace,
		Tier:      tier,
		Image:     "runner:dev",
		RepoURL:   "https://github.com/carnivoroustoad/orzbob.git",
		Branch:    "main",
		CloudConfig: cloudConfig,
	}

	// Use PodSpecBuilder to create the pod
	builder := scheduler.NewPodSpecBuilder(podConfig)
	pod := builder.Build()

	// Add additional environment variables for control plane
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == "runner" {
			pod.Spec.Containers[i].Env = append(pod.Spec.Containers[i].Env,
				corev1.EnvVar{
					Name:  "CONTROL_PLANE_URL",
					Value: "http://orzbob-cp:8080",
				},
			)
			break
		}
	}

	// Create the pod
	createdPod, err := k.clientset.CoreV1().Pods(k.namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create pod: %w", err)
	}

	return &Instance{
		ID:        instanceID,
		Status:    string(createdPod.Status.Phase),
		Tier:      tier,
		CreatedAt: createdPod.CreationTimestamp.Time,
		PodName:   createdPod.Name,
		Namespace: createdPod.Namespace,
	}, nil
}

// GetInstance retrieves a pod's details
func (k *LocalKind) GetInstance(ctx context.Context, id string) (*Instance, error) {
	pod, err := k.clientset.CoreV1().Pods(k.namespace).Get(ctx, id, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get pod: %w", err)
	}

	return &Instance{
		ID:        id,
		Status:    string(pod.Status.Phase),
		Tier:      pod.Labels["tier"],
		CreatedAt: pod.CreationTimestamp.Time,
		PodName:   pod.Name,
		Namespace: pod.Namespace,
	}, nil
}

// ListInstances lists all runner pods
func (k *LocalKind) ListInstances(ctx context.Context) ([]*Instance, error) {
	pods, err := k.clientset.CoreV1().Pods(k.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=orzbob-runner",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	instances := make([]*Instance, 0, len(pods.Items))
	for _, pod := range pods.Items {
		instances = append(instances, &Instance{
			ID:        pod.Labels["id"],
			Status:    string(pod.Status.Phase),
			Tier:      pod.Labels["tier"],
			CreatedAt: pod.CreationTimestamp.Time,
			PodName:   pod.Name,
			Namespace: pod.Namespace,
		})
	}

	return instances, nil
}

// DeleteInstance deletes a pod
func (k *LocalKind) DeleteInstance(ctx context.Context, id string) error {
	deletePolicy := metav1.DeletePropagationForeground
	err := k.clientset.CoreV1().Pods(k.namespace).Delete(ctx, id, metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	})
	if err != nil {
		return fmt.Errorf("failed to delete pod: %w", err)
	}
	return nil
}

// GetAttachURL returns a fake URL for now
func (k *LocalKind) GetAttachURL(ctx context.Context, id string) (string, error) {
	return fmt.Sprintf("ws://localhost:8080/attach/%s", id), nil
}