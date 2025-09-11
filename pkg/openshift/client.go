package openshift

import (
	"context"
	"fmt"
	"strings"

	"github.com/eliorerz/ovim-updated/pkg/config"
	templatev1 "github.com/openshift/api/template/v1"
	templateclient "github.com/openshift/client-go/template/clientset/versioned/typed/template/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Template represents a VM template from OpenShift
type Template struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	OSType      string `json:"osType"`
	OSVersion   string `json:"osVersion"`
	CPU         int    `json:"cpu"`
	Memory      string `json:"memory"`
	DiskSize    string `json:"diskSize"`
	Namespace   string `json:"namespace"`
	ImageURL    string `json:"imageUrl"`
}

// VirtualMachine represents a VM instance
type VirtualMachine struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Namespace string `json:"namespace"`
	Template  string `json:"template"`
	Created   string `json:"created"`
}

// DeployVMRequest represents a VM deployment request
type DeployVMRequest struct {
	TemplateName    string `json:"templateName"`
	VMName          string `json:"vmName"`
	TargetNamespace string `json:"targetNamespace"`
	DiskSize        string `json:"diskSize"`
}

// Client provides OpenShift integration capabilities
type Client struct {
	config         *config.OpenShiftConfig
	kubeClient     kubernetes.Interface
	templateClient templateclient.TemplateV1Interface
	restConfig     *rest.Config
}

// NewClient creates a new OpenShift client
func NewClient(cfg *config.OpenShiftConfig) (*Client, error) {
	var restConfig *rest.Config
	var err error

	if cfg.InCluster {
		restConfig, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to create in-cluster config: %w", err)
		}
	} else {
		kubeconfig := cfg.ConfigPath
		if kubeconfig == "" {
			kubeconfig = clientcmd.RecommendedHomeFile
		}

		restConfig, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to build config from flags: %w", err)
		}
	}

	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	templateClient, err := templateclient.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create template client: %w", err)
	}

	return &Client{
		config:         cfg,
		kubeClient:     kubeClient,
		templateClient: templateClient,
		restConfig:     restConfig,
	}, nil
}

// GetTemplates retrieves available VM templates from OpenShift
func (c *Client) GetTemplates(ctx context.Context) ([]Template, error) {
	return c.GetTemplatesFromNamespace(ctx, c.config.TemplateNamespace)
}

// GetTemplatesFromNamespace retrieves templates from a specific namespace
func (c *Client) GetTemplatesFromNamespace(ctx context.Context, namespace string) ([]Template, error) {
	tmplList, err := c.templateClient.Templates(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list templates from namespace %s: %w", namespace, err)
	}

	var templates []Template
	for _, tmpl := range tmplList.Items {
		template := c.convertTemplate(&tmpl)
		templates = append(templates, template)
	}

	return templates, nil
}

// convertTemplate converts an OpenShift Template to our Template struct
func (c *Client) convertTemplate(tmpl *templatev1.Template) Template {
	template := Template{
		ID:        string(tmpl.UID),
		Name:      c.extractDisplayName(tmpl),
		Namespace: tmpl.Namespace,
		OSType:    "Unknown",
		CPU:       1,
		Memory:    "2Gi",
		DiskSize:  "20Gi",
		ImageURL:  "",
	}

	// Extract description from various annotation keys
	template.Description = c.extractDescription(tmpl)

	// Determine OS type and version
	template.OSType, template.OSVersion = c.extractOSInfo(tmpl)

	// Determine flavor (CPU/Memory) from labels
	template.CPU, template.Memory = c.extractResourceInfo(tmpl)

	// Extract image URL from annotations
	template.ImageURL = c.extractImageURL(tmpl)

	return template
}

// extractDisplayName extracts the proper display name from template annotations
func (c *Client) extractDisplayName(tmpl *templatev1.Template) string {
	// Try display-name annotation first (this is what OpenShift Console uses)
	if displayName := tmpl.Annotations["openshift.io/display-name"]; displayName != "" {
		return displayName
	}

	// Try name.os.template.kubevirt.io annotation
	if osName := tmpl.Annotations["name.os.template.kubevirt.io"]; osName != "" {
		return osName
	}

	// Try template.openshift.io/long-description for shorter descriptive names
	if longDesc := tmpl.Annotations["template.openshift.io/long-description"]; longDesc != "" && len(longDesc) < 80 {
		return longDesc
	}

	// Fallback: clean up the template name
	return c.cleanupTemplateName(tmpl.Name)
}

