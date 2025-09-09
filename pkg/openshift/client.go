package openshift

import (
	"context"
	"fmt"
	"strings"

	"github.com/eliorerz/ovim-updated/pkg/config"
	templatev1 "github.com/openshift/api/template/v1"
	templateclient "github.com/openshift/client-go/template/clientset/versioned/typed/template/v1"
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
	tmplList, err := c.templateClient.Templates(c.config.TemplateNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list templates: %w", err)
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
		ID:          string(tmpl.UID),
		Name:        tmpl.Name,
		Namespace:   tmpl.Namespace,
		Description: tmpl.Annotations["openshift.io/description"],
		OSType:      "Unknown",
		CPU:         1,
		Memory:      "2Gi",
		DiskSize:    "20Gi",
		ImageURL:    "",
	}

	// Extract description from various annotation keys
	if template.Description == "" {
		if desc := tmpl.Annotations["description"]; desc != "" {
			template.Description = desc
		} else if desc := tmpl.Annotations["openshift.io/display-name"]; desc != "" {
			template.Description = desc
		}
	}

	// Determine OS type from labels
	for label, val := range tmpl.Labels {
		if strings.HasPrefix(label, "os.template.kubevirt.io/") && val == "true" {
			osName := strings.TrimPrefix(label, "os.template.kubevirt.io/")
			template.OSType = strings.Title(osName)
			template.OSVersion = template.OSType // Simple mapping for now
			break
		}
	}

	// Determine flavor (CPU/Memory) from labels
	if tmpl.Labels["flavor.template.kubevirt.io/tiny"] == "true" {
		template.CPU = 1
		template.Memory = "1Gi"
	} else if tmpl.Labels["flavor.template.kubevirt.io/small"] == "true" {
		template.CPU = 1
		template.Memory = "2Gi"
	} else if tmpl.Labels["flavor.template.kubevirt.io/medium"] == "true" {
		template.CPU = 1
		template.Memory = "4Gi"
	} else if tmpl.Labels["flavor.template.kubevirt.io/large"] == "true" {
		template.CPU = 2
		template.Memory = "8Gi"
	}

	return template
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
