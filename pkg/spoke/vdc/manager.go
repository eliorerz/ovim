package vdc

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ovimv1 "github.com/eliorerz/ovim-updated/pkg/api/v1"
	"github.com/eliorerz/ovim-updated/pkg/spoke"
	"github.com/eliorerz/ovim-updated/pkg/spoke/config"
)

// Manager handles VDC resource management on the spoke cluster
type Manager struct {
	cfg       *config.SpokeConfig
	logger    *slog.Logger
	k8sClient client.Client
	clientset kubernetes.Interface

	stopCh chan struct{}
}

// NewManager creates a new VDC manager
func NewManager(cfg *config.SpokeConfig, k8sClient client.Client, clientset kubernetes.Interface, logger *slog.Logger) *Manager {
	return &Manager{
		cfg:       cfg,
		logger:    logger.With("component", "vdc-manager"),
		k8sClient: k8sClient,
		clientset: clientset,
		stopCh:    make(chan struct{}),
	}
}

// Start begins watching for VDC replication requests
func (m *Manager) Start(ctx context.Context) error {
	m.logger.Info("🚀 SPOKE AGENT: Starting VDC manager",
		"spoke_zone", m.cfg.ZoneID,
		"spoke_cluster", m.cfg.ClusterID,
		"hub_endpoint", m.cfg.Hub.Endpoint,
		"hub_tls_enabled", m.cfg.Hub.TLSEnabled,
		"watch_interval", "30s")

	go m.watchVDCs(ctx)

	<-ctx.Done()
	return nil
}

// Stop stops the VDC manager
func (m *Manager) Stop() error {
	m.logger.Info("Stopping VDC manager")
	close(m.stopCh)
	return nil
}

// watchVDCs watches for VDC changes that require spoke cluster action
func (m *Manager) watchVDCs(ctx context.Context) {
	m.logger.Info("🔄 SPOKE AGENT: Starting VDC watch loop",
		"interval", "30s",
		"spoke_zone", m.cfg.ZoneID,
		"spoke_cluster", m.cfg.ClusterID)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Process VDCs immediately on startup
	m.logger.Info("🔄 SPOKE AGENT: Running initial VDC processing")
	m.processVDCs(ctx)

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("🔄 SPOKE AGENT: Watch loop stopped (context done)")
			return
		case <-m.stopCh:
			m.logger.Info("🔄 SPOKE AGENT: Watch loop stopped (stop channel)")
			return
		case <-ticker.C:
			m.processVDCs(ctx)
		}
	}
}

// processVDCs checks for VDCs requiring replication to this spoke cluster
func (m *Manager) processVDCs(ctx context.Context) {
	m.logger.Info("🔍 SPOKE AGENT: Starting VDC processing cycle",
		"spoke_zone", m.cfg.ZoneID,
		"spoke_cluster", m.cfg.ClusterID,
		"hub_endpoint", m.cfg.Hub.Endpoint)

	// List all VDCs that need replication to this spoke cluster
	vdcList := &ovimv1.VirtualDataCenterList{}
	if err := m.k8sClient.List(ctx, vdcList); err != nil {
		m.logger.Error("❌ SPOKE AGENT: Failed to list VDCs", "error", err)
		return
	}

	m.logger.Info("📋 SPOKE AGENT: Found VDCs in cluster",
		"total_vdcs", len(vdcList.Items),
		"spoke_zone", m.cfg.ZoneID)

	for i, vdc := range vdcList.Items {
		m.logger.Debug("🔍 SPOKE AGENT: Examining VDC",
			"index", i+1,
			"vdc_name", vdc.Name,
			"vdc_namespace", vdc.Namespace,
			"annotations", vdc.Annotations)

		// Check if this VDC needs replication to this spoke cluster
		if m.shouldReplicateVDC(&vdc) {
			m.logger.Info("✅ SPOKE AGENT: Found VDC requiring replication",
				"vdc", vdc.Name,
				"namespace", vdc.Namespace,
				"target_zone", vdc.Annotations["ovim.io/target-zone"],
				"target_namespace", vdc.Annotations["ovim.io/target-namespace"])
			if err := m.handleVDCReplication(ctx, &vdc); err != nil {
				m.logger.Error("❌ SPOKE AGENT: Failed to replicate VDC", "vdc", vdc.Name, "error", err)
				// TODO: Report error status back to hub
			} else {
				// Create spoke VDC record and update hub status
				if err := m.createSpokeVDC(ctx, &vdc); err != nil {
					m.logger.Error("❌ SPOKE AGENT: Failed to create spoke VDC record", "vdc", vdc.Name, "error", err)
				}
			}
		} else {
			m.logger.Debug("⏭️ SPOKE AGENT: Skipping VDC (no replication needed)",
				"vdc", vdc.Name,
				"replication_required", vdc.Annotations["ovim.io/spoke-replication-required"],
				"target_zone", vdc.Annotations["ovim.io/target-zone"],
				"our_zone", m.cfg.ZoneID)
		}
	}

	m.logger.Debug("📊 SPOKE AGENT: Finished VDC processing cycle")

	// Check for any existing VDC namespaces that need status reporting
	m.reportVDCStatus(ctx)
}

