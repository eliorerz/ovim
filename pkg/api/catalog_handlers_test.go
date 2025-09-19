package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/eliorerz/ovim-updated/pkg/models"
)

// MockCatalogService implements catalog.Provider interface for testing
type MockCatalogService struct {
	mock.Mock
}

func (m *MockCatalogService) GetTemplates(ctx context.Context, userOrgID string, source string, category string) ([]*models.Template, error) {
	args := m.Called(ctx, userOrgID, source, category)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Template), args.Error(1)
}

func (m *MockCatalogService) GetCatalogSources(ctx context.Context, userOrgID string) ([]models.CatalogSource, error) {
	args := m.Called(ctx, userOrgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.CatalogSource), args.Error(1)
}

func TestCatalogHandlers_ListTemplates(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name            string
		queryParams     map[string]string
		userContext     map[string]interface{}
		setupMocks      func(*MockStorage, *MockCatalogService)
		expectedStatus  int
		expectTemplates bool
	}{
		{
			name: "successful list as system admin",
			queryParams: map[string]string{
				"source": "global",
			},
			userContext: map[string]interface{}{
				"user_id":  "admin1",
				"username": "admin",
				"role":     models.RoleSystemAdmin,
				"org_id":   "",
			},
			setupMocks: func(mockStorage *MockStorage, mockCatalogService *MockCatalogService) {
				templates := []*models.Template{
					{
						ID:          "tpl1",
						Name:        "Ubuntu 20.04",
						Description: "Ubuntu Server 20.04 LTS",
						Category:    "Operating System",
						Source:      "global",
					},
					{
						ID:          "tpl2",
						Name:        "CentOS 8",
						Description: "CentOS 8 Server",
						Category:    "Operating System",
						Source:      "global",
					},
				}
				mockStorage.On("ListTemplates").Return(templates, nil)
			},
			expectedStatus:  http.StatusOK,
			expectTemplates: true,
		},
		{
			name: "successful list as org admin with organization filter",
			queryParams: map[string]string{
				"source": "organization",
			},
			userContext: map[string]interface{}{
				"user_id":  "orgadmin1",
				"username": "orgadmin",
				"role":     models.RoleOrgAdmin,
				"org_id":   "org1",
			},
			setupMocks: func(mockStorage *MockStorage, mockCatalogService *MockCatalogService) {
				templates := []*models.Template{
					{
						ID:          "tpl3",
						Name:        "Custom App Template",
						Description: "Organization-specific template",
						Category:    "Application",
						Source:      "organization",
						OrgID:       "org1",
					},
				}
				mockCatalogService.On("GetTemplates", mock.AnythingOfType("*context.valueCtx"), "org1", "organization", "").Return(templates, nil)
			},
			expectedStatus:  http.StatusOK,
			expectTemplates: true,
		},
		{
			name: "list with category filter",
			queryParams: map[string]string{
				"category": "Database",
			},
			userContext: map[string]interface{}{
				"user_id":  "user1",
				"username": "user",
				"role":     models.RoleOrgUser,
				"org_id":   "org1",
			},
			setupMocks: func(mockStorage *MockStorage, mockCatalogService *MockCatalogService) {
				templates := []*models.Template{
					{
						ID:          "tpl4",
						Name:        "PostgreSQL 13",
						Description: "PostgreSQL Database Server",
						Category:    "Database",
					},
				}
				mockCatalogService.On("GetTemplates", mock.AnythingOfType("*context.valueCtx"), "org1", "", "Database").Return(templates, nil)
			},
			expectedStatus:  http.StatusOK,
			expectTemplates: true,
		},
		{
			name:            "unauthorized - no user context",
			queryParams:     map[string]string{},
			userContext:     nil, // No user context
			setupMocks:      func(*MockStorage, *MockCatalogService) {},
			expectedStatus:  http.StatusUnauthorized,
			expectTemplates: false,
		},
		{
			name:        "forbidden - invalid role",
			queryParams: map[string]string{},
			userContext: map[string]interface{}{
				"user_id":  "user1",
				"username": "user",
				"role":     "invalid_role",
				"org_id":   "org1",
			},
			setupMocks:      func(*MockStorage, *MockCatalogService) {},
			expectedStatus:  http.StatusForbidden,
			expectTemplates: false,
		},
		{
			name:        "storage error",
			queryParams: map[string]string{},
			userContext: map[string]interface{}{
				"user_id":  "admin1",
				"username": "admin",
				"role":     models.RoleSystemAdmin,
				"org_id":   "",
			},
			setupMocks: func(mockStorage *MockStorage, mockCatalogService *MockCatalogService) {
				mockStorage.On("ListTemplates").Return(nil, assert.AnError)
			},
			expectedStatus:  http.StatusInternalServerError,
			expectTemplates: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := new(MockStorage)
			mockCatalogService := new(MockCatalogService)
			tt.setupMocks(mockStorage, mockCatalogService)

			handlers := NewCatalogHandlers(mockStorage, mockCatalogService)

			// Build request URL with query parameters
			url := "/catalog/templates"
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

			// Set up query parameters
			for key, value := range tt.queryParams {
				c.Request.URL.RawQuery = key + "=" + value
			}

			// Set user context if provided
			if tt.userContext != nil {
				for key, value := range tt.userContext {
					c.Set(key, value)
				}
			}

			handlers.ListTemplates(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectTemplates {
				var templates []*models.Template
				err := json.Unmarshal(w.Body.Bytes(), &templates)
				require.NoError(t, err)
				assert.Greater(t, len(templates), 0)
			}

			mockStorage.AssertExpectations(t)
			mockCatalogService.AssertExpectations(t)
		})
	}
}

