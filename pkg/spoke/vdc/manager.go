package vdc

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ovimv1 "github.com/eliorerz/ovim-updated/pkg/api/v1"
	"github.com/eliorerz/ovim-updated/pkg/spoke"
	"github.com/eliorerz/ovim-updated/pkg/spoke/config"
)

// Manager implements the VDCManager interface for Kubernetes
type Manager struct {
	k8sClient    client.Client
	k8sClientset kubernetes.Interface
	logger       *slog.Logger
	config       *config.SpokeConfig
}

// NewManager creates a new VDC manager
func NewManager(k8sClient client.Client, k8sClientset kubernetes.Interface, logger *slog.Logger, cfg *config.SpokeConfig) *Manager {
	return &Manager{
		k8sClient:    k8sClient,
		k8sClientset: k8sClientset,
		logger:       logger.With("component", "vdc-manager"),
		config:       cfg,
	}
}

func (m *Manager) createVDCNamespace(ctx context.Context, req *spoke.VDCCreateRequest) (*corev1.Namespace, error) {
	// Create namespace for the VDC
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: req.Name,
			Labels: map[string]string{
				"ovim.io/vdc":          "true",
				"ovim.io/organization": req.OrganizationName,
				"ovim.io/managed-by":   "ovim-spoke-agent",
			},
			Annotations: req.Annotations,
		},
	}

	// Merge additional labels
	if req.Labels != nil {
		for k, v := range req.Labels {
			namespace.Labels[k] = v
		}
	}

	// Create the namespace
	if err := m.k8sClient.Create(ctx, namespace); err != nil {
		return nil, fmt.Errorf("failed to create namespace %s: %w", req.Name, err)
	}

	m.logger.Info("Created namespace for VDC", "namespace", req.Name)
	return namespace, nil
}

// CreateVDC creates a new VDC with namespace, RBAC, and resource quotas atomically
func (m *Manager) CreateVDC(ctx context.Context, req *spoke.VDCCreateRequest) (*spoke.VDCStatus, error) {
	m.logger.Info("Creating VDC",
		"name", req.Name,
		"organization", req.OrganizationName,
		"cpu_quota", req.CPUQuota,
		"memory_quota", req.MemoryQuota,
		"storage_quota", req.StorageQuota)

	// Track created resources for rollback
	var createdNamespace bool

	// Rollback function to clean up on failure
	rollback := func() {
		if createdNamespace {
			m.logger.Warn("Rolling back VDC creation, deleting namespace", "namespace", req.Name)
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: req.Name},
			}
			if err := m.k8sClient.Delete(ctx, ns); err != nil {
				m.logger.Error("Failed to rollback namespace deletion", "namespace", req.Name, "error", err)
			}
		}
	}

	namespace, err := m.createVDCNamespace(ctx, req)
	if err != nil {
		return nil, err
	}
	createdNamespace = true

	// Note: VirtualDataCenter CRD is managed by the hub, not the spoke
	// The spoke agent only manages the workload namespace and its resources

	// Create ResourceQuota
	if err := m.createResourceQuota(ctx, req); err != nil {
		rollback()
		return nil, fmt.Errorf("failed to create resource quota: %w", err)
	}

	// Create LimitRange
	if err := m.createLimitRange(ctx, req); err != nil {
		rollback()
		return nil, fmt.Errorf("failed to create limit range: %w", err)
	}

	// Create RBAC for VDC (mandatory - fail if it fails)
	if err := m.createVDCRBAC(ctx, req); err != nil {
		rollback()
		return nil, fmt.Errorf("failed to create VDC RBAC: %w", err)
	}

	// Create NetworkPolicy if specified
	if req.NetworkPolicy != "" {
		if err := m.createNetworkPolicy(ctx, req); err != nil {
			rollback()
			return nil, fmt.Errorf("failed to create network policy: %w", err)
		}
	}

	m.logger.Info("VDC created successfully", "name", req.Name, "namespace", req.Name)

	// Return VDC status
	return &spoke.VDCStatus{
		Name:      req.Name,
		Namespace: req.Name,
		Status:    "Active",
		Labels:    namespace.Labels,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		ResourceUsage: spoke.ResourceMetrics{
			CPUUsed:         0,
			CPUCapacity:     req.CPUQuota * 1000, // Convert cores to millicores
			MemoryUsed:      0,
			MemoryCapacity:  req.MemoryQuota * 1024 * 1024 * 1024, // Convert GB to bytes
			StorageUsed:     0,
			StorageCapacity: req.StorageQuota * 1024 * 1024 * 1024, // Convert GB to bytes
			VMCount:         0,
		},
	}, nil
}

