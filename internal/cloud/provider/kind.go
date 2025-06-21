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
	var config *rest.Config
	var err error
	
	if kubeconfig == "" {
		// Try in-cluster config first
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to build in-cluster config: %w", err)
		}
	} else {
		// Use provided kubeconfig
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to build kubeconfig: %w", err)
		}
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
	return k.CreateInstanceWithSecrets(ctx, tier, nil)
}

// CreateInstanceWithSecrets creates a new pod with secrets
func (k *LocalKind) CreateInstanceWithSecrets(ctx context.Context, tier string, secrets []string) (*Instance, error) {
	return k.CreateInstanceWithConfig(ctx, tier, nil, secrets)
}

// CreateInstanceWithConfig creates a new pod with optional cloud config and secrets
func (k *LocalKind) CreateInstanceWithConfig(ctx context.Context, tier string, cloudConfig *config.CloudConfig, secrets []string) (*Instance, error) {
	// Generate unique instance ID
	instanceID := fmt.Sprintf("runner-%d", time.Now().Unix())
	
	// Build pod configuration
	podConfig := scheduler.PodConfig{
		Name:        instanceID,
		Namespace:   k.namespace,
		Tier:        tier,
		Image:       "runner:dev",
		RepoURL:     "https://github.com/carnivoroustoad/orzbob.git",
		Branch:      "main",
		CloudConfig: cloudConfig,
		Secrets:     secrets,
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
		Secrets:   secrets,
		Labels:    createdPod.Labels,
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
		Labels:    pod.Labels,
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
			Labels:    pod.Labels,
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

// CreateSecret creates a Kubernetes secret
func (k *LocalKind) CreateSecret(ctx context.Context, name string, data map[string]string) (*Secret, error) {
	// Convert string data to byte data
	byteData := make(map[string][]byte)
	for key, value := range data {
		byteData[key] = []byte(value)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: k.namespace,
			Labels: map[string]string{
				"app":     "orzbob",
				"type":    "user-secret",
				"managed": "true",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: byteData,
	}

	createdSecret, err := k.clientset.CoreV1().Secrets(k.namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create secret: %w", err)
	}

	return &Secret{
		Name:      createdSecret.Name,
		Namespace: createdSecret.Namespace,
		Data:      data,
		CreatedAt: createdSecret.CreationTimestamp.Time,
	}, nil
}

// GetSecret retrieves a Kubernetes secret
func (k *LocalKind) GetSecret(ctx context.Context, name string) (*Secret, error) {
	secret, err := k.clientset.CoreV1().Secrets(k.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get secret: %w", err)
	}

	// Convert byte data to string data
	data := make(map[string]string)
	for key, value := range secret.Data {
		data[key] = string(value)
	}

	return &Secret{
		Name:      secret.Name,
		Namespace: secret.Namespace,
		Data:      data,
		CreatedAt: secret.CreationTimestamp.Time,
	}, nil
}

// ListSecrets lists all Kubernetes secrets managed by orzbob
func (k *LocalKind) ListSecrets(ctx context.Context) ([]*Secret, error) {
	secretList, err := k.clientset.CoreV1().Secrets(k.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=orzbob,type=user-secret",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets: %w", err)
	}

	secrets := make([]*Secret, 0, len(secretList.Items))
	for _, secret := range secretList.Items {
		// Convert byte data to string data
		data := make(map[string]string)
		for key, value := range secret.Data {
			data[key] = string(value)
		}

		secrets = append(secrets, &Secret{
			Name:      secret.Name,
			Namespace: secret.Namespace,
			Data:      data,
			CreatedAt: secret.CreationTimestamp.Time,
		})
	}

	return secrets, nil
}

// DeleteSecret deletes a Kubernetes secret
func (k *LocalKind) DeleteSecret(ctx context.Context, name string) error {
	err := k.clientset.CoreV1().Secrets(k.namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete secret: %w", err)
	}
	return nil
}