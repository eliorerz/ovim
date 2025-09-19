package api

import (
	"context"
	"fmt"

	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/eliorerz/ovim-updated/pkg/models"
	"github.com/eliorerz/ovim-updated/pkg/storage"
)

// EventRecorder wraps the Kubernetes event recorder and provides database storage
type EventRecorder struct {
	recorder  record.EventRecorder
	k8sClient client.Client
	storage   storage.Storage
}

// NewEventRecorder creates a new EventRecorder instance
func NewEventRecorder(recorder record.EventRecorder, k8sClient client.Client) *EventRecorder {
	return &EventRecorder{
		recorder:  recorder,
		k8sClient: k8sClient,
	}
}

// NewEventRecorderWithStorage creates a new EventRecorder instance with database storage
func NewEventRecorderWithStorage(recorder record.EventRecorder, k8sClient client.Client, storage storage.Storage) *EventRecorder {
	return &EventRecorder{
		recorder:  recorder,
		k8sClient: k8sClient,
		storage:   storage,
	}
}

// SetStorage sets the storage backend for event persistence
func (er *EventRecorder) SetStorage(storage storage.Storage) {
	er.storage = storage
}

// Record sends an event to Kubernetes and stores it in the database
func (er *EventRecorder) Record(object client.Object, eventType, reason, message string) {
	// Send to Kubernetes
	if er.recorder != nil && object != nil {
		er.recorder.Event(object, eventType, reason, message)
	}

	// Store in database if storage is available
	if er.storage != nil && object != nil {
		event := &models.Event{
			Name:      fmt.Sprintf("%s.%s", object.GetName(), reason),
			Type:      eventType,
			Reason:    reason,
			Message:   message,
			Component: "ovim-controller",
			Category:  er.getEventCategory(object.GetObjectKind().GroupVersionKind().Kind),
			Namespace: object.GetNamespace(),

			// Involved object
			InvolvedObjectKind:      object.GetObjectKind().GroupVersionKind().Kind,
			InvolvedObjectName:      object.GetName(),
			InvolvedObjectNamespace: object.GetNamespace(),
			InvolvedObjectUID:       string(object.GetUID()),

			// Source
			SourceComponent: "ovim-controller",
		}

		if err := er.storage.CreateEvent(event); err != nil {
			klog.V(4).Infof("Failed to store event in database: %v", err)
		}
	}
}

// recordDatabaseEvent is a helper to record events only in the database
func (er *EventRecorder) recordDatabaseEvent(event *models.Event) {
	if er.storage == nil {
		return
	}

	if err := er.storage.CreateEvent(event); err != nil {
		klog.V(4).Infof("Failed to store event in database: %v", err)
	} else {
		klog.V(4).Infof("Recorded event: %s - %s", event.Reason, event.Message)
	}
}

// getEventCategory determines the event category based on the resource kind
func (er *EventRecorder) getEventCategory(kind string) string {
	switch kind {
	case "Organization":
		return models.EventCategoryOrganization
	case "VirtualDataCenter":
		return models.EventCategoryVDC
	case "VirtualMachine":
		return models.EventCategoryVM
	default:
		return models.EventCategorySystem
	}
}

// Organization event recording methods
func (er *EventRecorder) RecordOrganizationCreated(ctx context.Context, orgID string, username string) {
	event := &models.Event{
		Name:      fmt.Sprintf("org-%s-created", orgID),
		Type:      models.EventTypeNormal,
		Reason:    "OrganizationCreated",
		Message:   fmt.Sprintf("Organization '%s' created successfully by %s", orgID, username),
		Component: "ovim-api",
		Category:  models.EventCategoryOrganization,
		OrgID:     &orgID,
		Username:  username,

		InvolvedObjectKind: "Organization",
		InvolvedObjectName: orgID,

		SourceComponent: "ovim-api",
	}
	er.recordDatabaseEvent(event)
}

func (er *EventRecorder) RecordOrganizationUpdated(ctx context.Context, orgID string, username string) {
	event := &models.Event{
		Name:      fmt.Sprintf("org-%s-updated", orgID),
		Type:      models.EventTypeNormal,
		Reason:    "OrganizationUpdated",
		Message:   fmt.Sprintf("Organization '%s' configuration updated by %s", orgID, username),
		Component: "ovim-api",
		Category:  models.EventCategoryOrganization,
		OrgID:     &orgID,
		Username:  username,

		InvolvedObjectKind: "Organization",
		InvolvedObjectName: orgID,

		SourceComponent: "ovim-api",
	}
	er.recordDatabaseEvent(event)
}