// createResourceQuota creates resource quotas for the VDC namespace
func (m *Manager) createResourceQuota(ctx context.Context, req *spoke.VDCCreateRequest) error {
	quota := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vdc-quota",
			Namespace: req.Name,
			Labels: map[string]string{
				"ovim.io/vdc":        "true",
				"ovim.io/managed-by": "ovim-spoke-agent",
			},
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: corev1.ResourceList{
				"requests.cpu":                      resource.MustParse(fmt.Sprintf("%d", req.CPUQuota)),
				"requests.memory":                   resource.MustParse(fmt.Sprintf("%dGi", req.MemoryQuota)),
				"persistentvolumeclaims":            resource.MustParse("10"),
				"requests.storage":                  resource.MustParse(fmt.Sprintf("%dGi", req.StorageQuota)),
				"count/virtualmachines.kubevirt.io": resource.MustParse("50"), // Max 50 VMs per VDC
			},
		},
	}

	if err := m.k8sClient.Create(ctx, quota); err != nil {
		return fmt.Errorf("failed to create resource quota: %w", err)
	}

	m.logger.Info("Created resource quota for VDC", "namespace", req.Name)
	return nil
}

// createLimitRange creates limit ranges for the VDC namespace
func (m *Manager) createLimitRange(ctx context.Context, req *spoke.VDCCreateRequest) error {
	// Use provided LimitRange values or calculate defaults from quotas
	var maxCPU, maxMemory string
	var minCPU, minMemory string

	// Determine max limits
	if req.MaxCPU != nil {
		maxCPU = fmt.Sprintf("%dm", *req.MaxCPU*1000) // Convert cores to millicores
	} else {
		maxCPU = fmt.Sprintf("%d", req.CPUQuota) // fallback to quota
	}

	if req.MaxMemory != nil {
		maxMemory = fmt.Sprintf("%dMi", *req.MaxMemory*1024) // Convert GB to MiB
	} else {
		maxMemory = fmt.Sprintf("%dGi", req.MemoryQuota) // fallback to quota in GiB
	}

	// Calculate safe DefaultRequest values that respect the LimitRange constraints
	var defaultRequestCPU, defaultRequestMemory string

	// For DefaultRequest, use a conservative value that won't exceed min/max constraints
	if req.MinCPU != nil && req.MaxCPU != nil {
		// Use the minimum value to ensure it's always valid
		defaultRequestCPU = fmt.Sprintf("%dm", *req.MinCPU*1000) // Convert cores to millicores
	} else if req.MaxCPU != nil {
		// Use 10% of max CPU (in cores), or minimum 0.1 cores (100m)
		defaultCPUCores := *req.MaxCPU / 10
		if defaultCPUCores < 1 { // Less than 1 core
			defaultRequestCPU = "100m" // Minimum 100 millicores
		} else {
			defaultRequestCPU = fmt.Sprintf("%dm", defaultCPUCores*1000) // Convert cores to millicores
		}
	} else {
		defaultRequestCPU = "100m" // fallback
	}

	if req.MinMemory != nil && req.MaxMemory != nil {
		// Use the minimum value to ensure it's always valid
		defaultRequestMemory = fmt.Sprintf("%dMi", *req.MinMemory*1024) // Convert GB to MiB
	} else if req.MaxMemory != nil {
		// Use 10% of max memory (in GB), or minimum 256Mi
		defaultMemoryGB := *req.MaxMemory / 10
		if defaultMemoryGB < 1 { // Less than 1GB
			defaultRequestMemory = "256Mi" // Minimum 256 MiB
		} else {
			defaultRequestMemory = fmt.Sprintf("%dMi", defaultMemoryGB*1024) // Convert GB to MiB
		}
	} else {
		defaultRequestMemory = "256Mi" // fallback
	}

	// Determine min limits (optional for Min field in LimitRange)
	limitRangeItem := corev1.LimitRangeItem{
		Type: corev1.LimitTypeContainer,
		DefaultRequest: corev1.ResourceList{
			"cpu":    resource.MustParse(defaultRequestCPU),
			"memory": resource.MustParse(defaultRequestMemory),
		},
		Max: corev1.ResourceList{
			"cpu":    resource.MustParse(maxCPU),
			"memory": resource.MustParse(maxMemory),
		},
	}

	// Add Min constraints if provided
	if req.MinCPU != nil || req.MinMemory != nil {
		limitRangeItem.Min = corev1.ResourceList{}
		if req.MinCPU != nil {
			minCPU = fmt.Sprintf("%dm", *req.MinCPU*1000) // Convert cores to millicores
			limitRangeItem.Min["cpu"] = resource.MustParse(minCPU)
		}
		if req.MinMemory != nil {
			minMemory = fmt.Sprintf("%dMi", *req.MinMemory*1024) // Convert GB to MiB
			limitRangeItem.Min["memory"] = resource.MustParse(minMemory)
		}
	}

	limitRange := &corev1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vdc-limits",
			Namespace: req.Name,
			Labels: map[string]string{
				"ovim.io/vdc":        "true",
				"ovim.io/managed-by": "ovim-spoke-agent",
			},
		},
		Spec: corev1.LimitRangeSpec{
			Limits: []corev1.LimitRangeItem{limitRangeItem},
		},
	}

	if err := m.k8sClient.Create(ctx, limitRange); err != nil {
		return fmt.Errorf("failed to create limit range: %w", err)
	}

	// Log what LimitRange values were used
	if req.MinCPU != nil || req.MaxCPU != nil || req.MinMemory != nil || req.MaxMemory != nil {
		m.logger.Info("Created limit range for VDC using provided values",
			"namespace", req.Name,
			"min_cpu", req.MinCPU,
			"max_cpu", req.MaxCPU,
			"min_memory", req.MinMemory,
			"max_memory", req.MaxMemory)
	} else {
		m.logger.Info("Created limit range for VDC using calculated defaults",
			"namespace", req.Name,
			"default_cpu", defaultRequestCPU,
			"default_memory", defaultRequestMemory,
			"max_cpu", maxCPU,
			"max_memory", maxMemory)
	}
	return nil
}

