package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestEventsHandlers_GetEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		queryParams    map[string]string
		setupK8s       func() (client.Client, *fake.Clientset)
		expectedStatus int
		expectEvents   bool
		validateResp   func(*testing.T, *EventsResponse)
	}{
		{
			name:        "successful get events with default parameters",
			queryParams: map[string]string{},
			setupK8s: func() (client.Client, *fake.Clientset) {
				scheme := runtime.NewScheme()
				corev1.AddToScheme(scheme)
				k8sClient := clientfake.NewClientBuilder().WithScheme(scheme).Build()

				k8sClientset := fake.NewSimpleClientset(
					&corev1.Event{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "event-1",
							Namespace: "default",
							UID:       "event-1-uid",
						},
						Type:    "Normal",
						Reason:  "Created",
						Message: "Pod created successfully",
						Source: corev1.EventSource{
							Component: "kubelet",
						},
						InvolvedObject: corev1.ObjectReference{
							Kind: "Pod",
							Name: "test-pod",
						},
						FirstTimestamp: metav1.Time{Time: time.Now().Add(-1 * time.Hour)},
						LastTimestamp:  metav1.Time{Time: time.Now().Add(-30 * time.Minute)},
						Count:          1,
					},
					&corev1.Event{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "event-2",
							Namespace: "kube-system",
							UID:       "event-2-uid",
						},
						Type:    "Warning",
						Reason:  "Failed",
						Message: "Pod failed to start",
						Source: corev1.EventSource{
							Component: "scheduler",
						},
						InvolvedObject: corev1.ObjectReference{
							Kind: "Pod",
							Name: "system-pod",
						},
						FirstTimestamp: metav1.Time{Time: time.Now().Add(-2 * time.Hour)},
						LastTimestamp:  metav1.Time{Time: time.Now().Add(-1 * time.Hour)},
						Count:          3,
					},
				)

				return k8sClient, k8sClientset
			},
			expectedStatus: http.StatusOK,
			expectEvents:   true,
			validateResp: func(t *testing.T, resp *EventsResponse) {
				assert.Equal(t, 2, len(resp.Events))
				assert.Equal(t, 2, resp.TotalCount)
				assert.Equal(t, 1, resp.Page)
				assert.Equal(t, 50, resp.PageSize)

				// Events should be sorted by last timestamp (newest first)
				assert.Equal(t, "event-1", resp.Events[0].Name)
				assert.Equal(t, "event-2", resp.Events[1].Name)
			},
		},
		{
			name: "get events with pagination",
			queryParams: map[string]string{
				"page":      "2",
				"page_size": "1",
			},
			setupK8s: func() (client.Client, *fake.Clientset) {
				scheme := runtime.NewScheme()
				corev1.AddToScheme(scheme)
				k8sClient := clientfake.NewClientBuilder().WithScheme(scheme).Build()

				k8sClientset := fake.NewSimpleClientset(
					&corev1.Event{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "event-1",
							Namespace: "default",
							UID:       "event-1-uid",
						},
						Type:   "Normal",
						Reason: "Created",
						Source: corev1.EventSource{
							Component: "kubelet",
						},
						FirstTimestamp: metav1.Time{Time: time.Now().Add(-1 * time.Hour)},
						LastTimestamp:  metav1.Time{Time: time.Now().Add(-30 * time.Minute)},
					},
					&corev1.Event{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "event-2",
							Namespace: "default",
							UID:       "event-2-uid",
						},
						Type:   "Warning",
						Reason: "Failed",
						Source: corev1.EventSource{
							Component: "scheduler",
						},
						FirstTimestamp: metav1.Time{Time: time.Now().Add(-2 * time.Hour)},
						LastTimestamp:  metav1.Time{Time: time.Now().Add(-1 * time.Hour)},
					},
				)

				return k8sClient, k8sClientset
			},
			expectedStatus: http.StatusOK,
			expectEvents:   true,
			validateResp: func(t *testing.T, resp *EventsResponse) {
				assert.Equal(t, 1, len(resp.Events))
				assert.Equal(t, 2, resp.TotalCount)
				assert.Equal(t, 2, resp.Page)
				assert.Equal(t, 1, resp.PageSize)
				assert.Equal(t, "event-2", resp.Events[0].Name)
			},
		},
		{
			name: "get events with namespace filter",
			queryParams: map[string]string{
				"namespace": "kube-system",
			},
			setupK8s: func() (client.Client, *fake.Clientset) {
				scheme := runtime.NewScheme()
				corev1.AddToScheme(scheme)
				k8sClient := clientfake.NewClientBuilder().WithScheme(scheme).Build()

				k8sClientset := fake.NewSimpleClientset(
					&corev1.Event{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "system-event",
							Namespace: "kube-system",
							UID:       "system-event-uid",
						},
						Type:   "Normal",
						Reason: "SystemEvent",
						Source: corev1.EventSource{
							Component: "system-controller",
						},
						FirstTimestamp: metav1.Time{Time: time.Now().Add(-1 * time.Hour)},
						LastTimestamp:  metav1.Time{Time: time.Now().Add(-30 * time.Minute)},
					},
					&corev1.Event{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "default-event",
							Namespace: "default",
							UID:       "default-event-uid",
						},
						Type:   "Normal",
						Reason: "DefaultEvent",
						Source: corev1.EventSource{
							Component: "default-controller",
						},
						FirstTimestamp: metav1.Time{Time: time.Now().Add(-1 * time.Hour)},
						LastTimestamp:  metav1.Time{Time: time.Now().Add(-30 * time.Minute)},
					},
				)

				return k8sClient, k8sClientset
			},
			expectedStatus: http.StatusOK,
			expectEvents:   true,
			validateResp: func(t *testing.T, resp *EventsResponse) {
				assert.Equal(t, 1, len(resp.Events))
				assert.Equal(t, "system-event", resp.Events[0].Name)
				assert.Equal(t, "kube-system", resp.Events[0].Namespace)
			},
		},
		{
			name: "get events with type filter",
			queryParams: map[string]string{
				"type": "Warning",
			},
			setupK8s: func() (client.Client, *fake.Clientset) {
				scheme := runtime.NewScheme()
				corev1.AddToScheme(scheme)
				k8sClient := clientfake.NewClientBuilder().WithScheme(scheme).Build()

				k8sClientset := fake.NewSimpleClientset(
					&corev1.Event{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "warning-event",
							Namespace: "default",
							UID:       "warning-event-uid",
						},
						Type:   "Warning",
						Reason: "Failed",
						Source: corev1.EventSource{
							Component: "controller",
						},
						FirstTimestamp: metav1.Time{Time: time.Now().Add(-1 * time.Hour)},
						LastTimestamp:  metav1.Time{Time: time.Now().Add(-30 * time.Minute)},
					},
					&corev1.Event{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "normal-event",
							Namespace: "default",
							UID:       "normal-event-uid",
						},
						Type:   "Normal",
						Reason: "Created",
						Source: corev1.EventSource{
							Component: "controller",
						},
						FirstTimestamp: metav1.Time{Time: time.Now().Add(-1 * time.Hour)},
						LastTimestamp:  metav1.Time{Time: time.Now().Add(-30 * time.Minute)},
					},
				)

				return k8sClient, k8sClientset
			},
			expectedStatus: http.StatusOK,
			expectEvents:   true,
			validateResp: func(t *testing.T, resp *EventsResponse) {
				assert.Equal(t, 1, len(resp.Events))
				assert.Equal(t, "warning-event", resp.Events[0].Name)
				assert.Equal(t, "Warning", resp.Events[0].Type)
			},
		},
		{
			name:        "no kubernetes clientset configured",
			queryParams: map[string]string{},
			setupK8s: func() (client.Client, *fake.Clientset) {
				scheme := runtime.NewScheme()
				corev1.AddToScheme(scheme)
				k8sClient := clientfake.NewClientBuilder().WithScheme(scheme).Build()
				return k8sClient, nil // No clientset
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectEvents:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k8sClient, k8sClientset := tt.setupK8s()
			handlers := NewEventsHandlers(k8sClient, k8sClientset)

			// Build request URL with query parameters
			url := "/events"
			if len(tt.queryParams) > 0 {
				url += "?"
				for key, value := range tt.queryParams {
					url += key + "=" + value + "&"
				}
				url = url[:len(url)-1] // Remove trailing &
			}

			req := httptest.NewRequest(http.MethodGet, url, nil)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			handlers.GetEvents(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectEvents {
				var resp EventsResponse
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				require.NoError(t, err)
				if tt.validateResp != nil {
					tt.validateResp(t, &resp)
				}
			}
		})
	}
}

