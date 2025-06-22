package scheduler

import (
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"orzbob/internal/cloud/config"
)

// PodConfig contains configuration for building a pod spec
type PodConfig struct {
	Name           string
	Namespace      string
	Tier           string
	Image          string
	RepoURL        string
	Branch         string
	Program        string
	CacheDirs      []string
	InitCommands   []string
	OnAttachScript string
	Secrets        []string
	CloudConfig    *config.CloudConfig
}

// PodSpecBuilder builds Kubernetes pod specifications for runner instances
type PodSpecBuilder struct {
	config PodConfig
}

// NewPodSpecBuilder creates a new PodSpecBuilder
func NewPodSpecBuilder(config PodConfig) *PodSpecBuilder {
	return &PodSpecBuilder{
		config: config,
	}
}

// Build creates a Kubernetes PodSpec
func (b *PodSpecBuilder) Build() *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      b.config.Name,
			Namespace: b.config.Namespace,
			Labels: map[string]string{
				"app":     "orzbob-runner",
				"tier":    b.config.Tier,
				"id":      b.config.Name,
				"type":    "runner",
				"version": "v1",
			},
			Annotations: b.buildAnnotations(),
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Volumes:       b.buildVolumes(),
			Containers:    b.buildContainers(),
		},
	}

	// Add init containers if needed
	if len(b.config.InitCommands) > 0 {
		pod.Spec.InitContainers = b.buildInitContainers()
	}

	// Add security context
	pod.Spec.SecurityContext = b.buildPodSecurityContext()

	return pod
}

// buildAnnotations creates pod annotations
func (b *PodSpecBuilder) buildAnnotations() map[string]string {
	annotations := make(map[string]string)
	
	// Store secrets list in annotations for retrieval
	if len(b.config.Secrets) > 0 {
		annotations["orzbob.io/secrets"] = strings.Join(b.config.Secrets, ",")
	}
	
	return annotations
}

// buildVolumes creates the volume specifications
func (b *PodSpecBuilder) buildVolumes() []corev1.Volume {
	volumes := []corev1.Volume{
		{
			Name: "workspace",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "cache", 
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	// Only add scripts volume if we have scripts
	if b.config.OnAttachScript != "" || len(b.config.InitCommands) > 0 {
		volumes = append(volumes, corev1.Volume{
			Name: "scripts",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: fmt.Sprintf("%s-scripts", b.config.Name),
					},
					DefaultMode: intPtr(0755),
				},
			},
		})
	}

	return volumes
}

// buildContainers creates the main container and any sidecar containers
func (b *PodSpecBuilder) buildContainers() []corev1.Container {
	// Set pull policy based on image
	pullPolicy := corev1.PullIfNotPresent
	if strings.Contains(b.config.Image, ":e2e") || strings.Contains(b.config.Image, ":dev") {
		pullPolicy = corev1.PullNever
	}
	
	containers := []corev1.Container{
		{
			Name:            "runner",
			Image:           b.config.Image,
			ImagePullPolicy: pullPolicy,
			Resources:       b.buildResourceRequirements(),
			VolumeMounts:    b.buildVolumeMounts(),
			Env:             b.buildEnvVars(),
			EnvFrom:         b.buildEnvFrom(),
			Command:         []string{"/usr/local/bin/cloud-agent"},
			Args:            []string{},
			WorkingDir:      "/workspace",
			SecurityContext: b.buildContainerSecurityContext(),
		},
	}

	// Add sidecar containers from cloud config
	if b.config.CloudConfig != nil {
		for name, service := range b.config.CloudConfig.Services {
			containers = append(containers, b.buildSidecarContainer(name, service))
		}
	}

	return containers
}

// buildInitContainers creates init containers for setup
func (b *PodSpecBuilder) buildInitContainers() []corev1.Container {
	return []corev1.Container{
		{
			Name:            "init-workspace",
			Image:           b.config.Image,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Command:         []string{"/bin/bash", "-c"},
			Args:            []string{b.buildInitScript()},
			VolumeMounts:    b.buildVolumeMounts(),
			Env:             b.buildEnvVars(),
			WorkingDir:      "/workspace",
		},
	}
}

// buildVolumeMounts creates volume mount specifications
func (b *PodSpecBuilder) buildVolumeMounts() []corev1.VolumeMount {
	mounts := []corev1.VolumeMount{
		{
			Name:      "workspace",
			MountPath: "/workspace",
		},
		{
			Name:      "cache",
			MountPath: "/cache",
		},
	}

	// Only mount scripts if we have them
	if b.config.OnAttachScript != "" || len(b.config.InitCommands) > 0 {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      "scripts",
			MountPath: "/scripts",
			ReadOnly:  true,
		})
	}

	// Add cache directory mounts
	for i, cacheDir := range b.config.CacheDirs {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      "cache",
			MountPath: cacheDir,
			SubPath:   fmt.Sprintf("dir%d", i),
		})
	}

	return mounts
}

// buildEnvVars creates environment variables
func (b *PodSpecBuilder) buildEnvVars() []corev1.EnvVar {
	envVars := []corev1.EnvVar{
		{
			Name:  "REPO_URL",
			Value: b.config.RepoURL,
		},
		{
			Name:  "BRANCH",
			Value: b.config.Branch,
		},
		{
			Name:  "PROGRAM",
			Value: b.config.Program,
		},
		{
			Name:  "INSTANCE_ID",
			Value: b.config.Name,
		},
		{
			Name:  "TIER",
			Value: b.config.Tier,
		},
	}

	// Add environment variables from cloud config
	if b.config.CloudConfig != nil {
		for k, v := range b.config.CloudConfig.Env {
			envVars = append(envVars, corev1.EnvVar{
				Name:  k,
				Value: v,
			})
		}
	}

	return envVars
}