// createVDCRBAC creates RBAC resources for VDC management
func (m *Manager) createVDCRBAC(ctx context.Context, req *spoke.VDCCreateRequest) error {
	// Create ServiceAccount for VDC operations
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vdc-operator",
			Namespace: req.Name,
			Labels: map[string]string{
				"ovim.io/vdc":        "true",
				"ovim.io/managed-by": "ovim-spoke-agent",
			},
		},
	}

	if err := m.k8sClient.Create(ctx, sa); err != nil {
		if errors.IsForbidden(err) {
			m.logger.Warn("Insufficient permissions to create ServiceAccount, skipping RBAC setup", "error", err)
			return nil // Skip RBAC creation if no permissions
		}
		return fmt.Errorf("failed to create service account: %w", err)
	}

	// Create Role for VDC operations
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vdc-operator",
			Namespace: req.Name,
			Labels: map[string]string{
				"ovim.io/vdc":        "true",
				"ovim.io/managed-by": "ovim-spoke-agent",
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods", "services", "configmaps", "secrets", "persistentvolumeclaims"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
			{
				APIGroups: []string{"kubevirt.io"},
				Resources: []string{"virtualmachines", "virtualmachineinstances"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments", "statefulsets"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
		},
	}

	if err := m.k8sClient.Create(ctx, role); err != nil {
		return fmt.Errorf("failed to create role: %w", err)
	}

	// Create RoleBinding
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vdc-operator",
			Namespace: req.Name,
			Labels: map[string]string{
				"ovim.io/vdc":        "true",
				"ovim.io/managed-by": "ovim-spoke-agent",
			},
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "vdc-operator",
				Namespace: req.Name,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     "vdc-operator",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	if err := m.k8sClient.Create(ctx, roleBinding); err != nil {
		return fmt.Errorf("failed to create role binding: %w", err)
	}

	m.logger.Info("Created RBAC for VDC", "namespace", req.Name)
	return nil
}

