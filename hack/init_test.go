package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	ctx := context.Background()

	// Create temp directory for test repo
	tmpDir, err := os.MkdirTemp("", "orzbob-init-test-*")
	if err != nil {
		log.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create .orz directory
	orzDir := filepath.Join(tmpDir, ".orz")
	if err := os.MkdirAll(orzDir, 0755); err != nil {
		log.Fatalf("Failed to create .orz directory: %v", err)
	}

	// Create cloud.yaml with init script
	cloudConfig := `version: "1.0"
setup:
  init: |
    echo "Running init script..."
    touch /tmp/marker_init_done
    echo "Init completed at $(date)" > /workspace/.orz/init_log.txt
  onAttach: |
    echo "User attached at $(date)"
env:
  TEST_VAR: "init_test"
`
	configPath := filepath.Join(orzDir, "cloud.yaml")
	if err := os.WriteFile(configPath, []byte(cloudConfig), 0644); err != nil {
		log.Fatalf("Failed to write cloud.yaml: %v", err)
	}

	// Initialize git repository
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to init git repo: %v", err)
	}

	// Add files
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to add files: %v", err)
	}

	// Commit
	cmd = exec.Command("git", "commit", "-m", "Initial commit with cloud.yaml")
	cmd.Dir = tmpDir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to commit: %v", err)
	}

	log.Printf("Created test repository at %s", tmpDir)

	// Setup Kubernetes client
	kubeconfig := os.Getenv("HOME") + "/.kube/config"
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Fatalf("Failed to build kubeconfig: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create clientset: %v", err)
	}

	// Create test pod with local repo volume
	namespace := "default"
	podName := fmt.Sprintf("init-test-%d", time.Now().Unix())

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":  "orzbob-init-test",
				"test": "init-script",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "runner",
					Image:           "runner:dev",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Env: []corev1.EnvVar{
						{
							Name:  "REPO_URL",
							Value: fmt.Sprintf("file://%s", tmpDir),
						},
						{
							Name:  "BRANCH",
							Value: "master",
						},
					},
					WorkingDir: "/workspace",
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "repo",
							MountPath: "/tmp/repo",
							ReadOnly:  true,
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "repo",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: tmpDir,
							Type: func() *corev1.HostPathType { t := corev1.HostPathDirectory; return &t }(),
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	// Create the pod
	createdPod, err := clientset.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		log.Fatalf("Failed to create pod: %v", err)
	}
	log.Printf("Created pod: %s", createdPod.Name)

	// Cleanup function
	defer func() {
		log.Printf("Cleaning up pod %s", podName)
		err := clientset.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
		if err != nil {
			log.Printf("Failed to delete pod: %v", err)
		}
	}()

	// Wait for pod to be running
	log.Println("Waiting for pod to be running...")
	for i := 0; i < 60; i++ {
		pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			log.Printf("Failed to get pod: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		if pod.Status.Phase == corev1.PodRunning {
			log.Println("Pod is running")
			break
		}

		if pod.Status.Phase == corev1.PodFailed {
			log.Fatalf("Pod failed: %v", pod.Status.Message)
		}

		log.Printf("Pod status: %s", pod.Status.Phase)
		time.Sleep(2 * time.Second)
	}

	// Give the init script time to run
	log.Println("Waiting for init script to complete...")
	time.Sleep(10 * time.Second)

	// Check if marker file exists
	cmd = exec.Command("kubectl", "exec", podName, "--", "ls", "-la", "/tmp/")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Failed to list /tmp: %v\nOutput: %s", err, output)
	} else {
		log.Printf("/tmp contents:\n%s", output)
	}

	// Check for marker file
	cmd = exec.Command("kubectl", "exec", podName, "--", "test", "-f", "/tmp/marker_init_done")
	if err := cmd.Run(); err != nil {
		log.Fatalf("❌ Init marker file not found: %v", err)
	}
	log.Println("✅ Init marker file exists!")

	// Check for init log
	cmd = exec.Command("kubectl", "exec", podName, "--", "cat", "/workspace/.orz/init_log.txt")
	output, err = cmd.CombinedOutput()
	if err != nil {
		log.Printf("Warning: Failed to read init log: %v", err)
	} else {
		log.Printf("Init log contents: %s", output)
	}

	// Check if init_done marker exists
	cmd = exec.Command("kubectl", "exec", podName, "--", "test", "-f", "/workspace/.orz/.init_done")
	if err := cmd.Run(); err != nil {
		log.Fatalf("❌ Agent init_done marker not found: %v", err)
	}
	log.Println("✅ Agent init_done marker exists!")

	// Get pod logs
	cmd = exec.Command("kubectl", "logs", podName)
	output, err = cmd.CombinedOutput()
	if err != nil {
		log.Printf("Failed to get logs: %v", err)
	} else {
		log.Printf("Pod logs:\n%s", output)
	}

	log.Println("✅ Init script test passed!")
}