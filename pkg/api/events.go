package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/eliorerz/ovim-updated/pkg/models"
	"github.com/eliorerz/ovim-updated/pkg/storage"
)

// EventsHandlers handles Kubernetes and database events API operations
type EventsHandlers struct {
	k8sClient    client.Client
	k8sClientset kubernetes.Interface
	storage      storage.Storage
}

// NewEventsHandlers creates a new events handlers instance
func NewEventsHandlers(k8sClient client.Client, k8sClientset kubernetes.Interface) *EventsHandlers {
	return &EventsHandlers{
		k8sClient:    k8sClient,
		k8sClientset: k8sClientset,
	}
}

// NewEventsHandlersWithStorage creates a new events handlers instance with database storage
func NewEventsHandlersWithStorage(k8sClient client.Client, k8sClientset kubernetes.Interface, storage storage.Storage) *EventsHandlers {
	return &EventsHandlers{
		k8sClient:    k8sClient,
		k8sClientset: k8sClientset,
		storage:      storage,
	}
}

// SetStorage sets the storage backend for event operations
func (h *EventsHandlers) SetStorage(storage storage.Storage) {
	h.storage = storage
}

// EventInfo represents event information for API responses
type EventInfo struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Namespace          string `json:"namespace"`
	Type               string `json:"type"`
	Reason             string `json:"reason"`
	Message            string `json:"message"`
	Component          string `json:"component"`
	InvolvedObjectKind string `json:"involved_object_kind"`
	InvolvedObjectName string `json:"involved_object_name"`
	FirstTimestamp     string `json:"first_timestamp"`
	LastTimestamp      string `json:"last_timestamp"`
	Count              int32  `json:"count"`
}