// createNetworkPolicy creates network policies for VDC isolation
func (m *Manager) createNetworkPolicy(ctx context.Context, req *spoke.VDCCreateRequest) error {
	// This is a placeholder - network policy implementation depends on CNI
	m.logger.Info("Network policy creation requested but not implemented",
		"policy", req.NetworkPolicy, "namespace", req.Name)
	return nil
}

// DeleteVDC deletes a VDC and all its resources
func (m *Manager) DeleteVDC(ctx context.Context, namespace string) error {
	m.logger.Info("Deleting VDC", "namespace", namespace)

	// Delete the namespace (this will cascade delete all resources)
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}

	if err := m.k8sClient.Delete(ctx, ns); err != nil {
		return fmt.Errorf("failed to delete VDC namespace %s: %w", namespace, err)
	}

	m.logger.Info("VDC deleted successfully", "namespace", namespace)
	return nil
}

// GetVDCStatus returns the status of a VDC
func (m *Manager) GetVDCStatus(ctx context.Context, namespace string) (*spoke.VDCStatus, error) {
	// Get namespace
	ns := &corev1.Namespace{}
	if err := m.k8sClient.Get(ctx, client.ObjectKey{Name: namespace}, ns); err != nil {
		return nil, fmt.Errorf("failed to get VDC namespace %s: %w", namespace, err)
	}

	// Get resource quota
	quota := &corev1.ResourceQuota{}
	if err := m.k8sClient.Get(ctx, client.ObjectKey{Name: "vdc-quota", Namespace: namespace}, quota); err != nil {
		m.logger.Warn("Failed to get resource quota for VDC", "namespace", namespace, "error", err)
	}

	// TODO: Collect actual resource usage from metrics
	status := &spoke.VDCStatus{
		Name:      namespace,
		Namespace: namespace,
		Status:    "Active",
		Labels:    ns.Labels,
		CreatedAt: ns.CreationTimestamp.Time,
		UpdatedAt: time.Now(),
		ResourceUsage: spoke.ResourceMetrics{
			// Placeholder values - should be collected from actual metrics
			CPUUsed:    0,
			MemoryUsed: 0,
			VMCount:    0,
		},
	}

	return status, nil
}

// ListVDCs returns all VDCs managed by this agent
func (m *Manager) ListVDCs(ctx context.Context) ([]spoke.VDCStatus, error) {
	namespaces := &corev1.NamespaceList{}
	if err := m.k8sClient.List(ctx, namespaces, client.MatchingLabels{"ovim.io/vdc": "true"}); err != nil {
		return nil, fmt.Errorf("failed to list VDC namespaces: %w", err)
	}

	var vdcs []spoke.VDCStatus
	for _, ns := range namespaces.Items {
		status, err := m.GetVDCStatus(ctx, ns.Name)
		if err != nil {
			m.logger.Warn("Failed to get VDC status", "namespace", ns.Name, "error", err)
			continue
		}
		vdcs = append(vdcs, *status)
	}

	return vdcs, nil
}

