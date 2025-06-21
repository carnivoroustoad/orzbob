package scheduler

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"orzbob/internal/cloud/config"
)

func TestPodSpecBuilder_Build(t *testing.T) {
	tests := []struct {
		name   string
		config PodConfig
		verify func(t *testing.T, pod *corev1.Pod)
	}{
		{
			name: "basic pod with small tier",
			config: PodConfig{
				Name:      "test-runner-123",
				Namespace: "orzbob-runners",
				Tier:      "small",
				Image:     "runner:dev",
				RepoURL:   "https://github.com/test/repo.git",
				Branch:    "main",
				Program:   "claude",
			},
			verify: func(t *testing.T, pod *corev1.Pod) {
				// Check metadata
				if pod.Name != "test-runner-123" {
					t.Errorf("expected pod name test-runner-123, got %s", pod.Name)
				}
				if pod.Namespace != "orzbob-runners" {
					t.Errorf("expected namespace orzbob-runners, got %s", pod.Namespace)
				}

				// Check labels
				if pod.Labels["app"] != "orzbob-runner" {
					t.Errorf("expected app label orzbob-runner, got %s", pod.Labels["app"])
				}
				if pod.Labels["tier"] != "small" {
					t.Errorf("expected tier label small, got %s", pod.Labels["tier"])
				}

				// Check containers
				if len(pod.Spec.Containers) != 1 {
					t.Fatalf("expected 1 container, got %d", len(pod.Spec.Containers))
				}

				container := pod.Spec.Containers[0]
				if container.Name != "runner" {
					t.Errorf("expected container name runner, got %s", container.Name)
				}
				if container.Image != "runner:dev" {
					t.Errorf("expected image runner:dev, got %s", container.Image)
				}

				// Check resources
				cpuReq := container.Resources.Requests[corev1.ResourceCPU]
				if cpuReq.Cmp(resource.MustParse("2")) != 0 {
					t.Errorf("expected CPU request 2, got %v", cpuReq)
				}
				memReq := container.Resources.Requests[corev1.ResourceMemory]
				if memReq.Cmp(resource.MustParse("4Gi")) != 0 {
					t.Errorf("expected memory request 4Gi, got %v", memReq)
				}

				// Check volumes
				if len(pod.Spec.Volumes) != 3 {
					t.Errorf("expected 3 volumes, got %d", len(pod.Spec.Volumes))
				}

				// Check security context
				if pod.Spec.SecurityContext == nil {
					t.Error("expected pod security context to be set")
				} else {
					if *pod.Spec.SecurityContext.RunAsUser != 1000 {
						t.Errorf("expected RunAsUser 1000, got %d", *pod.Spec.SecurityContext.RunAsUser)
					}
				}
			},
		},
		{
			name: "pod with init commands",
			config: PodConfig{
				Name:      "test-runner-456",
				Namespace: "orzbob-runners",
				Tier:      "medium",
				Image:     "runner:dev",
				RepoURL:   "https://github.com/test/repo.git",
				Branch:    "develop",
				InitCommands: []string{
					"npm install",
					"make build",
				},
			},
			verify: func(t *testing.T, pod *corev1.Pod) {
				// Check init containers
				if len(pod.Spec.InitContainers) != 1 {
					t.Fatalf("expected 1 init container, got %d", len(pod.Spec.InitContainers))
				}

				initContainer := pod.Spec.InitContainers[0]
				if initContainer.Name != "init-workspace" {
					t.Errorf("expected init container name init-workspace, got %s", initContainer.Name)
				}

				// Check resources for medium tier
				container := pod.Spec.Containers[0]
				cpuReq := container.Resources.Requests[corev1.ResourceCPU]
				if cpuReq.Cmp(resource.MustParse("4")) != 0 {
					t.Errorf("expected CPU request 4, got %v", cpuReq)
				}
				memReq := container.Resources.Requests[corev1.ResourceMemory]
				if memReq.Cmp(resource.MustParse("8Gi")) != 0 {
					t.Errorf("expected memory request 8Gi, got %v", memReq)
				}
			},
		},
		{
			name: "pod with cache directories",
			config: PodConfig{
				Name:      "test-runner-789",
				Namespace: "orzbob-runners",
				Tier:      "gpu",
				Image:     "runner:dev",
				RepoURL:   "https://github.com/test/repo.git",
				Branch:    "main",
				CacheDirs: []string{
					"/home/runner/.cache",
					"/home/runner/.npm",
				},
			},
			verify: func(t *testing.T, pod *corev1.Pod) {
				container := pod.Spec.Containers[0]
				
				// Check volume mounts include cache dirs
				cacheMountCount := 0
				for _, mount := range container.VolumeMounts {
					if mount.Name == "cache" && mount.SubPath != "" {
						cacheMountCount++
					}
				}
				if cacheMountCount != 2 {
					t.Errorf("expected 2 cache directory mounts, got %d", cacheMountCount)
				}

				// Check GPU tier resources
				cpuReq := container.Resources.Requests[corev1.ResourceCPU]
				if cpuReq.Cmp(resource.MustParse("8")) != 0 {
					t.Errorf("expected CPU request 8, got %v", cpuReq)
				}
				memReq := container.Resources.Requests[corev1.ResourceMemory]
				if memReq.Cmp(resource.MustParse("24Gi")) != 0 {
					t.Errorf("expected memory request 24Gi, got %v", memReq)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewPodSpecBuilder(tt.config)
			pod := builder.Build()
			tt.verify(t, pod)
		})
	}
}

func TestCreatePVC(t *testing.T) {
	pvc := CreatePVC("test-pvc", "test-namespace", "10Gi")

	if pvc.Name != "test-pvc" {
		t.Errorf("expected PVC name test-pvc, got %s", pvc.Name)
	}
	if pvc.Namespace != "test-namespace" {
		t.Errorf("expected PVC namespace test-namespace, got %s", pvc.Namespace)
	}

	storageReq := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	if storageReq.Cmp(resource.MustParse("10Gi")) != 0 {
		t.Errorf("expected storage request 10Gi, got %v", storageReq)
	}

	if len(pvc.Spec.AccessModes) != 1 || pvc.Spec.AccessModes[0] != corev1.ReadWriteOnce {
		t.Errorf("expected ReadWriteOnce access mode")
	}
}

func TestCreateConfigMap(t *testing.T) {
	scripts := map[string]string{
		"init.sh":   "#!/bin/bash\necho 'init'",
		"attach.sh": "#!/bin/bash\necho 'attach'",
	}

	cm := CreateConfigMap("test-scripts", "test-namespace", scripts)

	if cm.Name != "test-scripts" {
		t.Errorf("expected ConfigMap name test-scripts, got %s", cm.Name)
	}
	if cm.Namespace != "test-namespace" {
		t.Errorf("expected ConfigMap namespace test-namespace, got %s", cm.Namespace)
	}

	if len(cm.Data) != 2 {
		t.Errorf("expected 2 scripts, got %d", len(cm.Data))
	}
	if cm.Data["init.sh"] != scripts["init.sh"] {
		t.Errorf("init.sh content mismatch")
	}
}

func TestGetTierResources(t *testing.T) {
	tests := []struct {
		tier           string
		expectedCPU    string
		expectedMemory string
	}{
		{"small", "2", "4Gi"},
		{"medium", "4", "8Gi"},
		{"gpu", "8", "24Gi"},
		{"unknown", "2", "4Gi"}, // default
	}

	for _, tt := range tests {
		t.Run(tt.tier, func(t *testing.T) {
			cpu, memory := getTierResources(tt.tier)
			if cpu != tt.expectedCPU {
				t.Errorf("expected CPU %s, got %s", tt.expectedCPU, cpu)
			}
			if memory != tt.expectedMemory {
				t.Errorf("expected memory %s, got %s", tt.expectedMemory, memory)
			}
		})
	}
}

func TestPodSpecBuilder_WithSidecars(t *testing.T) {
	tests := []struct {
		name          string
		config        PodConfig
		expectedCount int
		checkSidecar  string
	}{
		{
			name: "pod with postgres sidecar",
			config: PodConfig{
				Name:      "test-pod",
				Namespace: "default",
				Tier:      "small",
				Image:     "runner:latest",
				RepoURL:   "https://github.com/test/repo",
				Branch:    "main",
				CloudConfig: &config.CloudConfig{
					Services: map[string]config.ServiceConfig{
						"postgres": {
							Image: "postgres:15",
							Env: map[string]string{
								"POSTGRES_PASSWORD": "secret",
								"POSTGRES_DB":       "myapp",
							},
							Ports: []int{5432},
							Health: config.HealthConfig{
								Command:  []string{"pg_isready", "-U", "postgres"},
								Interval: "10s",
								Timeout:  "5s",
								Retries:  5,
							},
						},
					},
				},
			},
			expectedCount: 2, // runner + postgres
			checkSidecar:  "postgres",
		},
		{
			name: "pod with redis sidecar",
			config: PodConfig{
				Name:      "test-pod",
				Namespace: "default",
				Tier:      "small",
				Image:     "runner:latest",
				RepoURL:   "https://github.com/test/repo",
				Branch:    "main",
				CloudConfig: &config.CloudConfig{
					Services: map[string]config.ServiceConfig{
						"redis": {
							Image: "redis:7-alpine",
							Ports: []int{6379},
							Health: config.HealthConfig{
								Command: []string{"redis-cli", "ping"},
							},
						},
					},
				},
			},
			expectedCount: 2, // runner + redis
			checkSidecar:  "redis",
		},
		{
			name: "pod with multiple sidecars",
			config: PodConfig{
				Name:      "test-pod",
				Namespace: "default",
				Tier:      "medium",
				Image:     "runner:latest",
				RepoURL:   "https://github.com/test/repo",
				Branch:    "main",
				CloudConfig: &config.CloudConfig{
					Services: map[string]config.ServiceConfig{
						"postgres": {
							Image: "postgres:15",
							Env: map[string]string{
								"POSTGRES_PASSWORD": "secret",
							},
							Ports: []int{5432},
						},
						"redis": {
							Image: "redis:7-alpine",
							Ports: []int{6379},
						},
					},
				},
			},
			expectedCount: 3, // runner + postgres + redis
			checkSidecar:  "postgres",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewPodSpecBuilder(tt.config)
			pod := builder.Build()

			// Check container count
			if len(pod.Spec.Containers) != tt.expectedCount {
				t.Errorf("Expected %d containers, got %d", tt.expectedCount, len(pod.Spec.Containers))
			}

			// Find sidecar container
			var sidecar *corev1.Container
			for i := range pod.Spec.Containers {
				if pod.Spec.Containers[i].Name == tt.checkSidecar {
					sidecar = &pod.Spec.Containers[i]
					break
				}
			}

			if sidecar == nil {
				t.Fatalf("Sidecar container %s not found", tt.checkSidecar)
			}

			// Verify sidecar properties
			if tt.config.CloudConfig != nil {
				serviceConfig := tt.config.CloudConfig.Services[tt.checkSidecar]
				
				// Check image
				if sidecar.Image != serviceConfig.Image {
					t.Errorf("Expected image %s, got %s", serviceConfig.Image, sidecar.Image)
				}

				// Check env vars
				for k, v := range serviceConfig.Env {
					found := false
					for _, env := range sidecar.Env {
						if env.Name == k && env.Value == v {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Environment variable %s=%s not found", k, v)
					}
				}

				// Check ports
				if len(serviceConfig.Ports) > 0 {
					if len(sidecar.Ports) != len(serviceConfig.Ports) {
						t.Errorf("Expected %d ports, got %d", len(serviceConfig.Ports), len(sidecar.Ports))
					}
				}

				// Check health probe
				if len(serviceConfig.Health.Command) > 0 {
					if sidecar.LivenessProbe == nil {
						t.Error("Expected liveness probe to be set")
					}
					if sidecar.ReadinessProbe == nil {
						t.Error("Expected readiness probe to be set")
					}
				}

				// Check resources
				if sidecar.Resources.Requests == nil {
					t.Error("Expected resource requests to be set")
				}
			}
		})
	}
}