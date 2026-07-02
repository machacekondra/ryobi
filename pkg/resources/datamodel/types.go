package datamodel

import "time"

// Environment represents a deployment target.
type Environment struct {
	ID         string                `json:"id"`
	Name       string                `json:"name"`
	Type       string                `json:"type"`
	Properties EnvironmentProperties `json:"properties"`
	CreatedAt  time.Time             `json:"createdAt,omitempty"`
	UpdatedAt  time.Time             `json:"updatedAt,omitempty"`
}

// EnvironmentProperties holds environment configuration.
type EnvironmentProperties struct {
	Providers    map[string]ProviderConfig          `json:"providers,omitempty"`
	Recipes      map[string]map[string]RecipeConfig  `json:"recipes,omitempty"`
	RecipeConfig *RecipeGlobalConfig                `json:"recipeConfig,omitempty"`
}

// ProviderConfig holds cloud provider configuration.
type ProviderConfig struct {
	Scope string `json:"scope,omitempty"`
}

// RecipeConfig defines a recipe template.
type RecipeConfig struct {
	TemplateKind string         `json:"templateKind"`
	TemplatePath string         `json:"templatePath"`
	Parameters   map[string]any `json:"parameters,omitempty"`
}

// RecipeGlobalConfig holds global recipe configuration.
type RecipeGlobalConfig struct {
	Terraform *TerraformConfig `json:"terraform,omitempty"`
}

// TerraformConfig holds Terraform-specific configuration.
type TerraformConfig struct {
	Providers      map[string]map[string]any `json:"providers,omitempty"`
	Authentication map[string]any            `json:"authentication,omitempty"`
}

// Application represents a deployed application.
type Application struct {
	ID         string                `json:"id"`
	Name       string                `json:"name"`
	Type       string                `json:"type"`
	Properties ApplicationProperties `json:"properties"`
	CreatedAt  time.Time             `json:"createdAt,omitempty"`
	UpdatedAt  time.Time             `json:"updatedAt,omitempty"`
}

// ApplicationProperties holds application configuration.
type ApplicationProperties struct {
	Environment string            `json:"environment"`
	Status      ApplicationStatus `json:"status,omitempty"`
}

// ApplicationStatus represents the current status of an application.
type ApplicationStatus struct {
	State string `json:"state,omitempty"`
}

// Resource represents a Terraform-managed resource within an application.
type Resource struct {
	ID         string             `json:"id"`
	Name       string             `json:"name"`
	Type       string             `json:"type"`
	Properties ResourceProperties `json:"properties"`
	CreatedAt  time.Time          `json:"createdAt,omitempty"`
	UpdatedAt  time.Time          `json:"updatedAt,omitempty"`
}

// ResourceProperties holds resource configuration.
type ResourceProperties struct {
	ResourceType string           `json:"resourceType"`
	RecipeName   string           `json:"recipeName"`
	Parameters   map[string]any   `json:"parameters,omitempty"`
	Connections  []ConnectionRef  `json:"connections,omitempty"`
	Status       ResourceStatus   `json:"status,omitempty"`
}

// ConnectionRef references another resource's outputs.
type ConnectionRef struct {
	Name   string `json:"name"`
	Source string `json:"source"`
	Key    string `json:"key,omitempty"`
}

// ResourceStatus represents the status of a Terraform-managed resource.
type ResourceStatus struct {
	State           string         `json:"state,omitempty"`
	Outputs         map[string]any `json:"outputs,omitempty"`
	OutputResources []string       `json:"outputResources,omitempty"`
}

// Resource state constants.
const (
	StatePending   = "Pending"
	StateDeploying = "Deploying"
	StateSucceeded = "Succeeded"
	StateFailed    = "Failed"
	StateDeleting  = "Deleting"
)

// Resource type constants.
const (
	EnvironmentResourceType = "ryobi/environments"
	ApplicationResourceType = "ryobi/applications"
	ResourceResourceType    = "ryobi/resources"
)

// Well-known resource type identifiers for recipes.
const (
	ResourceTypeVirtualMachine = "Ryobi.Compute/virtualMachines"
	ResourceTypeContainer      = "Ryobi.Compute/containers"
)

