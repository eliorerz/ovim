package openshift

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/eliorerz/ovim-updated/pkg/config"
	"github.com/eliorerz/ovim-updated/pkg/kubevirt"
	"github.com/eliorerz/ovim-updated/pkg/models"
	templatev1 "github.com/openshift/api/template/v1"
	templateclient "github.com/openshift/client-go/template/clientset/versioned/typed/template/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Template represents a VM template from OpenShift
type Template struct {
	ID           string `json:"id"`
	Name         string `json:"name"`         // Display name for UI
	TemplateName string `json:"templateName"` // Actual OpenShift template name
	Description  string `json:"description"`
	OSType       string `json:"osType"`
	OSVersion    string `json:"osVersion"`
	CPU          int    `json:"cpu"`
	Memory       string `json:"memory"`
	DiskSize     string `json:"diskSize"`
	Namespace    string `json:"namespace"`
	ImageURL     string `json:"imageUrl"`
	IconClass    string `json:"iconClass"`
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
	VDCID           string `json:"vdcId"` // Required VDC selection for resource validation
}

// Client provides OpenShift integration capabilities
type Client struct {
	config         *config.OpenShiftConfig
	kubeClient     kubernetes.Interface
	templateClient templateclient.TemplateV1Interface
	dynamicClient  dynamic.Interface
	restConfig     *rest.Config
	kubeVirtClient kubevirt.VMProvisioner
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

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// Create controller-runtime client for KubeVirt
	k8sClient, err := client.New(restConfig, client.Options{})
	if err != nil {
		return nil, fmt.Errorf("failed to create controller-runtime client: %w", err)
	}

	kubeVirtClient, err := kubevirt.NewClient(restConfig, k8sClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubevirt client: %w", err)
	}

	return &Client{
		config:         cfg,
		kubeClient:     kubeClient,
		templateClient: templateClient,
		dynamicClient:  dynamicClient,
		restConfig:     restConfig,
		kubeVirtClient: kubeVirtClient,
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
		ID:           string(tmpl.UID),
		Name:         c.extractDisplayName(tmpl),
		TemplateName: tmpl.Name, // Actual OpenShift template name
		Namespace:    tmpl.Namespace,
		OSType:       "Unknown",
		CPU:          1,
		Memory:       "2Gi",
		DiskSize:     "20Gi",
		ImageURL:     "",
	}

	// Extract description from various annotation keys
	template.Description = c.extractDescription(tmpl)

	// Determine OS type and version
	template.OSType, template.OSVersion = c.extractOSInfo(tmpl)

	// Determine flavor (CPU/Memory) from labels
	template.CPU, template.Memory = c.extractResourceInfo(tmpl)

	// Extract image URL and icon class separately
	template.ImageURL, template.IconClass = c.extractImageInfo(tmpl)

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

// extractImageInfo extracts both image URL and icon class from template annotations
func (c *Client) extractImageInfo(tmpl *templatev1.Template) (string, string) {
	var imageURL, iconClass string

	// Check for actual image URLs first (these are real image URLs)
	if images := tmpl.Annotations["template.kubevirt.io/images"]; images != "" {
		// This contains actual image references
		imageURL = images
	}

	// Check for container disk images (these are also real image URLs)
	if containerDisks := tmpl.Annotations["template.kubevirt.io/containerdisks"]; containerDisks != "" && imageURL == "" {
		imageURL = containerDisks
	}

	// Check for icon class annotation (FontAwesome icons go to iconClass)
	if iconClassValue := tmpl.Annotations["iconClass"]; iconClassValue != "" {
		iconClass = iconClassValue
	}

	// If no icon class found, determine from tags or template name
	if iconClass == "" {
		iconClass = c.determineIconClass(tmpl)
	}

	return imageURL, iconClass
}

// determineIconClass determines appropriate icon class based on template metadata
func (c *Client) determineIconClass(tmpl *templatev1.Template) string {
	// Look for tag-based icon information
	if tags := tmpl.Annotations["tags"]; tags != "" {
		lowerTags := strings.ToLower(tags)
		if strings.Contains(lowerTags, "rhel") || strings.Contains(lowerTags, "red hat") {
			return "fa fa-redhat"
		} else if strings.Contains(lowerTags, "ubuntu") {
			return "fa fa-ubuntu"
		} else if strings.Contains(lowerTags, "centos") {
			return "fa fa-centos"
		} else if strings.Contains(lowerTags, "fedora") {
			return "fa fa-fedora"
		} else if strings.Contains(lowerTags, "windows") {
			return "fa fa-windows"
		}
	}

	// Fallback to template name-based icon determination
	templateName := strings.ToLower(tmpl.Name)

	// Check template name for common patterns
	if strings.Contains(templateName, "cache") || strings.Contains(templateName, "redis") {
		return "fa fa-database"
	} else if strings.Contains(templateName, "mysql") || strings.Contains(templateName, "mariadb") {
		return "fa fa-database"
	} else if strings.Contains(templateName, "postgresql") || strings.Contains(templateName, "postgres") {
		return "fa fa-database"
	} else if strings.Contains(templateName, "mongodb") || strings.Contains(templateName, "mongo") {
		return "fa fa-database"
	} else if strings.Contains(templateName, "php") || strings.Contains(templateName, "cake") {
		return "fa fa-code"
	} else if strings.Contains(templateName, "java") || strings.Contains(templateName, "spring") {
		return "fa fa-code"
	} else if strings.Contains(templateName, "nodejs") || strings.Contains(templateName, "node") {
		return "fa fa-code"
	} else if strings.Contains(templateName, "python") || strings.Contains(templateName, "django") {
		return "fa fa-code"
	} else if strings.Contains(templateName, "rhel") || strings.Contains(templateName, "red-hat") {
		return "fa fa-redhat"
	} else if strings.Contains(templateName, "centos") {
		return "fa fa-centos"
	} else if strings.Contains(templateName, "ubuntu") {
		return "fa fa-ubuntu"
	} else if strings.Contains(templateName, "fedora") {
		return "fa fa-fedora"
	} else if strings.Contains(templateName, "windows") {
		return "fa fa-windows"
	}

	// Final fallback based on general category
	if strings.Contains(templateName, "vm") {
		return "fa fa-desktop"
	}

	return "fa fa-cube" // Default for applications
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
			// Sanitize VM name to ensure Kubernetes compliance
			tmpl.Parameters[i].Value = c.sanitizeKubernetesName(req.VMName)
		case "NAMESPACE":
			tmpl.Parameters[i].Value = req.TargetNamespace
		case "SIZE":
			if req.DiskSize != "" {
				tmpl.Parameters[i].Value = req.DiskSize
			}
		}
	}

	// Process the template to create Kubernetes objects
	processed, err := c.processTemplate(ctx, tmpl)
	if err != nil {
		return nil, fmt.Errorf("failed to process template %s: %w", req.TemplateName, err)
	}

	// Deploy the processed objects to the target namespace
	err = c.deployObjects(ctx, processed, req.TargetNamespace)
	if err != nil {
		return nil, fmt.Errorf("failed to deploy objects from template %s: %w", req.TemplateName, err)
	}

	// Return the VM information
	vm := &VirtualMachine{
		ID:        fmt.Sprintf("vm-%s", req.VMName),
		Name:      req.VMName,
		Status:    "Provisioning",
		Namespace: req.TargetNamespace,
		Template:  req.TemplateName,
		Created:   time.Now().Format(time.RFC3339),
	}

	return vm, nil
}

