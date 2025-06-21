package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"orzbob/internal/cloud/config"
	"orzbob/internal/cloud/provider"
)

func main() {
	ctx := context.Background()

	log.Println("=== Testing Sidecar Containers ===")

	// Setup Kubernetes client
	kubeconfig := os.Getenv("HOME") + "/.kube/config"
	k8sConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Fatalf("Failed to build kubeconfig: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		log.Fatalf("Failed to create clientset: %v", err)
	}

	// Create LocalKind provider
	kind, err := provider.NewLocalKind(kubeconfig)
	if err != nil {
		log.Fatalf("Failed to create LocalKind provider: %v", err)
	}

	// Create cloud config with postgres sidecar
	cloudConfig := &config.CloudConfig{
		Version: "1.0",
		Services: map[string]config.ServiceConfig{
			"postgres": {
				Image: "postgres:15-alpine",
				Env: map[string]string{
					"POSTGRES_PASSWORD": "testpass",
					"POSTGRES_DB":       "testdb",
					"POSTGRES_USER":     "testuser",
				},
				Ports: []int{5432},
				Health: config.HealthConfig{
					Command:  []string{"pg_isready", "-U", "testuser"},
					Interval: "10s",
					Timeout:  "5s",
					Retries:  5,
				},
			},
		},
		Env: map[string]string{
			"PGHOST":     "localhost",
			"PGPORT":     "5432",
			"PGUSER":     "testuser",
			"PGPASSWORD": "testpass",
			"PGDATABASE": "testdb",
		},
	}

	// Create instance with sidecars
	log.Println("Creating instance with postgres sidecar...")
	instance, err := kind.CreateInstanceWithConfig(ctx, "small", cloudConfig)
	if err != nil {
		log.Fatalf("Failed to create instance: %v", err)
	}

	instanceID := instance.ID
	log.Printf("Created instance: %s", instanceID)

	// Cleanup function
	defer func() {
		log.Printf("Cleaning up instance %s", instanceID)
		if err := kind.DeleteInstance(ctx, instanceID); err != nil {
			log.Printf("Failed to delete instance: %v", err)
		}
	}()

	// Wait for pod to be running
	log.Println("Waiting for pod to be running...")
	namespace := "orzbob-runners"
	var pod *corev1.Pod
	
	for i := 0; i < 60; i++ {
		pod, err = clientset.CoreV1().Pods(namespace).Get(ctx, instanceID, metav1.GetOptions{})
		if err != nil {
			log.Printf("Failed to get pod: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		if pod.Status.Phase == corev1.PodRunning {
			// Check if all containers are ready
			allReady := true
			for _, cs := range pod.Status.ContainerStatuses {
				if !cs.Ready {
					allReady = false
					break
				}
			}
			if allReady {
				log.Println("Pod is running and all containers are ready")
				break
			}
		}

		if pod.Status.Phase == corev1.PodFailed {
			log.Fatalf("Pod failed: %v", pod.Status.Message)
		}

		log.Printf("Pod status: %s (waiting for all containers to be ready)", pod.Status.Phase)
		time.Sleep(2 * time.Second)
	}

	// Verify containers
	log.Printf("Pod has %d containers:", len(pod.Spec.Containers))
	for _, container := range pod.Spec.Containers {
		log.Printf("  - %s (image: %s)", container.Name, container.Image)
	}

	// Give postgres time to fully initialize
	log.Println("Waiting for postgres to initialize...")
	time.Sleep(10 * time.Second)

	// Test 1: Check if postgres is responding to health check
	log.Println("\n--- Test 1: Postgres Health Check ---")
	cmd := exec.Command("kubectl", "exec", instanceID, "-n", namespace, 
		"-c", "postgres", "--", "pg_isready", "-U", "testuser")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("❌ Health check failed: %v\nOutput: %s", err, output)
	} else {
		log.Printf("✅ Health check passed: %s", output)
	}

	// Test 2: Install psql in runner container
	log.Println("\n--- Test 2: Installing psql in runner container ---")
	cmd = exec.Command("kubectl", "exec", instanceID, "-n", namespace,
		"-c", "runner", "--", "sh", "-c", 
		"apk add --no-cache postgresql-client")
	output, err = cmd.CombinedOutput()
	if err != nil {
		log.Printf("Failed to install psql: %v\nOutput: %s", err, output)
		// Try apt-get if alpine apk failed
		cmd = exec.Command("kubectl", "exec", instanceID, "-n", namespace,
			"-c", "runner", "--", "sh", "-c",
			"apt-get update && apt-get install -y postgresql-client")
		output, err = cmd.CombinedOutput()
		if err != nil {
			log.Fatalf("Failed to install psql with apt: %v\nOutput: %s", err, output)
		}
	}
	log.Println("✅ psql client installed")

	// Test 3: Connect to postgres from runner container
	log.Println("\n--- Test 3: Connecting to Postgres from Runner ---")
	cmd = exec.Command("kubectl", "exec", instanceID, "-n", namespace,
		"-c", "runner", "--", "psql", "-h", "localhost", "-U", "testuser", 
		"-d", "testdb", "-c", "SELECT version();")
	cmd.Env = append(cmd.Env, "PGPASSWORD=testpass")
	output, err = cmd.CombinedOutput()
	if err != nil {
		log.Printf("❌ Failed to connect to postgres: %v\nOutput: %s", err, output)
		
		// Debug: Check environment variables
		cmd = exec.Command("kubectl", "exec", instanceID, "-n", namespace,
			"-c", "runner", "--", "env")
		envOutput, _ := cmd.CombinedOutput()
		log.Printf("Environment variables in runner:\n%s", envOutput)
	} else {
		log.Printf("✅ Successfully connected to postgres!\nOutput: %s", output)
	}

	// Test 4: Create a test table
	log.Println("\n--- Test 4: Creating Test Table ---")
	cmd = exec.Command("kubectl", "exec", instanceID, "-n", namespace,
		"-c", "runner", "--", "sh", "-c",
		`PGPASSWORD=testpass psql -h localhost -U testuser -d testdb -c "CREATE TABLE test_table (id SERIAL PRIMARY KEY, name VARCHAR(50));"`)
	output, err = cmd.CombinedOutput()
	if err != nil {
		log.Printf("❌ Failed to create table: %v\nOutput: %s", err, output)
	} else {
		log.Println("✅ Table created successfully")
	}

	// Test 5: Insert and query data
	log.Println("\n--- Test 5: Insert and Query Data ---")
	cmd = exec.Command("kubectl", "exec", instanceID, "-n", namespace,
		"-c", "runner", "--", "sh", "-c",
		`PGPASSWORD=testpass psql -h localhost -U testuser -d testdb -c "INSERT INTO test_table (name) VALUES ('test from runner'); SELECT * FROM test_table;"`)
	output, err = cmd.CombinedOutput()
	if err != nil {
		log.Printf("❌ Failed to insert/query data: %v\nOutput: %s", err, output)
	} else {
		log.Printf("✅ Data operations successful!\nOutput: %s", output)
	}

	// Get pod logs for debugging
	log.Println("\n--- Pod Logs ---")
	for _, container := range []string{"runner", "postgres"} {
		log.Printf("\nLogs for %s container:", container)
		cmd = exec.Command("kubectl", "logs", instanceID, "-n", namespace, "-c", container, "--tail=20")
		output, err = cmd.CombinedOutput()
		if err != nil {
			log.Printf("Failed to get logs: %v", err)
		} else {
			log.Printf("%s", output)
		}
	}

	log.Println("\n✅ All sidecar tests completed!")
}