func TestEventsHandlers_GetRecentEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		queryParams    map[string]string
		setupK8s       func() (client.Client, *fake.Clientset)
		expectedStatus int
		expectEvents   bool
		validateResp   func(*testing.T, []EventInfo)
	}{
		{
			name: "successful get recent events",
			queryParams: map[string]string{
				"limit": "5",
			},
			setupK8s: func() (client.Client, *fake.Clientset) {
				scheme := runtime.NewScheme()
				corev1.AddToScheme(scheme)
				k8sClient := clientfake.NewClientBuilder().WithScheme(scheme).Build()

				now := time.Now()
				k8sClientset := fake.NewSimpleClientset(
					&corev1.Event{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "recent-event",
							Namespace: "default",
							UID:       "recent-event-uid",
						},
						Type:    "Normal",
						Reason:  "Created",
						Message: "Recent event occurred",
						Source: corev1.EventSource{
							Component: "controller",
						},
						InvolvedObject: corev1.ObjectReference{
							Kind: "Pod",
							Name: "recent-pod",
						},
						FirstTimestamp: metav1.Time{Time: now.Add(-5 * time.Minute)},
						LastTimestamp:  metav1.Time{Time: now.Add(-1 * time.Minute)},
						Count:          2,
					},
					&corev1.Event{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "old-event",
							Namespace: "default",
							UID:       "old-event-uid",
						},
						Type:   "Normal",
						Reason: "OldEvent",
						Source: corev1.EventSource{
							Component: "controller",
						},
						FirstTimestamp: metav1.Time{Time: now.Add(-25 * time.Hour)}, // Too old
						LastTimestamp:  metav1.Time{Time: now.Add(-24 * time.Hour)},
					},
				)

				return k8sClient, k8sClientset
			},
			expectedStatus: http.StatusOK,
			expectEvents:   true,
			validateResp: func(t *testing.T, events []EventInfo) {
				// Only recent events (within 24 hours) should be returned
				assert.Equal(t, 1, len(events))
				assert.Equal(t, "recent-event", events[0].Name)
				assert.Equal(t, "Created", events[0].Reason)
				assert.Equal(t, int32(2), events[0].Count)
			},
		},
		{
			name:        "no recent events",
			queryParams: map[string]string{},
			setupK8s: func() (client.Client, *fake.Clientset) {
				scheme := runtime.NewScheme()
				corev1.AddToScheme(scheme)
				k8sClient := clientfake.NewClientBuilder().WithScheme(scheme).Build()

				now := time.Now()
				k8sClientset := fake.NewSimpleClientset(
					&corev1.Event{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "old-event",
							Namespace: "default",
							UID:       "old-event-uid",
						},
						Type:   "Normal",
						Reason: "OldEvent",
						Source: corev1.EventSource{
							Component: "controller",
						},
						FirstTimestamp: metav1.Time{Time: now.Add(-25 * time.Hour)}, // Too old
						LastTimestamp:  metav1.Time{Time: now.Add(-24 * time.Hour)},
					},
				)

				return k8sClient, k8sClientset
			},
			expectedStatus: http.StatusOK,
			expectEvents:   true,
			validateResp: func(t *testing.T, events []EventInfo) {
				assert.Equal(t, 0, len(events))
			},
		},
		{
			name:        "no kubernetes clientset configured",
			queryParams: map[string]string{},
			setupK8s: func() (client.Client, *fake.Clientset) {
				scheme := runtime.NewScheme()
				corev1.AddToScheme(scheme)
				k8sClient := clientfake.NewClientBuilder().WithScheme(scheme).Build()
				return k8sClient, nil // No clientset
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectEvents:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k8sClient, k8sClientset := tt.setupK8s()
			handlers := NewEventsHandlers(k8sClient, k8sClientset)

			// Build request URL with query parameters
			url := "/events/recent"
			if len(tt.queryParams) > 0 {
				url += "?"
				for key, value := range tt.queryParams {
					url += key + "=" + value + "&"
				}
				url = url[:len(url)-1] // Remove trailing &
			}

			req := httptest.NewRequest(http.MethodGet, url, nil)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			handlers.GetRecentEvents(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectEvents {
				var events []EventInfo
				err := json.Unmarshal(w.Body.Bytes(), &events)
				require.NoError(t, err)
				if tt.validateResp != nil {
					tt.validateResp(t, events)
				}
			}
		})
	}
}

