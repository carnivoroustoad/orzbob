package scheduler

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// buildVolumes creates the volume specifications
func (b *PodSpecBuilder) buildVolumes() []corev1.Volume {
	volumes := []corev1.Volume{
		{
			Name: "workspace",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: fmt.Sprintf("%s-workspace", b.config.Name),
				},
			},
		},
		{
			Name: "cache",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: fmt.Sprintf("%s-cache", b.config.Name),
				},
			},
		},
		{
			Name: "scripts",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: fmt.Sprintf("%s-scripts", b.config.Name),
					},
					DefaultMode: intPtr(0755),
				},
			},
		},
	}

	return volumes
}

// buildContainers creates the main container
func (b *PodSpecBuilder) buildContainers() []corev1.Container {
	return []corev1.Container{
		{
			Name:            "runner",
			Image:           b.config.Image,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Resources:       b.buildResourceRequirements(),
			VolumeMounts:    b.buildVolumeMounts(),
			Env:             b.buildEnvVars(),
			Command:         []string{"/usr/local/bin/cloud-agent"},
			Args:            []string{},
			WorkingDir:      "/workspace",
			SecurityContext: b.buildContainerSecurityContext(),
		},
	}
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
		{
			Name:      "scripts",
			MountPath: "/scripts",
			ReadOnly:  true,
		},
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
	return []corev1.EnvVar{
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