func (er *EventRecorder) RecordOrganizationDeleted(ctx context.Context, orgID string, username string) {
	event := &models.Event{
		Name:      fmt.Sprintf("org-%s-deleted", orgID),
		Type:      models.EventTypeNormal,
		Reason:    "OrganizationDeleted",
		Message:   fmt.Sprintf("Organization '%s' deleted by %s", orgID, username),
		Component: "ovim-api",
		Category:  models.EventCategoryOrganization,
		OrgID:     &orgID,
		Username:  username,

		InvolvedObjectKind: "Organization",
		InvolvedObjectName: orgID,

		SourceComponent: "ovim-api",
	}
	er.recordDatabaseEvent(event)
}

func (er *EventRecorder) RecordOrganizationReconcileForced(ctx context.Context, orgID string, username string) {
	event := &models.Event{
		Name:      fmt.Sprintf("org-%s-reconcile-forced", orgID),
		Type:      models.EventTypeNormal,
		Reason:    "ReconcileForced",
		Message:   fmt.Sprintf("Manual reconciliation triggered for organization '%s' by %s", orgID, username),
		Component: "ovim-api",
		Category:  models.EventCategoryOrganization,
		OrgID:     &orgID,
		Username:  username,

		InvolvedObjectKind: "Organization",
		InvolvedObjectName: orgID,

		SourceComponent: "ovim-api",
	}
	er.recordDatabaseEvent(event)
}

// VDC event recording methods
func (er *EventRecorder) RecordVDCCreated(ctx context.Context, vdcID string, orgID string, username string) {
	event := &models.Event{
		Name:      fmt.Sprintf("vdc-%s-created", vdcID),
		Type:      models.EventTypeNormal,
		Reason:    "VDCCreated",
		Message:   fmt.Sprintf("VDC '%s' created in organization '%s' by %s", vdcID, orgID, username),
		Component: "ovim-api",
		Category:  models.EventCategoryVDC,
		OrgID:     &orgID,
		VDCID:     &vdcID,
		Username:  username,

		InvolvedObjectKind: "VirtualDataCenter",
		InvolvedObjectName: vdcID,

		SourceComponent: "ovim-api",
	}
	er.recordDatabaseEvent(event)
}

func (er *EventRecorder) RecordVDCUpdated(ctx context.Context, vdcID string, orgID string, username string) {
	event := &models.Event{
		Name:      fmt.Sprintf("vdc-%s-updated", vdcID),
		Type:      models.EventTypeNormal,
		Reason:    "VDCUpdated",
		Message:   fmt.Sprintf("VDC '%s' in organization '%s' updated by %s", vdcID, orgID, username),
		Component: "ovim-api",
		Category:  models.EventCategoryVDC,
		OrgID:     &orgID,
		VDCID:     &vdcID,
		Username:  username,

		InvolvedObjectKind: "VirtualDataCenter",
		InvolvedObjectName: vdcID,

		SourceComponent: "ovim-api",
	}
	er.recordDatabaseEvent(event)
}

func (er *EventRecorder) RecordVDCDeleted(ctx context.Context, vdcID string, orgID string, username string) {
	event := &models.Event{
		Name:      fmt.Sprintf("vdc-%s-deleted", vdcID),
		Type:      models.EventTypeNormal,
		Reason:    "VDCDeleted",
		Message:   fmt.Sprintf("VDC '%s' in organization '%s' deleted by %s", vdcID, orgID, username),
		Component: "ovim-api",
		Category:  models.EventCategoryVDC,
		OrgID:     &orgID,
		VDCID:     &vdcID,
		Username:  username,

		InvolvedObjectKind: "VirtualDataCenter",
		InvolvedObjectName: vdcID,

		SourceComponent: "ovim-api",
	}
	er.recordDatabaseEvent(event)
}

// VM event recording methods
func (er *EventRecorder) RecordVMCreated(ctx context.Context, vmID string, vdcID string, orgID string, username string) {
	event := &models.Event{
		Name:      fmt.Sprintf("vm-%s-created", vmID),
		Type:      models.EventTypeNormal,
		Reason:    "VMCreated",
		Message:   fmt.Sprintf("VM '%s' created in VDC '%s' by %s", vmID, vdcID, username),
		Component: "ovim-api",
		Category:  models.EventCategoryVM,
		OrgID:     &orgID,
		VDCID:     &vdcID,
		VMID:      &vmID,
		Username:  username,

		InvolvedObjectKind: "VirtualMachine",
		InvolvedObjectName: vmID,

		SourceComponent: "ovim-api",
	}
	er.recordDatabaseEvent(event)
}