// GetVMs retrieves deployed VMs from OpenShift
func (c *Client) GetVMs(ctx context.Context, namespace string) ([]VirtualMachine, error) {
	if c.dynamicClient == nil {
		return nil, fmt.Errorf("dynamic client not initialized")
	}

	// Define the KubeVirt VirtualMachine resource
	vmGVR := schema.GroupVersionResource{
		Group:    "kubevirt.io",
		Version:  "v1",
		Resource: "virtualmachines",
	}

	// List VirtualMachines from the specified namespace
	vmList, err := c.dynamicClient.Resource(vmGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		// If KubeVirt is not installed or VMs not found, return empty list instead of error
		if errors.IsNotFound(err) || strings.Contains(err.Error(), "no matches for kind") {
			return []VirtualMachine{}, nil
		}
		return nil, fmt.Errorf("failed to list VMs from namespace %s: %w", namespace, err)
	}

	vms := make([]VirtualMachine, 0, len(vmList.Items))
	for _, item := range vmList.Items {
		vm := c.convertKubeVirtVM(&item)
		vms = append(vms, vm)
	}

	return vms, nil
}

// convertKubeVirtVM converts a KubeVirt VirtualMachine to our VirtualMachine struct
func (c *Client) convertKubeVirtVM(obj *unstructured.Unstructured) VirtualMachine {
	vm := VirtualMachine{
		ID:        string(obj.GetUID()),
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Template:  "",
		Created:   obj.GetCreationTimestamp().UTC().Format(time.RFC3339),
	}

	// Extract status information
	status, found, err := unstructured.NestedString(obj.Object, "status", "phase")
	if err == nil && found {
		vm.Status = status
	} else {
		// Fallback to printableStatus if phase is not available
		printableStatus, found, err := unstructured.NestedString(obj.Object, "status", "printableStatus")
		if err == nil && found {
			vm.Status = printableStatus
		} else {
			vm.Status = "Unknown"
		}
	}

	// Try to extract template information from labels or annotations
	if templateName := obj.GetLabels()["vm.kubevirt.io/template"]; templateName != "" {
		vm.Template = templateName
	} else if templateName := obj.GetAnnotations()["vm.kubevirt.io/template"]; templateName != "" {
		vm.Template = templateName
	} else {
		vm.Template = "Custom"
	}

	return vm
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

// CreateLimitRange creates a LimitRange for enforcing per-VM resource limits
func (c *Client) CreateLimitRange(ctx context.Context, namespace string, minCPU, maxCPU, minMemory, maxMemory int) error {
	if c.kubeClient == nil {
		return fmt.Errorf("kubernetes client not initialized")
	}

	// Check if LimitRange already exists (idempotent behavior)
	_, err := c.kubeClient.CoreV1().LimitRanges(namespace).Get(ctx, "vm-limits", metav1.GetOptions{})
	if err == nil {
		// LimitRange already exists, nothing to do
		return nil
	}
	if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check if LimitRange exists: %w", err)
	}

	// Create LimitRange object with per-VM constraints using provided parameters
	limitRange := &corev1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vm-limits",
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "ovim",
				"app.kubernetes.io/component":  "limitrange",
				"app.kubernetes.io/managed-by": "ovim",
			},
		},
		Spec: corev1.LimitRangeSpec{
			Limits: []corev1.LimitRangeItem{
				{
					Type: corev1.LimitTypeContainer,
					Default: corev1.ResourceList{
						"cpu":    resource.MustParse(fmt.Sprintf("%dm", maxCPU*1000)), // Default to max CPU
						"memory": resource.MustParse(fmt.Sprintf("%dGi", maxMemory)),  // Default to max memory
					},
					DefaultRequest: corev1.ResourceList{
						"cpu":    resource.MustParse(fmt.Sprintf("%dm", minCPU*1000)), // Default request to min CPU
						"memory": resource.MustParse(fmt.Sprintf("%dGi", minMemory)),  // Default request to min memory
					},
					Min: corev1.ResourceList{
						"cpu":    resource.MustParse(fmt.Sprintf("%dm", minCPU*1000)), // Minimum CPU from parameters
						"memory": resource.MustParse(fmt.Sprintf("%dGi", minMemory)),  // Minimum memory from parameters
					},
					Max: corev1.ResourceList{
						"cpu":    resource.MustParse(fmt.Sprintf("%dm", maxCPU*1000)), // Maximum CPU from parameters
						"memory": resource.MustParse(fmt.Sprintf("%dGi", maxMemory)),  // Maximum memory from parameters
					},
				},
			},
		},
	}

	// Create the LimitRange
	_, err = c.kubeClient.CoreV1().LimitRanges(namespace).Create(ctx, limitRange, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create LimitRange for namespace %s: %w", namespace, err)
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
		resourceQuota.Spec.Hard["requests.cpu"] = resource.MustParse(fmt.Sprintf("%dm", cpuQuota*1000))
		resourceQuota.Spec.Hard["limits.cpu"] = resource.MustParse(fmt.Sprintf("%dm", cpuQuota*1000))
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

// extractImageURL is a stub method for test compatibility
func (c *Client) extractImageURL(template *templatev1.Template) string {
	// This is a stub implementation to fix test compilation
	// TODO: Implement actual image URL extraction logic
	return ""
}

// UpdateLimitRange updates an existing LimitRange or creates it if it doesn't exist
func (c *Client) UpdateLimitRange(ctx context.Context, namespace string, minCPU, maxCPU, minMemory, maxMemory int) error {
	if c.kubeClient == nil {
		return fmt.Errorf("kubernetes client not initialized")
	}

	// Try to get existing LimitRange first
	_, err := c.kubeClient.CoreV1().LimitRanges(namespace).Get(ctx, "vm-limits", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// LimitRange doesn't exist, create it
			return c.CreateLimitRange(ctx, namespace, minCPU, maxCPU, minMemory, maxMemory)
		}
		return fmt.Errorf("failed to check existing LimitRange: %w", err)
	}

	// LimitRange exists, update it
	limitRange := &corev1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vm-limits",
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "ovim",
				"app.kubernetes.io/component":  "limitrange",
				"app.kubernetes.io/managed-by": "ovim",
			},
		},
		Spec: corev1.LimitRangeSpec{
			Limits: []corev1.LimitRangeItem{
				{
					Type: corev1.LimitTypeContainer,
					Default: corev1.ResourceList{
						"cpu":    resource.MustParse(fmt.Sprintf("%dm", maxCPU*1000)),
						"memory": resource.MustParse(fmt.Sprintf("%dGi", maxMemory)),
					},
					DefaultRequest: corev1.ResourceList{
						"cpu":    resource.MustParse(fmt.Sprintf("%dm", minCPU*1000)),
						"memory": resource.MustParse(fmt.Sprintf("%dGi", minMemory)),
					},
					Min: corev1.ResourceList{
						"cpu":    resource.MustParse(fmt.Sprintf("%dm", minCPU*1000)),
						"memory": resource.MustParse(fmt.Sprintf("%dGi", minMemory)),
					},
					Max: corev1.ResourceList{
						"cpu":    resource.MustParse(fmt.Sprintf("%dm", maxCPU*1000)),
						"memory": resource.MustParse(fmt.Sprintf("%dGi", maxMemory)),
					},
				},
			},
		},
	}

	// Update the LimitRange
	_, err = c.kubeClient.CoreV1().LimitRanges(namespace).Update(ctx, limitRange, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update LimitRange for namespace %s: %w", namespace, err)
	}

	return nil
}