// UpdateVDCQuotas updates resource quotas for a VDC
func (m *Manager) UpdateVDCQuotas(ctx context.Context, namespace string, cpuQuota, memoryQuota, storageQuota int64) error {
	quota := &corev1.ResourceQuota{}
	if err := m.k8sClient.Get(ctx, client.ObjectKey{Name: "vdc-quota", Namespace: namespace}, quota); err != nil {
		return fmt.Errorf("failed to get resource quota for VDC %s: %w", namespace, err)
	}

	// Update quota values
	quota.Spec.Hard["requests.cpu"] = resource.MustParse(fmt.Sprintf("%d", cpuQuota))
	quota.Spec.Hard["requests.memory"] = resource.MustParse(fmt.Sprintf("%dGi", memoryQuota))
	quota.Spec.Hard["requests.storage"] = resource.MustParse(fmt.Sprintf("%dGi", storageQuota))

	if err := m.k8sClient.Update(ctx, quota); err != nil {
		return fmt.Errorf("failed to update resource quota for VDC %s: %w", namespace, err)
	}

	m.logger.Info("Updated VDC quotas", "namespace", namespace,
		"cpu", cpuQuota, "memory", memoryQuota, "storage", storageQuota)
	return nil
}

// --- CR Integration Methods for Dual API Support ---

// EnsureOrganizationNamespace creates an organization namespace if it doesn't exist
func (m *Manager) EnsureOrganizationNamespace(ctx context.Context, organizationName, clusterID string) error {
	orgNamespace := fmt.Sprintf("org-%s", organizationName)

	// Check if namespace already exists
	ns := &corev1.Namespace{}
	err := m.k8sClient.Get(ctx, client.ObjectKey{Name: orgNamespace}, ns)
	if err == nil {
		// Namespace exists, just update labels if needed
		m.logger.Debug("Organization namespace already exists", "namespace", orgNamespace)
		return m.updateOrganizationNamespace(ctx, ns, organizationName, clusterID)
	}

	if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check organization namespace %s: %w", orgNamespace, err)
	}

	// Create organization namespace
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: orgNamespace,
			Labels: map[string]string{
				"ovim.io/organization":   organizationName,
				"ovim.io/namespace-type": "organization",
				"ovim.io/managed-by":     "spoke-agent",
				"ovim.io/cluster-id":     clusterID,
			},
			Annotations: map[string]string{
				"ovim.io/created-by": "spoke-vdc-manager",
				"ovim.io/created-at": time.Now().Format(time.RFC3339),
			},
		},
	}

	if err := m.k8sClient.Create(ctx, namespace); err != nil {
		return fmt.Errorf("failed to create organization namespace %s: %w", orgNamespace, err)
	}

	m.logger.Info("Created organization namespace", "namespace", orgNamespace, "organization", organizationName)
	return nil
}

// updateOrganizationNamespace updates labels and annotations on existing org namespace
func (m *Manager) updateOrganizationNamespace(ctx context.Context, ns *corev1.Namespace, organizationName, clusterID string) error {
	updated := false

	// Ensure required labels exist
	if ns.Labels == nil {
		ns.Labels = make(map[string]string)
	}
	requiredLabels := map[string]string{
		"ovim.io/organization":   organizationName,
		"ovim.io/namespace-type": "organization",
		"ovim.io/managed-by":     "spoke-agent",
		"ovim.io/cluster-id":     clusterID,
	}

	for key, value := range requiredLabels {
		if ns.Labels[key] != value {
			ns.Labels[key] = value
			updated = true
		}
	}

	if updated {
		if err := m.k8sClient.Update(ctx, ns); err != nil {
			return fmt.Errorf("failed to update organization namespace %s: %w", ns.Name, err)
		}
		m.logger.Debug("Updated organization namespace labels", "namespace", ns.Name)
	}

	return nil
}

