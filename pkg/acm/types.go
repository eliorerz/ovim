package acm

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// ManagedCluster represents an ACM managed cluster
// This is a simplified version of the ACM ManagedCluster CRD
type ManagedCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ManagedClusterSpec   `json:"spec,omitempty"`
	Status ManagedClusterStatus `json:"status,omitempty"`
}

// ManagedClusterSpec defines the desired state of ManagedCluster
type ManagedClusterSpec struct {
	// HubAcceptsClient indicates whether hub accepts the join of Klusterlet agent on the managed cluster
	HubAcceptsClient bool `json:"hubAcceptsClient"`

	// LeaseDurationSeconds is used to coordinate the lease update time of Klusterlet agents on the managed cluster
	LeaseDurationSeconds *int32 `json:"leaseDurationSeconds,omitempty"`

	// ManagedClusterClientConfigs represents a list of the apiserver address of the managed cluster
	ManagedClusterClientConfigs []ClientConfig `json:"managedClusterClientConfigs,omitempty"`

	// Taints is a property of managed cluster that allow the cluster to be repelled when scheduling
	Taints []Taint `json:"taints,omitempty"`
}

// ClientConfig represents the apiserver address of the managed cluster
type ClientConfig struct {
	// URL is the url of apiserver endpoint of the managed cluster
	URL string `json:"url"`

	// CABundle is the ca bundle to connect to apiserver of the managed cluster
	CABundle []byte `json:"caBundle,omitempty"`
}

// Taint represents a managed cluster taint
type Taint struct {
	// Key is the taint key to be applied to managed cluster
	Key string `json:"key"`

	// Value is the taint value corresponding to the taint key
	Value string `json:"value,omitempty"`

	// Effect indicates the taint effect to match
	Effect TaintEffect `json:"effect"`

	// TimeAdded represents the time at which the taint was added
	TimeAdded *metav1.Time `json:"timeAdded,omitempty"`
}

// TaintEffect is the effect of a taint on managed clusters that do not tolerate the taint
type TaintEffect string

const (
	// TaintEffectNoSelect means managed cluster will not be selected by placement if not tolerated
	TaintEffectNoSelect TaintEffect = "NoSelect"

	// TaintEffectPreferNoSelect means the scheduler tries not to select the managed cluster, rather than prohibit
	TaintEffectPreferNoSelect TaintEffect = "PreferNoSelect"

	// TaintEffectNoSelectIfNew means managed cluster will not be selected by placement if not tolerated, but pods already exist
	TaintEffectNoSelectIfNew TaintEffect = "NoSelectIfNew"
)

// ManagedClusterStatus represents the current status of managed cluster
type ManagedClusterStatus struct {
	// Conditions contains the different condition statuses for this managed cluster
	Conditions []metav1.Condition `json:"conditions"`

	// Capacity represents the total resource capacity from all nodeStatuses on the managed cluster
	Capacity ResourceList `json:"capacity,omitempty"`

	// Allocatable represents the total allocatable resources on the managed cluster
	Allocatable ResourceList `json:"allocatable,omitempty"`

	// Version represents the kubernetes version of the managed cluster
	Version ManagedClusterVersion `json:"version,omitempty"`

	// ClusterClaims represents cluster information that a managed cluster claims
	ClusterClaims []ManagedClusterClaim `json:"clusterClaims,omitempty"`
}

// ResourceList defines a map for the quantity of different resources
type ResourceList map[string]string

// ManagedClusterVersion represents version information about the managed cluster
type ManagedClusterVersion struct {
	// Kubernetes is the kubernetes version of managed cluster
	Kubernetes string `json:"kubernetes,omitempty"`
}

// ManagedClusterClaim represents a ClusterClaim collected from a managed cluster
type ManagedClusterClaim struct {
	// Name is the name of a ClusterClaim resource on managed cluster
	Name string `json:"name,omitempty"`

	// Value is a claim-dependent string
	Value string `json:"value,omitempty"`
}

// ManagedClusterList contains a list of ManagedCluster
type ManagedClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ManagedCluster `json:"items"`
}

// ClusterInfo represents processed cluster information from ACM
type ClusterInfo struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	APIEndpoint string `json:"api_endpoint"`
	Status      string `json:"status"`
	KubeVersion string `json:"kube_version"`
	Region      string `json:"region,omitempty"`
	Provider    string `json:"provider,omitempty"`
	NodeCount   int    `json:"node_count"`

	// Resource capacity (in standard units)
	CPUCores  int `json:"cpu_cores"`
	MemoryGB  int `json:"memory_gb"`
	StorageGB int `json:"storage_gb"`

	// Metadata and labels
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	Claims      map[string]string `json:"claims"`

	// Operational status
	Available bool      `json:"available"`
	Accepted  bool      `json:"accepted"`
	LastSeen  time.Time `json:"last_seen"`
}

// SyncConfig represents configuration for ACM sync operations
type SyncConfig struct {
	// Enabled controls whether ACM sync is active
	Enabled bool `json:"enabled"`

	// Interval is the sync interval duration
	Interval time.Duration `json:"interval"`

	// HubKubeconfig is the path to ACM hub cluster kubeconfig
	HubKubeconfig string `json:"hub_kubeconfig"`

	// Namespace is the ACM namespace (usually 'open-cluster-management')
	Namespace string `json:"namespace"`

	// AutoCreateZones controls whether to automatically create zones from discovered clusters
	AutoCreateZones bool `json:"auto_create_zones"`

	// ZonePrefix is a prefix to add to zone names created from ACM clusters
	ZonePrefix string `json:"zone_prefix"`

	// DefaultQuotaPercentage is the percentage of cluster capacity to allocate as quota
	DefaultQuotaPercentage int `json:"default_quota_percentage"`

	// ExcludedClusters is a list of cluster names to exclude from sync
	ExcludedClusters []string `json:"excluded_clusters"`

	// RequiredLabels are labels that must be present on clusters to include them
	RequiredLabels map[string]string `json:"required_labels"`
}