// handleVDCReplication creates the VDC resources in the spoke cluster
func (m *Manager) handleVDCReplication(ctx context.Context, vdc *ovimv1.VirtualDataCenter) error {
	targetNamespace, exists := vdc.Annotations["ovim.io/target-namespace"]
	if !exists {
		return fmt.Errorf("VDC %s missing target namespace annotation", vdc.Name)
	}

	m.logger.Info("Replicating VDC to spoke cluster",
		"vdc", vdc.Name,
		"namespace", targetNamespace,
		"zone", vdc.Spec.ZoneID)

	// Create namespace if it doesn't exist
	if err := m.ensureNamespace(ctx, targetNamespace, vdc); err != nil {
		return fmt.Errorf("failed to ensure namespace: %w", err)
	}

	// Create ResourceQuota
	if err := m.createResourceQuota(ctx, targetNamespace, vdc); err != nil {
		return fmt.Errorf("failed to create ResourceQuota: %w", err)
	}

	// Create LimitRange if specified
	if vdc.Spec.LimitRange != nil {
		if err := m.createLimitRange(ctx, targetNamespace, vdc); err != nil {
			return fmt.Errorf("failed to create LimitRange: %w", err)
		}
	}

	m.logger.Info("VDC replication completed successfully", "vdc", vdc.Name, "namespace", targetNamespace)
	return nil
}

// handleVDCDeletion removes VDC resources from the spoke cluster
func (m *Manager) handleVDCDeletion(ctx context.Context, vdc *ovimv1.VirtualDataCenter) error {
	targetNamespace, exists := vdc.Annotations["ovim.io/target-namespace"]
	if !exists {
		// Try to get from status
		targetNamespace = vdc.Status.Namespace
	}

	if targetNamespace == "" {
		m.logger.Warn("VDC deletion requested but no target namespace found", "vdc", vdc.Name)
		return nil
	}

	m.logger.Info("Deleting VDC from spoke cluster",
		"vdc", vdc.Name,
		"namespace", targetNamespace,
		"zone", vdc.Spec.ZoneID)

	// Delete namespace and all its resources
	if err := m.deleteNamespace(ctx, targetNamespace); err != nil {
		return fmt.Errorf("failed to delete namespace: %w", err)
	}

	m.logger.Info("VDC deletion completed successfully", "vdc", vdc.Name, "namespace", targetNamespace)
	return nil
}

// ensureNamespace creates the VDC namespace if it doesn't exist
func (m *Manager) ensureNamespace(ctx context.Context, namespaceName string, vdc *ovimv1.VirtualDataCenter) error {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespaceName,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "ovim",
				"app.kubernetes.io/component":  "vdc",
				"app.kubernetes.io/managed-by": "ovim-spoke-agent",
				"ovim.io/organization":         vdc.Spec.OrganizationRef,
				"ovim.io/vdc":                  vdc.Name,
				"ovim.io/zone":                 vdc.Spec.ZoneID,
				"type":                         "vdc",
				"org":                          vdc.Spec.OrganizationRef,
				"vdc":                          vdc.Name,
			},
			Annotations: map[string]string{
				"ovim.io/vdc-description": vdc.Spec.Description,
				"ovim.io/created-by":      "ovim-spoke-agent",
				"ovim.io/created-at":      time.Now().Format(time.RFC3339),
				"ovim.io/spoke-cluster":   m.cfg.ClusterID,
				"ovim.io/spoke-zone":      m.cfg.ZoneID,
			},
		},
	}

	if err := m.k8sClient.Create(ctx, namespace); err != nil {
		if errors.IsAlreadyExists(err) {
			m.logger.Debug("Namespace already exists", "namespace", namespaceName)
			return nil
		}
		return err
	}

	m.logger.Info("Created VDC namespace", "namespace", namespaceName)
	return nil
}