// cleanupTemplateName provides basic cleanup of template names as fallback
func (c *Client) cleanupTemplateName(name string) string {
	// Handle empty string
	if name == "" {
		return "VM"
	}

	// Simple cleanup: replace dashes with spaces and title case each word
	cleaned := strings.ReplaceAll(name, "-", " ")
	words := strings.Fields(cleaned)

	for i, word := range words {
		// Capitalize each word but preserve common acronyms/versions
		lowerWord := strings.ToLower(word)
		if len(word) <= 3 && (lowerWord == "vm" || lowerWord == "db" || lowerWord == "api" || lowerWord == "cpu" || lowerWord == "gpu" || lowerWord == "app") {
			// Common acronyms - keep uppercase
			words[i] = strings.ToUpper(word)
		} else if strings.Contains(word, "2k") {
			// Version numbers like "2k22" - keep as-is but title case
			words[i] = strings.Title(word)
		} else if strings.HasPrefix(strings.ToLower(word), "v") && len(word) <= 3 {
			// Version prefixes like "v2", "v3" - uppercase
			words[i] = strings.ToUpper(word)
		} else {
			// Regular words - title case
			words[i] = strings.Title(word)
		}
	}

	result := strings.Join(words, " ")

	// Ensure it ends with "VM" if it doesn't already contain it
	if !strings.Contains(strings.ToLower(result), "vm") {
		result += " VM"
	}

	return result
}

// extractDescription extracts description from template annotations
func (c *Client) extractDescription(tmpl *templatev1.Template) string {
	// Try various description annotation keys in order of preference
	descKeys := []string{
		"openshift.io/description",
		"description",
		"template.openshift.io/long-description",
		"openshift.io/display-name",
	}

	for _, key := range descKeys {
		if desc := tmpl.Annotations[key]; desc != "" {
			return desc
		}
	}

	return "Virtual Machine template"
}

// extractOSInfo determines OS type and version from template metadata
func (c *Client) extractOSInfo(tmpl *templatev1.Template) (string, string) {
	// Try to get OS info from annotations first
	if osType := tmpl.Annotations["os.template.kubevirt.io/name"]; osType != "" {
		osVersion := tmpl.Annotations["os.template.kubevirt.io/version"]
		return osType, osVersion
	}

	// Check template annotations for OS information
	if osInfo := tmpl.Annotations["template.kubevirt.io/operating-system"]; osInfo != "" {
		return osInfo, ""
	}

	// Check for OS labels as fallback
	for label, val := range tmpl.Labels {
		if strings.HasPrefix(label, "os.template.kubevirt.io/") && val == "true" {
			osName := strings.TrimPrefix(label, "os.template.kubevirt.io/")
			// Keep it simple - just clean up the label name
			return strings.Title(strings.ReplaceAll(osName, "_", " ")), ""
		}
	}

	// Final fallback: try to extract from template name
	name := strings.ToLower(tmpl.Name)
	if strings.Contains(name, "rhel") {
		return "Red Hat Enterprise Linux", ""
	} else if strings.Contains(name, "centos") {
		return "CentOS Stream", ""
	} else if strings.Contains(name, "fedora") {
		return "Fedora", ""
	} else if strings.Contains(name, "ubuntu") {
		return "Ubuntu", ""
	} else if strings.Contains(name, "windows") {
		return "Microsoft Windows", ""
	}

	return "Linux", ""
}