// DeleteLimitRange deletes the LimitRange for a namespace
func (c *Client) DeleteLimitRange(ctx context.Context, namespace string) error {
	if c.kubeClient == nil {
		return fmt.Errorf("kubernetes client not initialized")
	}

	err := c.kubeClient.CoreV1().LimitRanges(namespace).Delete(ctx, "vm-limits", metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// LimitRange doesn't exist, that's OK
			return nil
		}
		return fmt.Errorf("failed to delete LimitRange for namespace %s: %w", namespace, err)
	}

	return nil
}

// GetLimitRange retrieves the current LimitRange information for a namespace
func (c *Client) GetLimitRange(ctx context.Context, namespace string) (*models.LimitRangeInfo, error) {
	if c.kubeClient == nil {
		return nil, fmt.Errorf("kubernetes client not initialized")
	}

	limitRange, err := c.kubeClient.CoreV1().LimitRanges(namespace).Get(ctx, "vm-limits", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// LimitRange doesn't exist
			return &models.LimitRangeInfo{
				Exists:    false,
				MinCPU:    0,
				MaxCPU:    0,
				MinMemory: 0,
				MaxMemory: 0,
			}, nil
		}
		return nil, fmt.Errorf("failed to get LimitRange for namespace %s: %w", namespace, err)
	}

	// Parse the LimitRange to extract values
	info := &models.LimitRangeInfo{
		Exists: true,
	}

	// Find the container limit item
	for _, limit := range limitRange.Spec.Limits {
		if limit.Type == corev1.LimitTypeContainer {
			// Extract CPU values (convert from millicores to cores)
			if minCPU, exists := limit.Min["cpu"]; exists {
				info.MinCPU = int(minCPU.MilliValue() / 1000)
			}
			if maxCPU, exists := limit.Max["cpu"]; exists {
				info.MaxCPU = int(maxCPU.MilliValue() / 1000)
			}

			// Extract memory values (parse as GB)
			if minMemory, exists := limit.Min["memory"]; exists {
				// Convert bytes to GB (using binary GB = 1024^3 bytes)
				info.MinMemory = int(minMemory.Value() / (1024 * 1024 * 1024))
			}
			if maxMemory, exists := limit.Max["memory"]; exists {
				// Convert bytes to GB (using binary GB = 1024^3 bytes)
				info.MaxMemory = int(maxMemory.Value() / (1024 * 1024 * 1024))
			}
			break
		}
	}

	return info, nil
}