func (er *EventRecorder) RecordVMPowerStateChanged(ctx context.Context, vmID string, vdcID string, orgID string, username string, action string) {
	event := &models.Event{
		Name:      fmt.Sprintf("vm-%s-%s", vmID, action),
		Type:      models.EventTypeNormal,
		Reason:    fmt.Sprintf("VM%s", action),
		Message:   fmt.Sprintf("VM '%s' %s by %s", vmID, action, username),
		Component: "ovim-api",
		Category:  models.EventCategoryVM,
		OrgID:     &orgID,
		VDCID:     &vdcID,
		VMID:      &vmID,
		Username:  username,
		Action:    action,

		InvolvedObjectKind: "VirtualMachine",
		InvolvedObjectName: vmID,

		SourceComponent: "ovim-api",
	}
	er.recordDatabaseEvent(event)
}

func (er *EventRecorder) RecordVMDeleted(ctx context.Context, vmID string, vdcID string, orgID string, username string) {
	event := &models.Event{
		Name:      fmt.Sprintf("vm-%s-deleted", vmID),
		Type:      models.EventTypeNormal,
		Reason:    "VMDeleted",
		Message:   fmt.Sprintf("VM '%s' deleted from VDC '%s' by %s", vmID, vdcID, username),
		Component: "ovim-api",
		Category:  models.EventCategoryVM,
		OrgID:     &orgID,
		VDCID:     &vdcID,
		VMID:      &vmID,
		Username:  username,

		InvolvedObjectKind: "VirtualMachine",
		InvolvedObjectName: vmID,

		SourceComponent: "ovim-api",
	}
	er.recordDatabaseEvent(event)
}

// Security event recording methods
func (er *EventRecorder) RecordAuthenticationFailed(ctx context.Context, username string, ipAddress string) {
	event := &models.Event{
		Name:      fmt.Sprintf("auth-failed-%s", username),
		Type:      models.EventTypeWarning,
		Reason:    "AuthenticationFailed",
		Message:   fmt.Sprintf("Failed login attempt for user '%s' from IP %s", username, ipAddress),
		Component: "ovim-api",
		Category:  models.EventCategorySecurity,
		Username:  username,

		InvolvedObjectKind: "User",
		InvolvedObjectName: username,

		SourceComponent: "ovim-api",
		SourceHost:      ipAddress,
	}
	er.recordDatabaseEvent(event)
}

func (er *EventRecorder) RecordPermissionDenied(ctx context.Context, username string, action string, resource string) {
	event := &models.Event{
		Name:      fmt.Sprintf("permission-denied-%s-%s", username, resource),
		Type:      models.EventTypeWarning,
		Reason:    "PermissionDenied",
		Message:   fmt.Sprintf("User '%s' attempted to %s on resource '%s' without permission", username, action, resource),
		Component: "ovim-api",
		Category:  models.EventCategorySecurity,
		Username:  username,
		Action:    action,

		InvolvedObjectKind: "User",
		InvolvedObjectName: username,

		SourceComponent: "ovim-api",
	}
	er.recordDatabaseEvent(event)
}

// Quota event recording methods
func (er *EventRecorder) RecordQuotaExceeded(ctx context.Context, vdcID string, orgID string, resourceType string, requested int, available int) {
	event := &models.Event{
		Name:      fmt.Sprintf("vdc-%s-quota-exceeded-%s", vdcID, resourceType),
		Type:      models.EventTypeWarning,
		Reason:    "QuotaExceeded",
		Message:   fmt.Sprintf("VDC '%s' %s quota exceeded: requested %d, available %d", vdcID, resourceType, requested, available),
		Component: "ovim-api",
		Category:  models.EventCategoryQuota,
		OrgID:     &orgID,
		VDCID:     &vdcID,

		InvolvedObjectKind: "VirtualDataCenter",
		InvolvedObjectName: vdcID,

		SourceComponent: "ovim-api",
		Metadata: models.StringMap{
			"resource_type": resourceType,
			"requested":     fmt.Sprintf("%d", requested),
			"available":     fmt.Sprintf("%d", available),
		},
	}
	er.recordDatabaseEvent(event)
}

// Zone quota event recording method
func (er *EventRecorder) RecordQuotaEvent(orgID, zoneID, reason, message string) {
	event := &models.Event{
		Name:      fmt.Sprintf("org-%s-zone-%s-quota", orgID, zoneID),
		Type:      models.EventTypeNormal,
		Reason:    reason,
		Message:   message,
		Component: "ovim-api",
		Category:  models.EventCategoryQuota,
		OrgID:     &orgID,

		InvolvedObjectKind: "OrganizationZoneQuota",
		InvolvedObjectName: fmt.Sprintf("%s-%s", orgID, zoneID),

		SourceComponent: "ovim-api",
		Metadata: models.StringMap{
			"zone_id": zoneID,
		},
	}
	er.recordDatabaseEvent(event)
}