// extractImageURL extracts image URL from template annotations
func (c *Client) extractImageURL(tmpl *templatev1.Template) string {

	// Check for icon class annotation (commonly used for FontAwesome icons)
	if iconClass := tmpl.Annotations["iconClass"]; iconClass != "" {
		// Convert FontAwesome icons to image URLs or return the class for CSS
		return iconClass
	}

	// Check for template images annotation
	if images := tmpl.Annotations["template.kubevirt.io/images"]; images != "" {
		// This might contain JSON with image references
		return images
	}

	// Check for container disk images
	if containerDisks := tmpl.Annotations["template.kubevirt.io/containerdisks"]; containerDisks != "" {
		return containerDisks
	}

	// Look for tag-based image information
	if tags := tmpl.Annotations["tags"]; tags != "" {
		// Tags might contain OS information we can use to infer icons
		lowerTags := strings.ToLower(tags)
		if strings.Contains(lowerTags, "rhel") || strings.Contains(lowerTags, "red hat") {
			return "redhat-icon"
		} else if strings.Contains(lowerTags, "ubuntu") {
			return "ubuntu-icon"
		} else if strings.Contains(lowerTags, "centos") {
			return "centos-icon"
		} else if strings.Contains(lowerTags, "fedora") {
			return "fedora-icon"
		} else if strings.Contains(lowerTags, "windows") {
			return "windows-icon"
		}
	}

	// Fallback to OS-based icons using template name and OS info
	templateName := strings.ToLower(tmpl.Name)

	// Check template name for common patterns - return SimpleIcons URLs
	if strings.Contains(templateName, "cache") || strings.Contains(templateName, "redis") {
		return "https://cdn.simpleicons.org/redis"
	} else if strings.Contains(templateName, "mysql") || strings.Contains(templateName, "mariadb") {
		return "https://cdn.simpleicons.org/mysql"
	} else if strings.Contains(templateName, "postgresql") || strings.Contains(templateName, "postgres") {
		return "https://cdn.simpleicons.org/postgresql"
	} else if strings.Contains(templateName, "mongodb") || strings.Contains(templateName, "mongo") {
		return "https://cdn.simpleicons.org/mongodb"
	} else if strings.Contains(templateName, "php") || strings.Contains(templateName, "cake") {
		return "https://cdn.simpleicons.org/php"
	} else if strings.Contains(templateName, "java") || strings.Contains(templateName, "spring") {
		return "https://cdn.simpleicons.org/openjdk"
	} else if strings.Contains(templateName, "nodejs") || strings.Contains(templateName, "node") {
		return "https://cdn.simpleicons.org/nodedotjs"
	} else if strings.Contains(templateName, "python") || strings.Contains(templateName, "django") {
		return "https://cdn.simpleicons.org/python"
	} else if strings.Contains(templateName, "rhel") || strings.Contains(templateName, "red-hat") {
		return "https://cdn.simpleicons.org/redhat"
	} else if strings.Contains(templateName, "centos") {
		return "https://cdn.simpleicons.org/centos"
	} else if strings.Contains(templateName, "ubuntu") {
		return "https://cdn.simpleicons.org/ubuntu"
	} else if strings.Contains(templateName, "fedora") {
		return "https://cdn.simpleicons.org/fedora"
	} else if strings.Contains(templateName, "windows") {
		return "https://cdn.simpleicons.org/windows"
	}

	// Final fallback based on general category
	if strings.Contains(templateName, "vm") {
		return "https://cdn.simpleicons.org/virtualbox"
	}

	return "https://cdn.simpleicons.org/kubernetes" // Default for applications
}

// extractResourceInfo determines CPU and memory from template flavor labels
func (c *Client) extractResourceInfo(tmpl *templatev1.Template) (int, string) {
	// Check flavor labels
	if tmpl.Labels["flavor.template.kubevirt.io/tiny"] == "true" {
		return 1, "1Gi"
	} else if tmpl.Labels["flavor.template.kubevirt.io/small"] == "true" {
		return 1, "2Gi"
	} else if tmpl.Labels["flavor.template.kubevirt.io/medium"] == "true" {
		return 1, "4Gi"
	} else if tmpl.Labels["flavor.template.kubevirt.io/large"] == "true" {
		return 2, "8Gi"
	}

	// Try to extract from template name
	name := strings.ToLower(tmpl.Name)
	if strings.Contains(name, "tiny") {
		return 1, "1Gi"
	} else if strings.Contains(name, "small") {
		return 1, "2Gi"
	} else if strings.Contains(name, "medium") {
		return 1, "4Gi"
	} else if strings.Contains(name, "large") {
		return 2, "8Gi"
	}

	// Default values
	return 1, "2Gi"
}