// processTemplate processes an OpenShift template to generate Kubernetes objects
func (c *Client) processTemplate(ctx context.Context, tmpl *templatev1.Template) (*templatev1.Template, error) {
	if c.templateClient == nil {
		return nil, fmt.Errorf("template client not initialized")
	}

	// Create a copy of the template for processing
	processedTemplate := tmpl.DeepCopy()

	// Perform parameter substitution in template objects
	err := c.substituteTemplateParameters(processedTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to substitute template parameters: %w", err)
	}

	return processedTemplate, nil
}

// deployObjects deploys the processed template objects to the target namespace
func (c *Client) deployObjects(ctx context.Context, tmpl *templatev1.Template, namespace string) error {
	if c.dynamicClient == nil {
		return fmt.Errorf("dynamic client not initialized")
	}

	// Deploy each object from the template
	for _, obj := range tmpl.Objects {
		// Convert runtime.RawExtension to unstructured.Unstructured
		unstructuredObj, err := c.rawToUnstructured(&obj)
		if err != nil {
			return fmt.Errorf("failed to convert template object to unstructured: %w", err)
		}

		// Set the namespace for the object
		unstructuredObj.SetNamespace(namespace)

		// Get the Group, Version, Resource for this object
		gvk := unstructuredObj.GroupVersionKind()
		gvr, err := c.getGVRFromGVK(gvk)
		if err != nil {
			return fmt.Errorf("failed to get GVR for object %s/%s: %w", gvk.Kind, unstructuredObj.GetName(), err)
		}

		// Create the object in the cluster
		_, err = c.dynamicClient.Resource(gvr).Namespace(namespace).Create(ctx, unstructuredObj, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create object %s/%s in namespace %s: %w", gvk.Kind, unstructuredObj.GetName(), namespace, err)
		}
	}

	return nil
}

