package acm

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Client represents an ACM hub cluster client
type Client struct {
	kubeClient kubernetes.Interface
	dynClient  client.Client
	config     *rest.Config
	namespace  string
	scheme     *runtime.Scheme
}

// ClientOptions contains options for creating an ACM client
type ClientOptions struct {
	Kubeconfig string
	Namespace  string
	Timeout    time.Duration
}

// NewClient creates a new ACM client
func NewClient(opts ClientOptions) (*Client, error) {
	if opts.Namespace == "" {
		opts.Namespace = "open-cluster-management"
	}

	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Second
	}

	// Load kubeconfig
	var config *rest.Config
	var err error

	if opts.Kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", opts.Kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to load kubeconfig from %s: %w", opts.Kubeconfig, err)
		}
	} else {
		// Try in-cluster config
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to load in-cluster config: %w", err)
		}
	}

	// Configure client settings
	config.Timeout = opts.Timeout
	config.QPS = 20
	config.Burst = 30

	// Create Kubernetes client
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	// Create dynamic client for custom resources
	scheme := runtime.NewScheme()
	if err := addManagedClusterToScheme(scheme); err != nil {
		return nil, fmt.Errorf("failed to add ManagedCluster to scheme: %w", err)
	}

	dynClient, err := client.New(config, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	acmClient := &Client{
		kubeClient: kubeClient,
		dynClient:  dynClient,
		config:     config,
		namespace:  opts.Namespace,
		scheme:     scheme,
	}

	// Test connection
	if err := acmClient.healthCheck(); err != nil {
		return nil, fmt.Errorf("ACM client health check failed: %w", err)
	}

	klog.Infof("ACM client initialized successfully, namespace: %s", opts.Namespace)
	return acmClient, nil
}

// healthCheck verifies the client can connect to the ACM hub
func (c *Client) healthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Check if we can reach the Kubernetes API
	_, err := c.kubeClient.Discovery().ServerVersion()
	if err != nil {
		return fmt.Errorf("failed to reach Kubernetes API: %w", err)
	}

	// Check if ACM CRDs are available
	_, err = c.kubeClient.Discovery().ServerResourcesForGroupVersion("cluster.open-cluster-management.io/v1")
	if err != nil {
		klog.Warningf("ACM CRDs may not be available: %v", err)
		// Don't fail on this as ACM might not be fully installed yet
	}

	// Try to list namespaces to verify basic access
	_, err = c.kubeClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list namespaces: %w", err)
	}

	return nil
}

// ListManagedClusters retrieves all managed clusters from ACM
func (c *Client) ListManagedClusters(ctx context.Context) (*ManagedClusterList, error) {
	clusters := &ManagedClusterList{}

	// List ManagedCluster resources
	err := c.dynClient.List(ctx, clusters)
	if err != nil {
		return nil, fmt.Errorf("failed to list managed clusters: %w", err)
	}

	klog.V(4).Infof("Found %d managed clusters", len(clusters.Items))
	return clusters, nil
}

// GetManagedCluster retrieves a specific managed cluster by name
func (c *Client) GetManagedCluster(ctx context.Context, name string) (*ManagedCluster, error) {
	cluster := &ManagedCluster{}

	err := c.dynClient.Get(ctx, client.ObjectKey{Name: name}, cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to get managed cluster %s: %w", name, err)
	}

	return cluster, nil
}