// SyncResult represents the result of a sync operation
type SyncResult struct {
	Timestamp        time.Time `json:"timestamp"`
	Success          bool      `json:"success"`
	ClustersFound    int       `json:"clusters_found"`
	ZonesCreated     int       `json:"zones_created"`
	ZonesUpdated     int       `json:"zones_updated"`
	ZonesDeleted     int       `json:"zones_deleted"`
	ErrorMessage     string    `json:"error_message,omitempty"`
	ProcessingTimeMs int64     `json:"processing_time_ms"`
}

// ClusterConditionType represents the condition type of managed cluster
type ClusterConditionType string

const (
	// ManagedClusterConditionAvailable means the managed cluster is available
	ManagedClusterConditionAvailable ClusterConditionType = "ManagedClusterConditionAvailable"

	// ManagedClusterConditionHubAccepted means the hub has accepted the managed cluster
	ManagedClusterConditionHubAccepted ClusterConditionType = "HubClusterAccepted"

	// ManagedClusterConditionJoined means the managed cluster has joined the hub
	ManagedClusterConditionJoined ClusterConditionType = "ManagedClusterJoined"
)

// Well-known cluster claim names from ACM
const (
	ClusterClaimPlatform  = "platform.open-cluster-management.io"
	ClusterClaimRegion    = "region.open-cluster-management.io"
	ClusterClaimProvider  = "provider.open-cluster-management.io"
	ClusterClaimVersion   = "version.open-cluster-management.io"
	ClusterClaimNodeCount = "node.count.open-cluster-management.io"
	ClusterClaimCPUCores  = "cpu.cores.open-cluster-management.io"
	ClusterClaimMemoryGB  = "memory.gb.open-cluster-management.io"
	ClusterClaimStorageGB = "storage.gb.open-cluster-management.io"
)

// Common ACM label keys
const (
	LabelClusterName     = "cluster.open-cluster-management.io/clustername"
	LabelClusterProvider = "cluster.open-cluster-management.io/provider"
	LabelClusterRegion   = "cluster.open-cluster-management.io/region"
	LabelEnvironment     = "environment"
	LabelManagedBy       = "managed-by"
	LabelZoneType        = "zone.ovim.io/type"
	LabelZoneDefault     = "zone.ovim.io/default"
)

// DeepCopyInto copies all properties of this object into another object of the same type
func (in *ManagedCluster) DeepCopyInto(out *ManagedCluster) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy creates a deep copy of the ManagedCluster
func (in *ManagedCluster) DeepCopy() *ManagedCluster {
	if in == nil {
		return nil
	}
	out := new(ManagedCluster)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject creates a deep copy that implements runtime.Object interface
func (in *ManagedCluster) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto copies the ManagedClusterSpec
func (in *ManagedClusterSpec) DeepCopyInto(out *ManagedClusterSpec) {
	*out = *in
	if in.LeaseDurationSeconds != nil {
		in, out := &in.LeaseDurationSeconds, &out.LeaseDurationSeconds
		*out = new(int32)
		**out = **in
	}
	if in.ManagedClusterClientConfigs != nil {
		in, out := &in.ManagedClusterClientConfigs, &out.ManagedClusterClientConfigs
		*out = make([]ClientConfig, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Taints != nil {
		in, out := &in.Taints, &out.Taints
		*out = make([]Taint, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopyInto copies the ClientConfig
func (in *ClientConfig) DeepCopyInto(out *ClientConfig) {
	*out = *in
	if in.CABundle != nil {
		in, out := &in.CABundle, &out.CABundle
		*out = make([]byte, len(*in))
		copy(*out, *in)
	}
}

// DeepCopyInto copies the Taint
func (in *Taint) DeepCopyInto(out *Taint) {
	*out = *in
	if in.TimeAdded != nil {
		in, out := &in.TimeAdded, &out.TimeAdded
		*out = (*in).DeepCopy()
	}
}

// DeepCopyInto copies the ManagedClusterStatus
func (in *ManagedClusterStatus) DeepCopyInto(out *ManagedClusterStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Capacity != nil {
		in, out := &in.Capacity, &out.Capacity
		*out = make(ResourceList, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Allocatable != nil {
		in, out := &in.Allocatable, &out.Allocatable
		*out = make(ResourceList, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	in.Version.DeepCopyInto(&out.Version)
	if in.ClusterClaims != nil {
		in, out := &in.ClusterClaims, &out.ClusterClaims
		*out = make([]ManagedClusterClaim, len(*in))
		copy(*out, *in)
	}
}

// DeepCopyInto copies the ManagedClusterVersion
func (in *ManagedClusterVersion) DeepCopyInto(out *ManagedClusterVersion) {
	*out = *in
}

// DeepCopyInto copies the ManagedClusterList
func (in *ManagedClusterList) DeepCopyInto(out *ManagedClusterList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ManagedCluster, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy creates a deep copy of the ManagedClusterList
func (in *ManagedClusterList) DeepCopy() *ManagedClusterList {
	if in == nil {
		return nil
	}
	out := new(ManagedClusterList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject creates a deep copy that implements runtime.Object interface
func (in *ManagedClusterList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}