// rawToUnstructured converts a runtime.RawExtension to unstructured.Unstructured
func (c *Client) rawToUnstructured(raw *runtime.RawExtension) (*unstructured.Unstructured, error) {
	// Create a new unstructured object
	unstructuredObj := &unstructured.Unstructured{}

	// Unmarshal the raw JSON into the unstructured object
	err := unstructuredObj.UnmarshalJSON(raw.Raw)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal raw extension to unstructured: %w", err)
	}

	return unstructuredObj, nil
}

// getGVRFromGVK converts GroupVersionKind to GroupVersionResource
func (c *Client) getGVRFromGVK(gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	// Map common kinds to resources
	kindToResource := map[string]string{
		"VirtualMachine":         "virtualmachines",
		"VirtualMachineInstance": "virtualmachineinstances",
		"DataVolume":             "datavolumes",
		"PersistentVolumeClaim":  "persistentvolumeclaims",
		"Secret":                 "secrets",
		"ConfigMap":              "configmaps",
		"Service":                "services",
		"ServiceAccount":         "serviceaccounts",
		"Role":                   "roles",
		"RoleBinding":            "rolebindings",
		"Deployment":             "deployments",
		"Pod":                    "pods",
	}

	resource, exists := kindToResource[gvk.Kind]
	if !exists {
		// Default: convert Kind to lowercase + s
		resource = strings.ToLower(gvk.Kind) + "s"
	}

	return schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: resource,
	}, nil
}

