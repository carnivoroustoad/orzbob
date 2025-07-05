package scheduler_test

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"orzbob/internal/scheduler"
)

func TestPodSpecBuilder_WithSecrets(t *testing.T) {
	tests := []struct {
		name            string
		secrets         []string
		expectedEnvFrom int
	}{
		{
			name:            "no secrets",
			secrets:         []string{},
			expectedEnvFrom: 0,
		},
		{
			name:            "single secret",
			secrets:         []string{"my-secret"},
			expectedEnvFrom: 1,
		},
		{
			name:            "multiple secrets",
			secrets:         []string{"db-secret", "api-secret", "cache-secret"},
			expectedEnvFrom: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := scheduler.PodConfig{
				Name:      "test-pod",
				Namespace: "default",
				Tier:      "small",
				Image:     "runner:latest",
				RepoURL:   "https://github.com/test/repo",
				Branch:    "main",
				Secrets:   tt.secrets,
			}

			builder := scheduler.NewPodSpecBuilder(config)
			pod := builder.Build()

			// Find runner container
			var runnerContainer *corev1.Container
			for i := range pod.Spec.Containers {
				if pod.Spec.Containers[i].Name == "runner" {
					runnerContainer = &pod.Spec.Containers[i]
					break
				}
			}

			if runnerContainer == nil {
				t.Fatal("Runner container not found")
			}

			// Check envFrom count
			if len(runnerContainer.EnvFrom) != tt.expectedEnvFrom {
				t.Errorf("Expected %d envFrom entries, got %d", tt.expectedEnvFrom, len(runnerContainer.EnvFrom))
			}

			// Verify each secret is referenced
			for i, secretName := range tt.secrets {
				if i >= len(runnerContainer.EnvFrom) {
					t.Errorf("Missing envFrom entry for secret %s", secretName)
					continue
				}

				envFrom := runnerContainer.EnvFrom[i]
				if envFrom.SecretRef == nil {
					t.Errorf("Expected secretRef for secret %s", secretName)
					continue
				}

				if envFrom.SecretRef.Name != secretName {
					t.Errorf("Expected secret name %s, got %s", secretName, envFrom.SecretRef.Name)
				}
			}
		})
	}
}

func TestPodWithSecretsAndEnvVars(t *testing.T) {
	// Test that both direct env vars and secrets work together
	config := scheduler.PodConfig{
		Name:      "test-pod",
		Namespace: "default",
		Tier:      "small",
		Image:     "runner:latest",
		RepoURL:   "https://github.com/test/repo",
		Branch:    "main",
		Secrets:   []string{"database-credentials", "api-keys"},
	}

	builder := scheduler.NewPodSpecBuilder(config)
	pod := builder.Build()

	// Find runner container
	var runnerContainer *corev1.Container
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == "runner" {
			runnerContainer = &pod.Spec.Containers[i]
			break
		}
	}

	if runnerContainer == nil {
		t.Fatal("Runner container not found")
	}

	// Should have both env vars and envFrom
	if len(runnerContainer.Env) == 0 {
		t.Error("Expected env vars to be set")
	}

	if len(runnerContainer.EnvFrom) != 2 {
		t.Errorf("Expected 2 envFrom entries, got %d", len(runnerContainer.EnvFrom))
	}

	// Check that basic env vars are still present
	envMap := make(map[string]string)
	for _, env := range runnerContainer.Env {
		envMap[env.Name] = env.Value
	}

	expectedEnvs := []string{"REPO_URL", "BRANCH", "INSTANCE_ID", "TIER"}
	for _, name := range expectedEnvs {
		if _, exists := envMap[name]; !exists {
			t.Errorf("Expected env var %s to exist", name)
		}
	}

	t.Logf("Pod successfully configured with %d env vars and %d secrets",
		len(runnerContainer.Env), len(runnerContainer.EnvFrom))
}