// createResourceQuota creates the ResourceQuota for the VDC
func (m *Manager) createResourceQuota(ctx context.Context, namespaceName string, vdc *ovimv1.VirtualDataCenter) error {
	quota := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vdc-quota",
			Namespace: namespaceName,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "ovim",
				"app.kubernetes.io/component":  "vdc",
				"app.kubernetes.io/managed-by": "ovim-spoke-agent",
				"ovim.io/organization":         vdc.Spec.OrganizationRef,
				"ovim.io/vdc":                  vdc.Name,
				"type":                         "vdc-quota",
			},
			Annotations: map[string]string{
				"ovim.io/created-by":    "ovim-spoke-agent",
				"ovim.io/created-at":    time.Now().Format(time.RFC3339),
				"ovim.io/spoke-cluster": m.cfg.ClusterID,
				"ovim.io/spoke-zone":    m.cfg.ZoneID,
			},
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: corev1.ResourceList{
				corev1.ResourceRequestsCPU:                               resource.MustParse(vdc.Spec.Quota.CPU),
				corev1.ResourceRequestsMemory:                            resource.MustParse(vdc.Spec.Quota.Memory),
				corev1.ResourceRequestsStorage:                           resource.MustParse(vdc.Spec.Quota.Storage),
				corev1.ResourceLimitsCPU:                                 resource.MustParse(vdc.Spec.Quota.CPU),
				corev1.ResourceLimitsMemory:                              resource.MustParse(vdc.Spec.Quota.Memory),
				corev1.ResourcePersistentVolumeClaims:                    resource.MustParse("50"),
				corev1.ResourcePods:                                      resource.MustParse("100"),
				corev1.ResourceServices:                                  resource.MustParse("20"),
				corev1.ResourceName("count/virtualmachines.kubevirt.io"): resource.MustParse("50"),
			},
		},
	}

	if err := m.k8sClient.Create(ctx, quota); err != nil {
		if errors.IsAlreadyExists(err) {
			m.logger.Debug("ResourceQuota already exists", "namespace", namespaceName)
			return nil
		}
		return err
	}

	m.logger.Info("Created ResourceQuota", "namespace", namespaceName)
	return nil
}

// createLimitRange creates the LimitRange for the VDC
func (m *Manager) createLimitRange(ctx context.Context, namespaceName string, vdc *ovimv1.VirtualDataCenter) error {
	limitRange := &corev1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vdc-limits",
			Namespace: namespaceName,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "ovim",
				"app.kubernetes.io/component":  "vdc",
				"app.kubernetes.io/managed-by": "ovim-spoke-agent",
				"ovim.io/organization":         vdc.Spec.OrganizationRef,
				"ovim.io/vdc":                  vdc.Name,
				"type":                         "vdc-limits",
			},
			Annotations: map[string]string{
				"ovim.io/created-by":    "ovim-spoke-agent",
				"ovim.io/created-at":    time.Now().Format(time.RFC3339),
				"ovim.io/spoke-cluster": m.cfg.ClusterID,
				"ovim.io/spoke-zone":    m.cfg.ZoneID,
			},
		},
		Spec: corev1.LimitRangeSpec{
			Limits: []corev1.LimitRangeItem{
				{
					Type: corev1.LimitTypeContainer,
					Default: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%dm", vdc.Spec.LimitRange.MaxCpu)),
						corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dMi", vdc.Spec.LimitRange.MaxMemory)),
					},
					DefaultRequest: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%dm", vdc.Spec.LimitRange.MinCpu)),
						corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dMi", vdc.Spec.LimitRange.MinMemory)),
					},
					Min: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%dm", vdc.Spec.LimitRange.MinCpu)),
						corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dMi", vdc.Spec.LimitRange.MinMemory)),
					},
					Max: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%dm", vdc.Spec.LimitRange.MaxCpu)),
						corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dMi", vdc.Spec.LimitRange.MaxMemory)),
					},
				},
			},
		},
	}

	if err := m.k8sClient.Create(ctx, limitRange); err != nil {
		if errors.IsAlreadyExists(err) {
			m.logger.Debug("LimitRange already exists", "namespace", namespaceName)
			return nil
		}
		return err
	}

	m.logger.Info("Created LimitRange", "namespace", namespaceName)
	return nil
}