func TestCatalogHandlers_GetTemplate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		templateID     string
		userContext    map[string]interface{}
		setupMocks     func(*MockStorage, *MockCatalogService)
		expectedStatus int
		expectTemplate bool
	}{
		{
			name:       "successful get as system admin",
			templateID: "tpl1",
			userContext: map[string]interface{}{
				"user_id":  "admin1",
				"username": "admin",
				"role":     models.RoleSystemAdmin,
				"org_id":   "",
			},
			setupMocks: func(mockStorage *MockStorage, mockCatalogService *MockCatalogService) {
				template := &models.Template{
					ID:          "tpl1",
					Name:        "Ubuntu 20.04",
					Description: "Ubuntu Server 20.04 LTS",
					Category:    "Operating System",
				}
				mockStorage.On("GetTemplate", "tpl1").Return(template, nil)
			},
			expectedStatus: http.StatusOK,
			expectTemplate: true,
		},
		{
			name:       "successful get as org user with catalog service",
			templateID: "tpl2",
			userContext: map[string]interface{}{
				"user_id":  "user1",
				"username": "user",
				"role":     models.RoleOrgUser,
				"org_id":   "org1",
			},
			setupMocks: func(mockStorage *MockStorage, mockCatalogService *MockCatalogService) {
				template := &models.Template{
					ID:          "tpl2",
					Name:        "Custom Template",
					Description: "Organization-specific template",
					OrgID:       "org1",
				}
				mockStorage.On("GetTemplate", "tpl2").Return(nil, ErrNotFound)
				mockCatalogService.On("GetTemplate", "tpl2", "org1").Return(template, nil)
			},
			expectedStatus: http.StatusOK,
			expectTemplate: true,
		},
		{
			name:       "template not found",
			templateID: "nonexistent",
			userContext: map[string]interface{}{
				"user_id":  "admin1",
				"username": "admin",
				"role":     models.RoleSystemAdmin,
				"org_id":   "",
			},
			setupMocks: func(mockStorage *MockStorage, mockCatalogService *MockCatalogService) {
				mockStorage.On("GetTemplate", "nonexistent").Return(nil, ErrNotFound)
				mockCatalogService.On("GetTemplate", "nonexistent", "").Return(nil, ErrNotFound)
			},
			expectedStatus: http.StatusNotFound,
			expectTemplate: false,
		},
		{
			name:           "unauthorized - no user context",
			templateID:     "tpl1",
			userContext:    nil,
			setupMocks:     func(*MockStorage, *MockCatalogService) {},
			expectedStatus: http.StatusUnauthorized,
			expectTemplate: false,
		},
		{
			name:       "forbidden - invalid role",
			templateID: "tpl1",
			userContext: map[string]interface{}{
				"user_id":  "user1",
				"username": "user",
				"role":     "invalid_role",
				"org_id":   "org1",
			},
			setupMocks:     func(*MockStorage, *MockCatalogService) {},
			expectedStatus: http.StatusForbidden,
			expectTemplate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := new(MockStorage)
			mockCatalogService := new(MockCatalogService)
			tt.setupMocks(mockStorage, mockCatalogService)

			handlers := NewCatalogHandlers(mockStorage, mockCatalogService)

			req := httptest.NewRequest(http.MethodGet, "/catalog/templates/"+tt.templateID, nil)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req
			c.Params = []gin.Param{
				{Key: "id", Value: tt.templateID},
			}

			// Set user context if provided
			if tt.userContext != nil {
				for key, value := range tt.userContext {
					c.Set(key, value)
				}
			}

			handlers.GetTemplate(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectTemplate {
				var template models.Template
				err := json.Unmarshal(w.Body.Bytes(), &template)
				require.NoError(t, err)
				assert.Equal(t, tt.templateID, template.ID)
			}

			mockStorage.AssertExpectations(t)
			mockCatalogService.AssertExpectations(t)
		})
	}
}

