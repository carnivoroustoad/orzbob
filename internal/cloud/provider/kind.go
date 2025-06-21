package provider

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
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
	// Generate unique instance ID
	instanceID := fmt.Sprintf("runner-%d", time.Now().Unix())
	
	// Create pod spec based on tier
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instanceID,
			Namespace: k.namespace,
			Labels: map[string]string{
				"app":  "orzbob-runner",
				"tier": tier,
				"id":   instanceID,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "runner",
					Image:   "runner:dev",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Env: []corev1.EnvVar{
						{
							Name:  "REPO_URL",
							Value: "https://github.com/carnivoroustoad/orzbob.git",
						},
						{
							Name:  "BRANCH",
							Value: "main",
						},
						{
							Name:  "INSTANCE_ID",
							Value: instanceID,
						},
						{
							Name:  "CONTROL_PLANE_URL",
							Value: "http://orzbob-cp:8080",
						},
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    getResourceForTier(tier, "cpu"),
							corev1.ResourceMemory: getResourceForTier(tier, "memory"),
						},
					},
					WorkingDir: "/workspace",
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
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

// getResourceForTier returns resource requirements based on tier
func getResourceForTier(tier, resourceType string) resource.Quantity {
	switch tier {
	case "small":
		if resourceType == "cpu" {
			return resource.MustParse("2")
		}
		return resource.MustParse("4Gi")
	case "medium":
		if resourceType == "cpu" {
			return resource.MustParse("4")
		}
		return resource.MustParse("8Gi")
	case "gpu":
		if resourceType == "cpu" {
			return resource.MustParse("8")
		}
		return resource.MustParse("24Gi")
	default:
		if resourceType == "cpu" {
			return resource.MustParse("2")
		}
		return resource.MustParse("4Gi")
	}
}