// deleteNamespace deletes the VDC namespace and all its resources
func (m *Manager) deleteNamespace(ctx context.Context, namespaceName string) error {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespaceName,
		},
	}

	if err := m.k8sClient.Delete(ctx, namespace); err != nil {
		if errors.IsNotFound(err) {
			m.logger.Debug("Namespace already deleted", "namespace", namespaceName)
			return nil
		}
		return err
	}

	m.logger.Info("Deleted VDC namespace", "namespace", namespaceName)
	return nil
}

// Interface implementation methods for spoke.VDCManager

// CreateVDC creates a new VDC from the given request
func (m *Manager) CreateVDC(ctx context.Context, req *spoke.VDCCreateRequest) (*spoke.VDCStatus, error) {
	// This is handled through VDC watching - we don't expose direct creation
	return nil, fmt.Errorf("VDC creation is handled through VDC replication from hub")
}

// DeleteVDC deletes the specified VDC and all its resources
func (m *Manager) DeleteVDC(ctx context.Context, namespace string) error {
	return m.deleteNamespace(ctx, namespace)
}

// GetVDCStatus returns the status of the specified VDC
func (m *Manager) GetVDCStatus(ctx context.Context, namespace string) (*spoke.VDCStatus, error) {
	// Check if namespace exists
	var ns corev1.Namespace
	err := m.k8sClient.Get(ctx, client.ObjectKey{Name: namespace}, &ns)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("VDC namespace %s not found", namespace)
		}
		return nil, err
	}

	status := &spoke.VDCStatus{
		Name:      namespace, // Use namespace as name since we don't have VDC name separately
		Namespace: namespace,
		Status:    "Active",
		CreatedAt: ns.CreationTimestamp.Time,
	}

	// Get resource quota information for resource usage
	quotaList := &corev1.ResourceQuotaList{}
	if err := m.k8sClient.List(ctx, quotaList, client.InNamespace(namespace)); err == nil {
		for _, quota := range quotaList.Items {
			if quota.Name == "vdc-quota" {
				// Extract used and hard resources from quota status
				if cpu, ok := quota.Status.Used[corev1.ResourceRequestsCPU]; ok {
					status.ResourceUsage.CPUUsed = cpu.MilliValue()
				}
				if memory, ok := quota.Status.Used[corev1.ResourceRequestsMemory]; ok {
					status.ResourceUsage.MemoryUsed = memory.Value()
				}
				if cpu, ok := quota.Status.Hard[corev1.ResourceRequestsCPU]; ok {
					status.ResourceUsage.CPUCapacity = cpu.MilliValue()
				}
				if memory, ok := quota.Status.Hard[corev1.ResourceRequestsMemory]; ok {
					status.ResourceUsage.MemoryCapacity = memory.Value()
				}
				break
			}
		}
	}

	return status, nil
}

// ListVDCs returns a list of all VDCs managed by this agent
func (m *Manager) ListVDCs(ctx context.Context) ([]spoke.VDCStatus, error) {
	// List all namespaces with VDC labels
	namespaces := &corev1.NamespaceList{}
	if err := m.k8sClient.List(ctx, namespaces, client.MatchingLabels{
		"app.kubernetes.io/component":  "vdc",
		"app.kubernetes.io/managed-by": "ovim-spoke-agent",
	}); err != nil {
		return nil, err
	}

	var vdcs []spoke.VDCStatus
	for _, ns := range namespaces.Items {
		status, err := m.GetVDCStatus(ctx, ns.Name)
		if err == nil && status != nil {
			vdcs = append(vdcs, *status)
		}
	}

	return vdcs, nil
}