// buildResourceRequirements creates resource requirements based on tier
func (b *PodSpecBuilder) buildResourceRequirements() corev1.ResourceRequirements {
	cpu, memory := getTierResources(b.config.Tier)
	
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cpu),
			corev1.ResourceMemory: resource.MustParse(memory),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cpu),
			corev1.ResourceMemory: resource.MustParse(memory),
		},
	}
}

// buildPodSecurityContext creates pod-level security context
func (b *PodSpecBuilder) buildPodSecurityContext() *corev1.PodSecurityContext {
	return &corev1.PodSecurityContext{
		RunAsNonRoot: boolPtr(true),
		RunAsUser:    int64Ptr(1000),
		RunAsGroup:   int64Ptr(1000),
		FSGroup:      int64Ptr(1000),
	}
}

// buildContainerSecurityContext creates container-level security context
func (b *PodSpecBuilder) buildContainerSecurityContext() *corev1.SecurityContext {
	return &corev1.SecurityContext{
		AllowPrivilegeEscalation: boolPtr(false),
		ReadOnlyRootFilesystem:   boolPtr(false),
		RunAsNonRoot:             boolPtr(true),
		RunAsUser:                int64Ptr(1000),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
	}
}

// buildInitScript creates the initialization script
func (b *PodSpecBuilder) buildInitScript() string {
	script := `set -e
echo "Initializing workspace..."

# Clone repository if not exists
if [ ! -d ".git" ]; then
    git clone "$REPO_URL" .
    git checkout "$BRANCH"
else
    git fetch origin
    git checkout "$BRANCH"
    git pull origin "$BRANCH"
fi

# Run custom init commands
`
	for _, cmd := range b.config.InitCommands {
		script += fmt.Sprintf("%s\n", cmd)
	}

	script += `
echo "Workspace initialization complete"
`
	return script
}

// CreatePVC creates a PersistentVolumeClaim for the pod
func CreatePVC(name, namespace string, size string) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(size),
				},
			},
		},
	}
}

// CreateConfigMap creates a ConfigMap for scripts
func CreateConfigMap(name, namespace string, scripts map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: scripts,
	}
}

// Helper functions
func getTierResources(tier string) (cpu, memory string) {
	switch tier {
	case "small":
		return "2", "4Gi"
	case "medium":
		return "4", "8Gi"
	case "gpu":
		return "8", "24Gi"
	default:
		return "2", "4Gi"
	}
}

func boolPtr(b bool) *bool {
	return &b
}

func int64Ptr(i int64) *int64 {
	return &i
}

func intPtr(i int32) *int32 {
	return &i
}

// buildEnvFrom creates envFrom entries for mounting secrets
func (b *PodSpecBuilder) buildEnvFrom() []corev1.EnvFromSource {
	var envFrom []corev1.EnvFromSource

	// Add each secret as an envFrom source
	for _, secretName := range b.config.Secrets {
		envFrom = append(envFrom, corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: secretName,
				},
			},
		})
	}

	return envFrom
}

// buildSidecarContainer creates a sidecar container from service config
func (b *PodSpecBuilder) buildSidecarContainer(name string, service config.ServiceConfig) corev1.Container {
	container := corev1.Container{
		Name:            name,
		Image:           service.Image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		SecurityContext: b.buildContainerSecurityContext(),
	}

	// Add environment variables
	if len(service.Env) > 0 {
		for k, v := range service.Env {
			container.Env = append(container.Env, corev1.EnvVar{
				Name:  k,
				Value: v,
			})
		}
	}

	// Add ports
	if len(service.Ports) > 0 {
		for _, port := range service.Ports {
			container.Ports = append(container.Ports, corev1.ContainerPort{
				ContainerPort: int32(port),
				Protocol:      corev1.ProtocolTCP,
			})
		}
	}

	// Add health probe if configured
	if len(service.Health.Command) > 0 {
		container.LivenessProbe = b.buildHealthProbe(service.Health)
		container.ReadinessProbe = b.buildHealthProbe(service.Health)
	}

	// Set resource limits based on service type
	container.Resources = b.buildSidecarResources(name)

	return container
}

// buildHealthProbe creates a health probe from health config
func (b *PodSpecBuilder) buildHealthProbe(health config.HealthConfig) *corev1.Probe {
	interval, _ := time.ParseDuration(health.Interval)
	if interval == 0 {
		interval = 30 * time.Second
	}

	timeout, _ := time.ParseDuration(health.Timeout)
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	retries := health.Retries
	if retries == 0 {
		retries = 3
	}

	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: health.Command,
			},
		},
		InitialDelaySeconds: 10,
		PeriodSeconds:       int32(interval.Seconds()),
		TimeoutSeconds:      int32(timeout.Seconds()),
		FailureThreshold:    int32(retries),
		SuccessThreshold:    1,
	}
}

// buildSidecarResources returns resource requirements for sidecar containers
func (b *PodSpecBuilder) buildSidecarResources(name string) corev1.ResourceRequirements {
	// Default resources for sidecars
	cpu := "500m"
	memory := "512Mi"

	// Adjust based on service type
	switch name {
	case "postgres":
		cpu = "1"
		memory = "1Gi"
	case "redis":
		cpu = "500m"
		memory = "256Mi"
	}

	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cpu),
			corev1.ResourceMemory: resource.MustParse(memory),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cpu),
			corev1.ResourceMemory: resource.MustParse(memory),
		},
	}
}