func TestCatalogHandlers_GetCatalogSources(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		userContext    map[string]interface{}
		setupMocks     func(*MockStorage, *MockCatalogService)
		expectedStatus int
		expectSources  bool
	}{
		{
			name: "successful get catalog sources",
			userContext: map[string]interface{}{
				"user_id":  "admin1",
				"username": "admin",
				"role":     models.RoleSystemAdmin,
				"org_id":   "",
			},
			setupMocks: func(mockStorage *MockStorage, mockCatalogService *MockCatalogService) {
				sources := []models.CatalogSource{
					{
						ID:          "source1",
						Name:        "Global Templates",
						Type:        "git",
						URL:         "https://github.com/example/templates.git",
						Enabled:     true,
						Description: "Global template repository",
					},
					{
						ID:          "source2",
						Name:        "Local Templates",
						Type:        "local",
						Path:        "/var/lib/ovim/templates",
						Enabled:     true,
						Description: "Local template storage",
					},
				}
				mockCatalogService.On("GetCatalogSources", mock.AnythingOfType("*context.valueCtx"), "").Return(sources, nil)
			},
			expectedStatus: http.StatusOK,
			expectSources:  true,
		},
		{
			name: "catalog service error",
			userContext: map[string]interface{}{
				"user_id":  "admin1",
				"username": "admin",
				"role":     models.RoleSystemAdmin,
				"org_id":   "",
			},
			setupMocks: func(mockStorage *MockStorage, mockCatalogService *MockCatalogService) {
				mockCatalogService.On("GetCatalogSources", mock.AnythingOfType("*context.valueCtx"), "").Return(nil, assert.AnError)
			},
			expectedStatus: http.StatusInternalServerError,
			expectSources:  false,
		},
		{
			name:           "unauthorized - no user context",
			userContext:    nil,
			setupMocks:     func(*MockStorage, *MockCatalogService) {},
			expectedStatus: http.StatusUnauthorized,
			expectSources:  false,
		},
		{
			name: "forbidden - invalid role",
			userContext: map[string]interface{}{
				"user_id":  "user1",
				"username": "user",
				"role":     "invalid_role",
				"org_id":   "org1",
			},
			setupMocks:     func(*MockStorage, *MockCatalogService) {},
			expectedStatus: http.StatusForbidden,
			expectSources:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := new(MockStorage)
			mockCatalogService := new(MockCatalogService)
			tt.setupMocks(mockStorage, mockCatalogService)

			handlers := NewCatalogHandlers(mockStorage, mockCatalogService)

			req := httptest.NewRequest(http.MethodGet, "/catalog/sources", nil)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			// Set user context if provided
			if tt.userContext != nil {
				for key, value := range tt.userContext {
					c.Set(key, value)
				}
			}

			handlers.GetCatalogSources(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectSources {
				var sources []models.CatalogSource
				err := json.Unmarshal(w.Body.Bytes(), &sources)
				require.NoError(t, err)
				assert.Greater(t, len(sources), 0)
			}

			mockStorage.AssertExpectations(t)
			mockCatalogService.AssertExpectations(t)
		})
	}
}

func TestNewCatalogHandlers(t *testing.T) {
	mockStorage := new(MockStorage)
	mockCatalogService := new(MockCatalogService)

	handlers := NewCatalogHandlers(mockStorage, mockCatalogService)

	assert.NotNil(t, handlers)
	assert.Equal(t, mockStorage, handlers.storage)
	assert.Equal(t, mockCatalogService, handlers.catalogService)
}

// Helper function to create error for not found cases
var ErrNotFound = assert.AnError