// GetClusterInfo processes a ManagedCluster into simplified ClusterInfo
func (c *Client) GetClusterInfo(cluster *ManagedCluster) *ClusterInfo {
	info := &ClusterInfo{
		Name:        cluster.Name,
		DisplayName: cluster.Name,
		Labels:      make(map[string]string),
		Annotations: make(map[string]string),
		Claims:      make(map[string]string),
		LastSeen:    time.Now(),
	}

	// Copy labels and annotations
	if cluster.Labels != nil {
		for k, v := range cluster.Labels {
			info.Labels[k] = v
		}
	}
	if cluster.Annotations != nil {
		for k, v := range cluster.Annotations {
			info.Annotations[k] = v
		}
	}

	// Extract API endpoint
	if len(cluster.Spec.ManagedClusterClientConfigs) > 0 {
		info.APIEndpoint = cluster.Spec.ManagedClusterClientConfigs[0].URL
	}

	// Process cluster status and conditions
	info.Accepted = cluster.Spec.HubAcceptsClient
	info.Available = c.isClusterAvailable(cluster)
	info.Status = c.getClusterStatus(cluster)

	// Extract Kubernetes version
	info.KubeVersion = cluster.Status.Version.Kubernetes

	// Process cluster claims for additional metadata
	for _, claim := range cluster.Status.ClusterClaims {
		info.Claims[claim.Name] = claim.Value

		switch claim.Name {
		case ClusterClaimProvider:
			info.Provider = claim.Value
		case ClusterClaimRegion:
			info.Region = claim.Value
		case ClusterClaimNodeCount:
			if count, err := parseIntClaim(claim.Value); err == nil {
				info.NodeCount = count
			}
		case ClusterClaimCPUCores:
			if cores, err := parseIntClaim(claim.Value); err == nil {
				info.CPUCores = cores
			}
		case ClusterClaimMemoryGB:
			if memory, err := parseIntClaim(claim.Value); err == nil {
				info.MemoryGB = memory
			}
		case ClusterClaimStorageGB:
			if storage, err := parseIntClaim(claim.Value); err == nil {
				info.StorageGB = storage
			}
		}
	}

	// Extract from labels if not found in claims
	if info.Provider == "" {
		info.Provider = info.Labels[LabelClusterProvider]
	}
	if info.Region == "" {
		info.Region = info.Labels[LabelClusterRegion]
	}

	// Try to extract capacity from status if not available in claims
	if info.CPUCores == 0 || info.MemoryGB == 0 || info.StorageGB == 0 {
		c.extractCapacityFromStatus(cluster, info)
	}

	// Set display name from labels or keep cluster name
	if displayName := info.Labels["cluster.open-cluster-management.io/display-name"]; displayName != "" {
		info.DisplayName = displayName
	}

	return info
}

// isClusterAvailable checks if the cluster is available based on conditions
func (c *Client) isClusterAvailable(cluster *ManagedCluster) bool {
	for _, condition := range cluster.Status.Conditions {
		if condition.Type == string(ManagedClusterConditionAvailable) {
			return condition.Status == "True"
		}
	}
	return false
}

// getClusterStatus determines the overall cluster status
func (c *Client) getClusterStatus(cluster *ManagedCluster) string {
	// Check if hub accepts the cluster
	if !cluster.Spec.HubAcceptsClient {
		return "pending-acceptance"
	}

	// Check availability condition
	for _, condition := range cluster.Status.Conditions {
		switch condition.Type {
		case string(ManagedClusterConditionAvailable):
			if condition.Status == "True" {
				return "available"
			} else {
				return "unavailable"
			}
		case string(ManagedClusterConditionJoined):
			if condition.Status != "True" {
				return "joining"
			}
		}
	}

	// Check for taints that might affect availability
	for _, taint := range cluster.Spec.Taints {
		if taint.Effect == TaintEffectNoSelect {
			return "maintenance"
		}
	}

	return "unknown"
}

// extractCapacityFromStatus tries to extract capacity from cluster status
func (c *Client) extractCapacityFromStatus(cluster *ManagedCluster, info *ClusterInfo) {
	if cluster.Status.Capacity != nil {
		// Try to parse CPU capacity
		if cpuStr, exists := cluster.Status.Capacity["cpu"]; exists {
			if cores, err := parseResourceQuantity(cpuStr); err == nil {
				info.CPUCores = cores
			}
		}

		// Try to parse memory capacity
		if memStr, exists := cluster.Status.Capacity["memory"]; exists {
			if memGB, err := parseMemoryToGB(memStr); err == nil {
				info.MemoryGB = memGB
			}
		}

		// Try to parse storage capacity
		if storageStr, exists := cluster.Status.Capacity["ephemeral-storage"]; exists {
			if storageGB, err := parseStorageToGB(storageStr); err == nil {
				info.StorageGB = storageGB
			}
		}
	}
}

// Close closes the ACM client connections
func (c *Client) Close() error {
	// In this implementation, we don't have persistent connections to close
	// But this method provides a hook for cleanup if needed in the future
	klog.Info("ACM client closed")
	return nil
}

// GetNamespace returns the ACM namespace the client is configured for
func (c *Client) GetNamespace() string {
	return c.namespace
}

// GetConfig returns the Kubernetes rest config
func (c *Client) GetConfig() *rest.Config {
	return c.config
}

// addManagedClusterToScheme adds the ManagedCluster type to the scheme
func addManagedClusterToScheme(scheme *runtime.Scheme) error {
	// Add the ManagedCluster GroupVersionKind
	gvk := schema.GroupVersionKind{
		Group:   "cluster.open-cluster-management.io",
		Version: "v1",
		Kind:    "ManagedCluster",
	}

	// Register the type
	scheme.AddKnownTypeWithName(gvk, &ManagedCluster{})
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{
			Group:   "cluster.open-cluster-management.io",
			Version: "v1",
			Kind:    "ManagedClusterList",
		},
		&ManagedClusterList{},
	)

	return nil
}