// reportVDCStatus reports the status of VDC namespaces to the hub
func (m *Manager) reportVDCStatus(ctx context.Context) {
	// List VDC namespaces on the spoke cluster
	vdcs, err := m.ListVDCs(ctx)
	if err != nil {
		m.logger.Error("Failed to list VDCs for status reporting", "error", err)
		return
	}

	// Report status for each VDC back to hub (implementation would depend on hub communication)
	for _, vdc := range vdcs {
		m.logger.Debug("VDC status reported", "vdc", vdc.Name, "namespace", vdc.Namespace, "status", vdc.Status)
	}
}

// UpdateVDCQuotas updates the resource quotas for the specified VDC
func (m *Manager) UpdateVDCQuotas(ctx context.Context, namespace string, cpuQuota, memoryQuota, storageQuota int64) error {
	// Get existing resource quota
	quota := &corev1.ResourceQuota{}
	err := m.k8sClient.Get(ctx, client.ObjectKey{Name: "vdc-quota", Namespace: namespace}, quota)
	if err != nil {
		return fmt.Errorf("failed to get resource quota for namespace %s: %w", namespace, err)
	}

	// Update quotas
	quota.Spec.Hard[corev1.ResourceRequestsCPU] = resource.MustParse(fmt.Sprintf("%d", cpuQuota))
	quota.Spec.Hard[corev1.ResourceRequestsMemory] = resource.MustParse(fmt.Sprintf("%dGi", memoryQuota))
	quota.Spec.Hard[corev1.ResourceRequestsStorage] = resource.MustParse(fmt.Sprintf("%dGi", storageQuota))
	quota.Spec.Hard[corev1.ResourceLimitsCPU] = resource.MustParse(fmt.Sprintf("%d", cpuQuota))
	quota.Spec.Hard[corev1.ResourceLimitsMemory] = resource.MustParse(fmt.Sprintf("%dGi", memoryQuota))

	if err := m.k8sClient.Update(ctx, quota); err != nil {
		return fmt.Errorf("failed to update resource quota for namespace %s: %w", namespace, err)
	}

	m.logger.Info("Updated VDC resource quotas", "namespace", namespace, "cpu", cpuQuota, "memory", memoryQuota, "storage", storageQuota)
	return nil
}

// shouldReplicateVDC checks if a VDC should be replicated to this spoke cluster
func (m *Manager) shouldReplicateVDC(vdc *ovimv1.VirtualDataCenter) bool {
	m.logger.Debug("🔍 SPOKE AGENT: Checking if VDC should be replicated",
		"vdc", vdc.Name,
		"our_zone", m.cfg.ZoneID,
		"annotations_count", len(vdc.Annotations))

	// Check if replication is required
	replicationRequired := vdc.Annotations["ovim.io/spoke-replication-required"]
	m.logger.Debug("🔍 SPOKE AGENT: Checking replication requirement",
		"vdc", vdc.Name,
		"replication_required", replicationRequired,
		"expected", "true")
	if replicationRequired != "true" {
		m.logger.Debug("❌ SPOKE AGENT: VDC does not require replication",
			"vdc", vdc.Name,
			"replication_required", replicationRequired)
		return false
	}

	// Check if this VDC targets this spoke's zone
	targetZone := vdc.Annotations["ovim.io/target-zone"]
	m.logger.Debug("🔍 SPOKE AGENT: Checking zone match",
		"vdc", vdc.Name,
		"target_zone", targetZone,
		"our_zone", m.cfg.ZoneID)
	if targetZone != m.cfg.ZoneID {
		m.logger.Debug("❌ SPOKE AGENT: VDC targets different zone",
			"vdc", vdc.Name,
			"target_zone", targetZone,
			"our_zone", m.cfg.ZoneID)
		return false
	}

	// Check if we already have this VDC replicated
	targetNamespace := vdc.Annotations["ovim.io/target-namespace"]
	m.logger.Debug("🔍 SPOKE AGENT: Checking target namespace",
		"vdc", vdc.Name,
		"target_namespace", targetNamespace)
	if targetNamespace == "" {
		m.logger.Warn("❌ SPOKE AGENT: VDC missing target namespace annotation", "vdc", vdc.Name)
		return false
	}

	// Check if namespace already exists (already replicated)
	var ns corev1.Namespace
	err := m.k8sClient.Get(context.Background(), client.ObjectKey{Name: targetNamespace}, &ns)
	if err == nil {
		// Namespace exists, check if it's managed by us
		m.logger.Debug("🔍 SPOKE AGENT: Target namespace exists, checking if managed by us",
			"vdc", vdc.Name,
			"namespace", targetNamespace,
			"managed_by", ns.Labels["app.kubernetes.io/managed-by"])
		if ns.Labels["app.kubernetes.io/managed-by"] == "ovim-spoke-agent" {
			m.logger.Debug("❌ SPOKE AGENT: VDC already replicated",
				"vdc", vdc.Name,
				"namespace", targetNamespace)
			return false
		}
	} else {
		m.logger.Debug("🔍 SPOKE AGENT: Target namespace does not exist yet",
			"vdc", vdc.Name,
			"namespace", targetNamespace,
			"error", err)
	}

	m.logger.Info("✅ SPOKE AGENT: VDC should be replicated",
		"vdc", vdc.Name,
		"target_zone", targetZone,
		"target_namespace", targetNamespace)
	return true
}