// substituteTemplateParameters performs parameter substitution in template objects
func (c *Client) substituteTemplateParameters(tmpl *templatev1.Template) error {
	// Create a map of parameter names to values
	paramMap := make(map[string]string)
	for _, param := range tmpl.Parameters {
		if param.Value != "" {
			paramMap[param.Name] = param.Value
		}
	}

	// Substitute parameters in each object
	for i, obj := range tmpl.Objects {
		// Convert to JSON for easy string replacement
		objBytes, err := obj.MarshalJSON()
		if err != nil {
			return fmt.Errorf("failed to marshal object %d: %w", i, err)
		}

		objString := string(objBytes)

		// Replace parameter placeholders
		for paramName, paramValue := range paramMap {
			placeholder := fmt.Sprintf("${%s}", paramName)
			objString = strings.ReplaceAll(objString, placeholder, paramValue)
		}

		// Convert back to RawExtension
		tmpl.Objects[i].Raw = []byte(objString)
	}

	return nil
}

// sanitizeKubernetesName sanitizes a name to comply with Kubernetes RFC 1123 subdomain naming rules
func (c *Client) sanitizeKubernetesName(name string) string {
	if name == "" {
		return "vm"
	}

	// Convert to lowercase and replace invalid chars with hyphens
	sanitized := strings.ToLower(name)
	sanitized = regexp.MustCompile(`[^a-z0-9-]`).ReplaceAllString(sanitized, "-")
	sanitized = regexp.MustCompile(`-+`).ReplaceAllString(sanitized, "-")
	sanitized = strings.Trim(sanitized, "-")

	if sanitized == "" {
		sanitized = "vm"
	}

	// Ensure it doesn't exceed 63 characters
	if len(sanitized) > 63 {
		sanitized = sanitized[:63]
		sanitized = strings.TrimRight(sanitized, "-")
	}

	return sanitized
}

// VM Power Management Operations

// StartVM starts a virtual machine via KubeVirt
func (c *Client) StartVM(ctx context.Context, vmID, namespace string) error {
	return c.kubeVirtClient.StartVM(ctx, vmID, namespace)
}

// StopVM stops a virtual machine via KubeVirt
func (c *Client) StopVM(ctx context.Context, vmID, namespace string) error {
	return c.kubeVirtClient.StopVM(ctx, vmID, namespace)
}

// RestartVM restarts a virtual machine via KubeVirt
func (c *Client) RestartVM(ctx context.Context, vmID, namespace string) error {
	return c.kubeVirtClient.RestartVM(ctx, vmID, namespace)
}

// DeleteVM deletes a virtual machine via KubeVirt
func (c *Client) DeleteVM(ctx context.Context, vmID, namespace string) error {
	return c.kubeVirtClient.DeleteVM(ctx, vmID, namespace)
}

// GetVMStatus gets the status of a virtual machine via KubeVirt
func (c *Client) GetVMStatus(ctx context.Context, vmID, namespace string) (*kubevirt.VMStatus, error) {
	return c.kubeVirtClient.GetVMStatus(ctx, vmID, namespace)
}

// GetVMConsoleURL gets the console URL for a virtual machine via KubeVirt
func (c *Client) GetVMConsoleURL(ctx context.Context, vmID, namespace string) (string, error) {
	return c.kubeVirtClient.GetVMConsoleURL(ctx, vmID, namespace)
}