func TestNewEventsHandlers(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	k8sClient := clientfake.NewClientBuilder().WithScheme(scheme).Build()
	k8sClientset := fake.NewSimpleClientset()

	handlers := NewEventsHandlers(k8sClient, k8sClientset)

	assert.NotNil(t, handlers)
	assert.Equal(t, k8sClient, handlers.k8sClient)
	assert.Equal(t, k8sClientset, handlers.k8sClientset)
}

func TestEventInfo_Structure(t *testing.T) {
	event := EventInfo{
		ID:                 "event-123",
		Name:               "test-event",
		Namespace:          "default",
		Type:               "Warning",
		Reason:             "Failed",
		Message:            "Test event message",
		Component:          "test-controller",
		InvolvedObjectKind: "Pod",
		InvolvedObjectName: "test-pod",
		FirstTimestamp:     "2023-01-01T12:00:00Z",
		LastTimestamp:      "2023-01-01T12:05:00Z",
		Count:              5,
	}

	// Test JSON serialization
	data, err := json.Marshal(event)
	require.NoError(t, err)

	var unmarshaled EventInfo
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, event.ID, unmarshaled.ID)
	assert.Equal(t, event.Name, unmarshaled.Name)
	assert.Equal(t, event.Namespace, unmarshaled.Namespace)
	assert.Equal(t, event.Type, unmarshaled.Type)
	assert.Equal(t, event.Reason, unmarshaled.Reason)
	assert.Equal(t, event.Message, unmarshaled.Message)
	assert.Equal(t, event.Component, unmarshaled.Component)
	assert.Equal(t, event.InvolvedObjectKind, unmarshaled.InvolvedObjectKind)
	assert.Equal(t, event.InvolvedObjectName, unmarshaled.InvolvedObjectName)
	assert.Equal(t, event.FirstTimestamp, unmarshaled.FirstTimestamp)
	assert.Equal(t, event.LastTimestamp, unmarshaled.LastTimestamp)
	assert.Equal(t, event.Count, unmarshaled.Count)
}

func TestEventsResponse_Structure(t *testing.T) {
	response := EventsResponse{
		Events: []EventInfo{
			{
				ID:        "event-1",
				Name:      "test-event-1",
				Namespace: "default",
				Type:      "Normal",
			},
			{
				ID:        "event-2",
				Name:      "test-event-2",
				Namespace: "kube-system",
				Type:      "Warning",
			},
		},
		TotalCount: 2,
		Page:       1,
		PageSize:   50,
	}

	// Test JSON serialization
	data, err := json.Marshal(response)
	require.NoError(t, err)

	var unmarshaled EventsResponse
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, response.TotalCount, unmarshaled.TotalCount)
	assert.Equal(t, response.Page, unmarshaled.Page)
	assert.Equal(t, response.PageSize, unmarshaled.PageSize)
	assert.Len(t, unmarshaled.Events, 2)
	assert.Equal(t, "event-1", unmarshaled.Events[0].ID)
	assert.Equal(t, "event-2", unmarshaled.Events[1].ID)
}
