package scheduler_test

import (
	"encoding/json"
	"testing"

	"orzbob/internal/cloud/config"
	"orzbob/internal/scheduler"
)

func TestSidecarPodGeneration(t *testing.T) {
	// Create cloud config with sidecars
	cloudConfig := &config.CloudConfig{
		Version: "1.0",
		Services: map[string]config.ServiceConfig{
			"postgres": {
				Image: "postgres:15",
				Env: map[string]string{
					"POSTGRES_PASSWORD": "testpass",
					"POSTGRES_DB":       "testdb",
				},
				Ports: []int{5432},
				Health: config.HealthConfig{
					Command:  []string{"pg_isready", "-U", "postgres"},
					Interval: "10s",
					Timeout:  "5s",
					Retries:  5,
				},
			},
			"redis": {
				Image: "redis:7",
				Ports: []int{6379},
				Health: config.HealthConfig{
					Command: []string{"redis-cli", "ping"},
				},
			},
		},
		Env: map[string]string{
			"DATABASE_URL": "postgres://localhost:5432/testdb",
			"REDIS_URL":    "redis://localhost:6379",
		},
	}

	// Create pod config
	podConfig := scheduler.PodConfig{
		Name:        "test-instance",
		Namespace:   "default",
		Tier:        "small",
		Image:       "runner:dev",
		RepoURL:     "https://github.com/test/repo",
		Branch:      "main",
		CloudConfig: cloudConfig,
	}

	// Build pod spec
	builder := scheduler.NewPodSpecBuilder(podConfig)
	pod := builder.Build()

	// Verify pod structure
	t.Logf("Generated pod spec for: %s", pod.Name)

	// Check container count (should be 3: runner + postgres + redis)
	if len(pod.Spec.Containers) != 3 {
		t.Errorf("Expected 3 containers, got %d", len(pod.Spec.Containers))
	}

	// Find and verify each container
	containers := make(map[string]bool)
	for _, container := range pod.Spec.Containers {
		containers[container.Name] = true
		t.Logf("Container: %s (image: %s)", container.Name, container.Image)

		// Check postgres container
		if container.Name == "postgres" {
			if container.Image != "postgres:15" {
				t.Errorf("Expected postgres:15 image, got %s", container.Image)
			}

			// Check env vars
			envMap := make(map[string]string)
			for _, env := range container.Env {
				envMap[env.Name] = env.Value
			}
			if envMap["POSTGRES_PASSWORD"] != "testpass" {
				t.Errorf("Expected POSTGRES_PASSWORD=testpass, got %s", envMap["POSTGRES_PASSWORD"])
			}
			if envMap["POSTGRES_DB"] != "testdb" {
				t.Errorf("Expected POSTGRES_DB=testdb, got %s", envMap["POSTGRES_DB"])
			}

			// Check ports
			if len(container.Ports) != 1 || container.Ports[0].ContainerPort != 5432 {
				t.Error("Expected port 5432 for postgres")
			}

			// Check health probes
			if container.LivenessProbe == nil {
				t.Error("Expected liveness probe for postgres")
			} else {
				if len(container.LivenessProbe.Exec.Command) != 3 {
					t.Errorf("Expected 3 command args for health check, got %d", 
						len(container.LivenessProbe.Exec.Command))
				}
			}
		}

		// Check redis container
		if container.Name == "redis" {
			if container.Image != "redis:7" {
				t.Errorf("Expected redis:7 image, got %s", container.Image)
			}
			if len(container.Ports) != 1 || container.Ports[0].ContainerPort != 6379 {
				t.Error("Expected port 6379 for redis")
			}
		}

		// Check runner container has env vars from cloud config
		if container.Name == "runner" {
			envMap := make(map[string]string)
			for _, env := range container.Env {
				envMap[env.Name] = env.Value
			}
			if envMap["DATABASE_URL"] != "postgres://localhost:5432/testdb" {
				t.Error("Expected DATABASE_URL to be set in runner container")
			}
			if envMap["REDIS_URL"] != "redis://localhost:6379" {
				t.Error("Expected REDIS_URL to be set in runner container")
			}
		}
	}

	// Verify all expected containers exist
	for _, name := range []string{"runner", "postgres", "redis"} {
		if !containers[name] {
			t.Errorf("Container %s not found", name)
		}
	}

	// Print pod spec as JSON for debugging
	podJSON, err := json.MarshalIndent(pod.Spec, "", "  ")
	if err != nil {
		t.Errorf("Failed to marshal pod spec: %v", err)
	} else {
		t.Logf("Pod spec:\n%s", podJSON)
	}
}