// DeployVM deploys a new VM from a template
func (c *Client) DeployVM(ctx context.Context, req DeployVMRequest) (*VirtualMachine, error) {
	// Get the template
	tmpl, err := c.templateClient.Templates(c.config.TemplateNamespace).Get(ctx, req.TemplateName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get template %s: %w", req.TemplateName, err)
	}

	// Set template parameters
	for i, param := range tmpl.Parameters {
		switch param.Name {
		case "NAME":
			tmpl.Parameters[i].Value = req.VMName
		case "NAMESPACE":
			tmpl.Parameters[i].Value = req.TargetNamespace
		case "SIZE":
			if req.DiskSize != "" {
				tmpl.Parameters[i].Value = req.DiskSize
			}
		}
	}

	// TODO: Implement full template processing with KubeVirt VM creation
	vm := &VirtualMachine{
		ID:        fmt.Sprintf("vm-%s", req.VMName),
		Name:      req.VMName,
		Status:    "Provisioning",
		Namespace: req.TargetNamespace,
		Template:  req.TemplateName,
		Created:   "2024-01-01T00:00:00Z",
	}

	return vm, nil
}

// GetVMs retrieves deployed VMs from OpenShift
func (c *Client) GetVMs(ctx context.Context, namespace string) ([]VirtualMachine, error) {
	// TODO: Implement KubeVirt VM listing from the cluster
	// For now, return empty list until full implementation
	return []VirtualMachine{}, nil
}

// IsConnected checks if the OpenShift client is properly connected
func (c *Client) IsConnected(ctx context.Context) bool {
	if c.kubeClient == nil {
		return false
	}

	// Try to list namespaces as a connectivity test
	_, err := c.kubeClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 1})
	return err == nil
}

// CreateNamespace creates a new namespace with optional resource quotas
func (c *Client) CreateNamespace(ctx context.Context, name string, labels map[string]string, annotations map[string]string) error {
	if c.kubeClient == nil {
		return fmt.Errorf("kubernetes client not initialized")
	}

	// Create namespace object
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      labels,
			Annotations: annotations,
		},
	}

	// Create the namespace
	_, err := c.kubeClient.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create namespace %s: %w", name, err)
	}

	return nil
}

// CreateResourceQuota creates a resource quota for a namespace
func (c *Client) CreateResourceQuota(ctx context.Context, namespace string, cpuQuota, memoryQuota, storageQuota int) error {
	if c.kubeClient == nil {
		return fmt.Errorf("kubernetes client not initialized")
	}

	// Create resource quota object
	resourceQuota := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "organization-quota",
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "ovim",
				"app.kubernetes.io/component":  "resource-quota",
				"app.kubernetes.io/managed-by": "ovim",
			},
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: corev1.ResourceList{},
		},
	}

	// Set CPU quota if specified
	if cpuQuota > 0 {
		resourceQuota.Spec.Hard["requests.cpu"] = resource.MustParse(fmt.Sprintf("%d", cpuQuota))
		resourceQuota.Spec.Hard["limits.cpu"] = resource.MustParse(fmt.Sprintf("%d", cpuQuota))
	}

	// Set memory quota if specified (convert from GiB to bytes)
	if memoryQuota > 0 {
		resourceQuota.Spec.Hard["requests.memory"] = resource.MustParse(fmt.Sprintf("%dGi", memoryQuota))
		resourceQuota.Spec.Hard["limits.memory"] = resource.MustParse(fmt.Sprintf("%dGi", memoryQuota))
	}

	// Set storage quota if specified
	if storageQuota > 0 {
		resourceQuota.Spec.Hard["requests.storage"] = resource.MustParse(fmt.Sprintf("%dGi", storageQuota))
		resourceQuota.Spec.Hard["persistentvolumeclaims"] = resource.MustParse("10") // Allow up to 10 PVCs
	}

	// Create the resource quota
	_, err := c.kubeClient.CoreV1().ResourceQuotas(namespace).Create(ctx, resourceQuota, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create resource quota for namespace %s: %w", namespace, err)
	}

	return nil
}

// DeleteNamespace deletes a namespace and all its resources
func (c *Client) DeleteNamespace(ctx context.Context, name string) error {
	if c.kubeClient == nil {
		return fmt.Errorf("kubernetes client not initialized")
	}

	// Delete the namespace (this cascades to all resources within it)
	err := c.kubeClient.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete namespace %s: %w", name, err)
	}

	return nil
}

// NamespaceExists checks if a namespace exists
func (c *Client) NamespaceExists(ctx context.Context, name string) (bool, error) {
	if c.kubeClient == nil {
		return false, fmt.Errorf("kubernetes client not initialized")
	}

	_, err := c.kubeClient.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check if namespace %s exists: %w", name, err)
	}

	return true, nil
}