// VirtualMachineProperties holds VM-specific configuration passed as recipe parameters.
// This serves as documentation for the expected parameter schema when using
// the Ryobi.Compute/virtualMachines resource type with the kubevirt-vm recipe.
type VirtualMachineProperties struct {
	// Name of the virtual machine
	Name string `json:"name"`

	// Namespace for the VM (default: "default")
	Namespace string `json:"namespace,omitempty"`

	// CPU configuration
	CPUCores   int `json:"cpu_cores,omitempty"`
	CPUSockets int `json:"cpu_sockets,omitempty"`
	CPUThreads int `json:"cpu_threads,omitempty"`

	// Memory request (e.g. "1Gi", "512Mi")
	Memory      string `json:"memory,omitempty"`
	MemoryLimit string `json:"memory_limit,omitempty"`

	// Disk image from container registry (e.g. "docker://quay.io/containerdisks/fedora:latest")
	DiskImage      string `json:"disk_image,omitempty"`
	DiskSize       string `json:"disk_size,omitempty"`
	DiskBus        string `json:"disk_bus,omitempty"`
	StorageClass   string `json:"storage_class,omitempty"`

	// Network type: "masquerade" or "bridge"
	NetworkType string `json:"network_type,omitempty"`

	// Cloud-init configuration
	CloudInitEnabled  bool   `json:"cloud_init_enabled,omitempty"`
	CloudInitUserData string `json:"cloud_init_user_data,omitempty"`

	// Service exposure
	ServicePorts []ServicePort `json:"service_ports,omitempty"`
	ServiceType  string        `json:"service_type,omitempty"`

	// Whether the VM should be started immediately
	Running bool `json:"running,omitempty"`
}

// ServicePort defines a port to expose via a Kubernetes Service.
type ServicePort struct {
	Name       string `json:"name"`
	Port       int    `json:"port"`
	TargetPort int    `json:"target_port"`
	Protocol   string `json:"protocol,omitempty"`
}

// ContainerProperties holds container-specific configuration passed as recipe parameters.
// This serves as documentation for the expected parameter schema when using
// the Ryobi.Compute/containers resource type with the kubernetes-pod recipe.
type ContainerProperties struct {
	// Name of the deployment
	Name string `json:"name"`

	// Container image (e.g. "nginx:1.25")
	Image string `json:"image"`

	// Namespace for the deployment (default: "default")
	Namespace string `json:"namespace,omitempty"`

	// Number of replicas
	Replicas int `json:"replicas,omitempty"`

	// Ports to expose
	Ports []ContainerPort `json:"ports,omitempty"`

	// Environment variables
	Env []ContainerEnvVar `json:"env,omitempty"`

	// Config data injected as a ConfigMap
	ConfigData map[string]string `json:"config_data,omitempty"`

	// CPU and memory requests/limits
	CPURequest    string `json:"cpu_request,omitempty"`
	CPULimit      string `json:"cpu_limit,omitempty"`
	MemoryRequest string `json:"memory_request,omitempty"`
	MemoryLimit   string `json:"memory_limit,omitempty"`

	// Volume mounts
	Volumes []ContainerVolume `json:"volumes,omitempty"`

	// Health check
	HealthCheckPath string `json:"health_check_path,omitempty"`
	HealthCheckPort int    `json:"health_check_port,omitempty"`

	// Service exposure
	ServiceType string `json:"service_type,omitempty"`

	// Ingress
	IngressHost      string `json:"ingress_host,omitempty"`
	IngressPath      string `json:"ingress_path,omitempty"`
	IngressClass     string `json:"ingress_class,omitempty"`
	IngressTLSSecret string `json:"ingress_tls_secret,omitempty"`
}

// ContainerPort defines a port exposed by the container.
type ContainerPort struct {
	ContainerPort int    `json:"container_port"`
	ServicePort   int    `json:"service_port,omitempty"`
	Protocol      string `json:"protocol,omitempty"`
	Name          string `json:"name,omitempty"`
}

// ContainerEnvVar defines an environment variable for the container.
type ContainerEnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value,omitempty"`
}

// ContainerVolume defines a volume to mount into the container.
type ContainerVolume struct {
	Name         string `json:"name"`
	MountPath    string `json:"mount_path"`
	Size         string `json:"size,omitempty"`
	StorageClass string `json:"storage_class,omitempty"`
	ReadOnly     bool   `json:"read_only,omitempty"`
}