// createSpokeVDC creates a spoke VDC record in the spoke cluster
func (m *Manager) createSpokeVDC(ctx context.Context, hubVDC *ovimv1.VirtualDataCenter) error {
	targetNamespace := hubVDC.Annotations["ovim.io/target-namespace"]

	// Create a spoke VDC resource that references the hub VDC
	spokeVDC := &ovimv1.VirtualDataCenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hubVDC.Name + "-spoke",
			Namespace: targetNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "ovim",
				"app.kubernetes.io/component":  "spoke-vdc",
				"app.kubernetes.io/managed-by": "ovim-spoke-agent",
				"ovim.io/organization":         hubVDC.Spec.OrganizationRef,
				"ovim.io/hub-vdc":              hubVDC.Name,
				"ovim.io/zone":                 m.cfg.ZoneID,
			},
			Annotations: map[string]string{
				"ovim.io/created-by":        "ovim-spoke-agent",
				"ovim.io/created-at":        time.Now().Format(time.RFC3339),
				"ovim.io/spoke-cluster":     m.cfg.ClusterID,
				"ovim.io/spoke-zone":        m.cfg.ZoneID,
				"ovim.io/hub-vdc-name":      hubVDC.Name,
				"ovim.io/hub-vdc-namespace": hubVDC.Namespace,
			},
		},
		Spec: ovimv1.VirtualDataCenterSpec{
			OrganizationRef: hubVDC.Spec.OrganizationRef,
			ZoneID:          m.cfg.ZoneID,
			DisplayName:     hubVDC.Spec.DisplayName + " (Spoke)",
			Description:     "Spoke VDC for " + hubVDC.Spec.DisplayName,
			Quota:           hubVDC.Spec.Quota,
			LimitRange:      hubVDC.Spec.LimitRange,
			NetworkPolicy:   hubVDC.Spec.NetworkPolicy,
			VDCType:         ovimv1.VDCTypeSpoke,
			HubVDCRef: &ovimv1.VDCReference{
				Name:      hubVDC.Name,
				Namespace: hubVDC.Namespace,
				ZoneID:    hubVDC.Spec.ZoneID,
				ClusterID: "hub-cluster", // TODO: Get actual hub cluster ID
			},
		},
		Status: ovimv1.VirtualDataCenterStatus{
			Namespace: targetNamespace,
			Phase:     ovimv1.VirtualDataCenterPhaseActive,
			HubConnectionStatus: &ovimv1.HubConnectionStatus{
				Connected:    true,
				LastPingTime: &metav1.Time{Time: time.Now()},
				HubEndpoint:  m.cfg.Hub.Endpoint,
			},
			ObservedGeneration: hubVDC.Generation,
		},
	}

	if err := m.k8sClient.Create(ctx, spokeVDC); err != nil {
		if errors.IsAlreadyExists(err) {
			m.logger.Debug("Spoke VDC already exists", "vdc", spokeVDC.Name)
			return nil
		}
		return fmt.Errorf("failed to create spoke VDC: %w", err)
	}

	m.logger.Info("Created spoke VDC record", "spoke_vdc", spokeVDC.Name, "namespace", targetNamespace, "hub_vdc", hubVDC.Name)
	return nil
}