// CreateVDCWorkloadNamespace creates a workload namespace for a VDC within an organization
func (m *Manager) CreateVDCWorkloadNamespace(ctx context.Context, vdc *ovimv1.VirtualDataCenter) error {
	workloadNamespace := vdc.Spec.TargetNamespace
	if workloadNamespace == "" {
		workloadNamespace = fmt.Sprintf("vdc-%s-%s", vdc.Spec.OrganizationRef, vdc.Name)
	}

	// Check if namespace already exists
	ns := &corev1.Namespace{}
	err := m.k8sClient.Get(ctx, client.ObjectKey{Name: workloadNamespace}, ns)
	if err == nil {
		m.logger.Debug("Workload namespace already exists", "namespace", workloadNamespace)
		return nil
	}

	if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check workload namespace %s: %w", workloadNamespace, err)
	}

	// Create workload namespace
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: workloadNamespace,
			Labels: map[string]string{
				"ovim.io/vdc":            vdc.Name,
				"ovim.io/organization":   vdc.Spec.OrganizationRef,
				"ovim.io/namespace-type": "workload",
				"ovim.io/managed-by":     "spoke-agent",
				"ovim.io/zone-id":        vdc.Spec.ZoneID,
			},
			Annotations: map[string]string{
				"ovim.io/created-by":       "spoke-vdc-controller",
				"ovim.io/created-at":       time.Now().Format(time.RFC3339),
				"ovim.io/vdc-display-name": vdc.Spec.DisplayName,
				"ovim.io/vdc-description":  vdc.Spec.Description,
			},
		},
	}

	if err := m.k8sClient.Create(ctx, namespace); err != nil {
		return fmt.Errorf("failed to create workload namespace %s: %w", workloadNamespace, err)
	}

	// Create VDC resources in the workload namespace
	vdcReq := &spoke.VDCCreateRequest{
		Name:             workloadNamespace,
		OrganizationName: vdc.Spec.OrganizationRef,
		CPUQuota:         m.parseQuotaInt64(vdc.Spec.Quota.CPU),
		MemoryQuota:      m.parseQuotaInt64(vdc.Spec.Quota.Memory),
		StorageQuota:     m.parseQuotaInt64(vdc.Spec.Quota.Storage),
		NetworkPolicy:    vdc.Spec.NetworkPolicy,
		Labels: map[string]string{
			"ovim.io/vdc-name":   vdc.Name,
			"ovim.io/created-by": "spoke-controller",
		},
		Annotations: map[string]string{
			"ovim.io/vdc-display-name": vdc.Spec.DisplayName,
			"ovim.io/vdc-description":  vdc.Spec.Description,
		},
	}

	// Create resource quota
	if err := m.createResourceQuota(ctx, vdcReq); err != nil {
		return fmt.Errorf("failed to create resource quota for VDC workload namespace: %w", err)
	}

	// Create limit range if specified
	if vdc.Spec.LimitRange != nil {
		minCPU := vdc.Spec.LimitRange.MinCpu
		maxCPU := vdc.Spec.LimitRange.MaxCpu
		minMemory := vdc.Spec.LimitRange.MinMemory
		maxMemory := vdc.Spec.LimitRange.MaxMemory

		vdcReq.MinCPU = &minCPU
		vdcReq.MaxCPU = &maxCPU
		vdcReq.MinMemory = &minMemory
		vdcReq.MaxMemory = &maxMemory

		if err := m.createLimitRange(ctx, vdcReq); err != nil {
			return fmt.Errorf("failed to create limit range for VDC workload namespace: %w", err)
		}
	}

	// Create RBAC
	if err := m.createVDCRBAC(ctx, vdcReq); err != nil {
		return fmt.Errorf("failed to create RBAC for VDC workload namespace: %w", err)
	}

	// Create NetworkPolicy if specified
	if vdc.Spec.NetworkPolicy != "" {
		if err := m.createNetworkPolicy(ctx, vdcReq); err != nil {
			return fmt.Errorf("failed to create network policy for VDC workload namespace: %w", err)
		}
	}

	m.logger.Info("Created VDC workload namespace with resources",
		"namespace", workloadNamespace,
		"vdc", vdc.Name,
		"organization", vdc.Spec.OrganizationRef)

	return nil
}

