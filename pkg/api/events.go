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
)

// EventsHandlers handles Kubernetes events API operations
type EventsHandlers struct {
	k8sClient    client.Client
	k8sClientset kubernetes.Interface
}

// NewEventsHandlers creates a new events handlers instance
func NewEventsHandlers(k8sClient client.Client, k8sClientset kubernetes.Interface) *EventsHandlers {
	return &EventsHandlers{
		k8sClient:    k8sClient,
		k8sClientset: k8sClientset,
	}
}

// EventInfo represents event information for API responses
type EventInfo struct {
	ID                   string `json:"id"`
	Name                 string `json:"name"`
	Namespace            string `json:"namespace"`
	Type                 string `json:"type"`
	Reason               string `json:"reason"`
	Message              string `json:"message"`
	Component            string `json:"component"`
	InvolvedObjectKind   string `json:"involved_object_kind"`
	InvolvedObjectName   string `json:"involved_object_name"`
	FirstTimestamp       string `json:"first_timestamp"`
	LastTimestamp        string `json:"last_timestamp"`
	Count                int32  `json:"count"`
}

// EventsResponse represents paginated events response
type EventsResponse struct {
	Events    []EventInfo `json:"events"`
	TotalCount int        `json:"total_count"`
	Page       int        `json:"page"`
	PageSize   int        `json:"page_size"`
}

// RecentEventsResponse represents recent events response
type RecentEventsResponse struct {
	Events []EventInfo `json:"events"`
	Count  int         `json:"count"`
}

// GetEvents handles GET /api/v1/events
func (h *EventsHandlers) GetEvents(c *gin.Context) {
	if h.k8sClientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Kubernetes client not available"})
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
			ID:                   string(event.UID),
			Name:                 event.Name,
			Namespace:            event.Namespace,
			Type:                 event.Type,
			Reason:               event.Reason,
			Message:              event.Message,
			Component:            event.Source.Component,
			InvolvedObjectKind:   event.InvolvedObject.Kind,
			InvolvedObjectName:   event.InvolvedObject.Name,
			Count:                event.Count,
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
	if h.k8sClientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Kubernetes client not available"})
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
			ID:                   string(event.UID),
			Name:                 event.Name,
			Namespace:            event.Namespace,
			Type:                 event.Type,
			Reason:               event.Reason,
			Message:              event.Message,
			Component:            event.Source.Component,
			InvolvedObjectKind:   event.InvolvedObject.Kind,
			InvolvedObjectName:   event.InvolvedObject.Name,
			Count:                event.Count,
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