// EventsResponse represents paginated events response
type EventsResponse struct {
	Events     []EventInfo `json:"events"`
	TotalCount int         `json:"total_count"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
}

// RecentEventsResponse represents recent events response
type RecentEventsResponse struct {
	Events []EventInfo `json:"events"`
	Count  int         `json:"count"`
}

// GetEvents handles GET /api/v1/events - supports both database and Kubernetes events
func (h *EventsHandlers) GetEvents(c *gin.Context) {
	// Prefer database events if storage is available
	if h.storage != nil {
		h.getDatabaseEvents(c)
		return
	}

	// Fallback to Kubernetes events
	h.getKubernetesEvents(c)
}

// getDatabaseEvents handles events from database storage
func (h *EventsHandlers) getDatabaseEvents(c *gin.Context) {
	// Parse query parameters into EventFilter
	var filter models.EventFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid query parameters", "details": err.Error()})
		return
	}

	// Set defaults
	if filter.Limit == 0 {
		filter.Limit = 50
	}
	if filter.Limit > 200 {
		filter.Limit = 200
	}
	if filter.Page == 0 {
		filter.Page = 1
	}

	// Get events from database
	response, err := h.storage.ListEvents(&filter)
	if err != nil {
		klog.Errorf("Failed to list database events: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve events"})
		return
	}

	c.JSON(http.StatusOK, response)
}

// getKubernetesEvents handles events from Kubernetes API (fallback)
func (h *EventsHandlers) getKubernetesEvents(c *gin.Context) {
	if h.k8sClientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Event storage not available"})
		return
	}

	// Parse query parameters
	limitStr := c.DefaultQuery("limit", "50")
	pageStr := c.DefaultQuery("page", "1")
	eventType := c.Query("type")
	component := c.Query("component")
	namespace := c.Query("namespace")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	page, err := strconv.Atoi(pageStr)
	if err != nil || page <= 0 {
		page = 1
	}

	// List events from Kubernetes
	ctx := context.Background()
	listOptions := metav1.ListOptions{
		Limit: int64(limit),
	}

	var eventList *corev1.EventList
	if namespace != "" {
		eventList, err = h.k8sClientset.CoreV1().Events(namespace).List(ctx, listOptions)
	} else {
		eventList, err = h.k8sClientset.CoreV1().Events("").List(ctx, listOptions)
	}

	if err != nil {
		klog.Errorf("Failed to list events: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve events"})
		return
	}

	// Filter and convert events
	var events []EventInfo
	for _, event := range eventList.Items {
		// Apply filters
		if eventType != "" && !strings.EqualFold(event.Type, eventType) {
			continue
		}
		if component != "" && !strings.Contains(strings.ToLower(event.Source.Component), strings.ToLower(component)) {
			continue
		}

		eventInfo := EventInfo{
			ID:                 string(event.UID),
			Name:               event.Name,
			Namespace:          event.Namespace,
			Type:               event.Type,
			Reason:             event.Reason,
			Message:            event.Message,
			Component:          event.Source.Component,
			InvolvedObjectKind: event.InvolvedObject.Kind,
			InvolvedObjectName: event.InvolvedObject.Name,
			Count:              event.Count,
		}

		if !event.FirstTimestamp.IsZero() {
			eventInfo.FirstTimestamp = event.FirstTimestamp.Format("2006-01-02T15:04:05Z")
		}
		if !event.LastTimestamp.IsZero() {
			eventInfo.LastTimestamp = event.LastTimestamp.Format("2006-01-02T15:04:05Z")
		}

		events = append(events, eventInfo)
	}

	response := EventsResponse{
		Events:     events,
		TotalCount: len(events),
		Page:       page,
		PageSize:   limit,
	}

	c.JSON(http.StatusOK, response)
}

// GetRecentEvents handles GET /api/v1/events/recent
func (h *EventsHandlers) GetRecentEvents(c *gin.Context) {
	// Prefer database events if storage is available
	if h.storage != nil {
		h.getDatabaseRecentEvents(c)
		return
	}

	// Fallback to Kubernetes events
	h.getKubernetesRecentEvents(c)
}

// getDatabaseRecentEvents handles recent events from database storage
func (h *EventsHandlers) getDatabaseRecentEvents(c *gin.Context) {
	// Parse limit parameter
	limitStr := c.DefaultQuery("limit", "5")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 5
	}
	if limit > 50 {
		limit = 50
	}

	// Create filter for recent events
	filter := &models.EventFilter{
		Limit:     limit,
		Page:      1,
		SortBy:    "last_timestamp",
		SortOrder: "desc",
	}

	// Get events from database
	response, err := h.storage.ListEvents(filter)
	if err != nil {
		klog.Errorf("Failed to list recent database events: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve recent events"})
		return
	}

	recentResponse := RecentEventsResponse{
		Events: convertEventsToEventInfo(response.Events),
		Count:  len(response.Events),
	}

	c.JSON(http.StatusOK, recentResponse)
}

// getKubernetesRecentEvents handles recent events from Kubernetes API (fallback)
func (h *EventsHandlers) getKubernetesRecentEvents(c *gin.Context) {
	if h.k8sClientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Event storage not available"})
		return
	}

	// Parse limit parameter
	limitStr := c.DefaultQuery("limit", "5")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 5
	}
	if limit > 50 {
		limit = 50
	}

	// List recent events from all namespaces
	ctx := context.Background()
	listOptions := metav1.ListOptions{
		Limit: int64(limit * 5), // Get more events to filter and sort
	}

	eventList, err := h.k8sClientset.CoreV1().Events("").List(ctx, listOptions)
	if err != nil {
		klog.Errorf("Failed to list recent events: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve recent events"})
		return
	}

	// Convert and sort events by last timestamp
	var events []EventInfo
	for _, event := range eventList.Items {
		eventInfo := EventInfo{
			ID:                 string(event.UID),
			Name:               event.Name,
			Namespace:          event.Namespace,
			Type:               event.Type,
			Reason:             event.Reason,
			Message:            event.Message,
			Component:          event.Source.Component,
			InvolvedObjectKind: event.InvolvedObject.Kind,
			InvolvedObjectName: event.InvolvedObject.Name,
			Count:              event.Count,
		}

		if !event.FirstTimestamp.IsZero() {
			eventInfo.FirstTimestamp = event.FirstTimestamp.Format("2006-01-02T15:04:05Z")
		}
		if !event.LastTimestamp.IsZero() {
			eventInfo.LastTimestamp = event.LastTimestamp.Format("2006-01-02T15:04:05Z")
		}

		events = append(events, eventInfo)
	}

	// Limit to requested number
	if len(events) > limit {
		events = events[:limit]
	}

	response := RecentEventsResponse{
		Events: events,
		Count:  len(events),
	}

	c.JSON(http.StatusOK, response)
}

// convertEventsToEventInfo converts database events to EventInfo format
func convertEventsToEventInfo(events []models.Event) []EventInfo {
	var eventInfos []EventInfo
	for _, event := range events {
		eventInfo := EventInfo{
			ID:                 event.ID,
			Name:               event.Name,
			Namespace:          event.Namespace,
			Type:               event.Type,
			Reason:             event.Reason,
			Message:            event.Message,
			Component:          event.Component,
			InvolvedObjectKind: event.InvolvedObjectKind,
			InvolvedObjectName: event.InvolvedObjectName,
			Count:              int32(event.Count),
		}

		if !event.FirstTimestamp.IsZero() {
			eventInfo.FirstTimestamp = event.FirstTimestamp.Format("2006-01-02T15:04:05Z")
		}
		if !event.LastTimestamp.IsZero() {
			eventInfo.LastTimestamp = event.LastTimestamp.Format("2006-01-02T15:04:05Z")
		}

		eventInfos = append(eventInfos, eventInfo)
	}
	return eventInfos
}

// New database-specific endpoints

// CreateEvent handles POST /api/v1/events
func (h *EventsHandlers) CreateEvent(c *gin.Context) {
	if h.storage == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Event storage not available"})
		return
	}

	var req models.CreateEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	// Convert request to event model
	event := &models.Event{
		Name:      req.Name,
		Type:      req.Type,
		Reason:    req.Reason,
		Message:   req.Message,
		Component: req.Component,
		Category:  req.Category,
		Action:    req.Action,

		// Context
		Namespace: req.Namespace,
		Username:  req.Username,

		// Involved object
		InvolvedObjectKind:      req.InvolvedObjectKind,
		InvolvedObjectName:      req.InvolvedObjectName,
		InvolvedObjectNamespace: req.InvolvedObjectNamespace,
		InvolvedObjectUID:       req.InvolvedObjectUID,

		// Convert metadata
		Metadata:    models.StringMap(req.Metadata),
		Annotations: models.StringMap(req.Annotations),
		Labels:      models.StringMap(req.Labels),
	}

	// Set context IDs if provided
	if req.OrgID != "" {
		event.OrgID = &req.OrgID
	}
	if req.VDCID != "" {
		event.VDCID = &req.VDCID
	}
	if req.VMID != "" {
		event.VMID = &req.VMID
	}
	if req.UserID != "" {
		event.UserID = &req.UserID
	}

	// Set event time
	if req.EventTime != nil {
		event.EventTime = *req.EventTime
	}

	// Create event in database
	if err := h.storage.CreateEvent(event); err != nil {
		klog.Errorf("Failed to create event: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create event"})
		return
	}

	c.JSON(http.StatusCreated, event)
}

// CreateBulkEvents handles POST /api/v1/events/bulk
func (h *EventsHandlers) CreateBulkEvents(c *gin.Context) {
	if h.storage == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Event storage not available"})
		return
	}

	var req models.BulkCreateEventsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	// Convert requests to event models
	var events []*models.Event
	for _, eventReq := range req.Events {
		event := &models.Event{
			Name:      eventReq.Name,
			Type:      eventReq.Type,
			Reason:    eventReq.Reason,
			Message:   eventReq.Message,
			Component: eventReq.Component,
			Category:  eventReq.Category,
			Action:    eventReq.Action,

			// Context
			Namespace: eventReq.Namespace,
			Username:  eventReq.Username,

			// Involved object
			InvolvedObjectKind:      eventReq.InvolvedObjectKind,
			InvolvedObjectName:      eventReq.InvolvedObjectName,
			InvolvedObjectNamespace: eventReq.InvolvedObjectNamespace,
			InvolvedObjectUID:       eventReq.InvolvedObjectUID,

			// Convert metadata
			Metadata:    models.StringMap(eventReq.Metadata),
			Annotations: models.StringMap(eventReq.Annotations),
			Labels:      models.StringMap(eventReq.Labels),
		}

		// Set context IDs if provided
		if eventReq.OrgID != "" {
			event.OrgID = &eventReq.OrgID
		}
		if eventReq.VDCID != "" {
			event.VDCID = &eventReq.VDCID
		}
		if eventReq.VMID != "" {
			event.VMID = &eventReq.VMID
		}
		if eventReq.UserID != "" {
			event.UserID = &eventReq.UserID
		}

		// Set event time
		if eventReq.EventTime != nil {
			event.EventTime = *eventReq.EventTime
		}

		events = append(events, event)
	}

	// Create events in database
	if err := h.storage.CreateEvents(events); err != nil {
		klog.Errorf("Failed to create bulk events: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create events"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Events created successfully",
		"count":   len(events),
	})
}

// GetEvent handles GET /api/v1/events/:id
func (h *EventsHandlers) GetEvent(c *gin.Context) {
	if h.storage == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Event storage not available"})
		return
	}

	eventID := c.Param("id")
	if eventID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Event ID is required"})
		return
	}

	event, err := h.storage.GetEvent(eventID)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Event not found"})
			return
		}
		klog.Errorf("Failed to get event: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve event"})
		return
	}

	c.JSON(http.StatusOK, event)
}

// DeleteEvent handles DELETE /api/v1/events/:id
func (h *EventsHandlers) DeleteEvent(c *gin.Context) {
	if h.storage == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Event storage not available"})
		return
	}

	eventID := c.Param("id")
	if eventID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Event ID is required"})
		return
	}

	if err := h.storage.DeleteEvent(eventID); err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Event not found"})
			return
		}
		klog.Errorf("Failed to delete event: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete event"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Event deleted successfully"})
}

// GetEventCategories handles GET /api/v1/events/categories
func (h *EventsHandlers) GetEventCategories(c *gin.Context) {
	if h.storage == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Event storage not available"})
		return
	}

	categories, err := h.storage.ListEventCategories()
	if err != nil {
		klog.Errorf("Failed to list event categories: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve event categories"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"categories": categories})
}

// CleanupOldEvents handles POST /api/v1/events/cleanup
func (h *EventsHandlers) CleanupOldEvents(c *gin.Context) {
	if h.storage == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Event storage not available"})
		return
	}

	deletedCount, err := h.storage.CleanupOldEvents()
	if err != nil {
		klog.Errorf("Failed to cleanup old events: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cleanup old events"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Old events cleaned up successfully",
		"deleted_count": deletedCount,
	})
}