// DeleteVDCWorkloadNamespace deletes a VDC workload namespace
func (m *Manager) DeleteVDCWorkloadNamespace(ctx context.Context, workloadNamespace string) error {
	if workloadNamespace == "" {
		return fmt.Errorf("workload namespace name cannot be empty")
	}

	m.logger.Info("Deleting VDC workload namespace", "namespace", workloadNamespace)

	// Delete the namespace (this will cascade delete all resources)
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: workloadNamespace,
		},
	}

	if err := m.k8sClient.Delete(ctx, ns); err != nil {
		if errors.IsNotFound(err) {
			m.logger.Info("Workload namespace already deleted", "namespace", workloadNamespace)
			return nil
		}
		return fmt.Errorf("failed to delete VDC workload namespace %s: %w", workloadNamespace, err)
	}

	m.logger.Info("VDC workload namespace deleted successfully", "namespace", workloadNamespace)
	return nil
}

// CanDeleteOrganizationNamespace checks if an organization namespace can be safely deleted
func (m *Manager) CanDeleteOrganizationNamespace(ctx context.Context, orgNamespace, excludeVDC string) (bool, error) {
	// List all VDCs in the organization namespace
	vdcList := &ovimv1.VirtualDataCenterList{}
	if err := m.k8sClient.List(ctx, vdcList, client.InNamespace(orgNamespace)); err != nil {
		return false, fmt.Errorf("failed to list VDCs in namespace %s: %w", orgNamespace, err)
	}

	// Check if there are other VDCs besides the one being excluded
	for _, vdc := range vdcList.Items {
		if vdc.Name != excludeVDC && vdc.DeletionTimestamp.IsZero() {
			return false, nil // Other VDCs still exist
		}
	}

	return true, nil // No other VDCs found, safe to delete
}

// DeleteOrganizationNamespace deletes an organization namespace if it's safe to do so
func (m *Manager) DeleteOrganizationNamespace(ctx context.Context, orgNamespace string) error {
	m.logger.Info("Deleting organization namespace", "namespace", orgNamespace)

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: orgNamespace,
		},
	}

	if err := m.k8sClient.Delete(ctx, ns); err != nil {
		if errors.IsNotFound(err) {
			m.logger.Info("Organization namespace already deleted", "namespace", orgNamespace)
			return nil
		}
		return fmt.Errorf("failed to delete organization namespace %s: %w", orgNamespace, err)
	}

	m.logger.Info("Organization namespace deleted successfully", "namespace", orgNamespace)
	return nil
}

// parseQuotaInt64 parses a quota string and returns an int64 value
func (m *Manager) parseQuotaInt64(quotaStr string) int64 {
	quotaStr = strings.TrimSpace(quotaStr)
	if quotaStr == "" {
		return 1
	}

	// Handle different suffix formats
	if strings.HasSuffix(quotaStr, "Ti") {
		quotaStr = strings.TrimSuffix(quotaStr, "Ti")
		if val, err := strconv.ParseFloat(quotaStr, 64); err == nil {
			return int64(val * 1024) // Convert Ti to Gi for consistency
		}
	} else if strings.HasSuffix(quotaStr, "Gi") {
		quotaStr = strings.TrimSuffix(quotaStr, "Gi")
		if val, err := strconv.ParseFloat(quotaStr, 64); err == nil {
			return int64(val)
		}
	} else if strings.HasSuffix(quotaStr, "G") {
		quotaStr = strings.TrimSuffix(quotaStr, "G")
		if val, err := strconv.ParseFloat(quotaStr, 64); err == nil {
			return int64(val)
		}
	} else {
		// Try to parse as a plain number
		if val, err := strconv.ParseFloat(quotaStr, 64); err == nil {
			return int64(val)
		}
	}

	m.logger.Warn("Failed to parse quota format, using default", "quota", quotaStr)
	return 1
}
