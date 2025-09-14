# OVIM UI Workflow Diagrams

This document contains comprehensive diagrams showing user workflows, authentication flows, component interactions, and system processes within the OVIM frontend application.

## Application Flow Overview

```mermaid
graph TB
    subgraph "Browser Session"
        INIT[Application Initialize]
        HEALTH[Backend Health Check]
        AUTH_CHECK[Authentication Check]
    end

    subgraph "Authentication Flow"
        LOGIN[Login Page]
        JWT[JWT Validation]
        OIDC[OIDC Flow]
        TOKEN[Token Storage]
    end

    subgraph "Main Application"
        DASHBOARD[Dashboard]
        NAV[Navigation]
        PAGES[Page Components]
        API[API Calls]
    end

    subgraph "Data Management"
        CONTEXT[React Context]
        HOOKS[Custom Hooks]
        STATE[Component State]
    end

    INIT --> HEALTH
    HEALTH --> AUTH_CHECK
    AUTH_CHECK --> LOGIN
    LOGIN --> JWT
    LOGIN --> OIDC
    JWT --> TOKEN
    OIDC --> TOKEN
    TOKEN --> DASHBOARD
    DASHBOARD --> NAV
    NAV --> PAGES
    PAGES --> API
    API --> CONTEXT
    CONTEXT --> HOOKS
    HOOKS --> STATE
    STATE --> PAGES
```

## User Authentication Flows

### Username/Password Authentication

```mermaid
sequenceDiagram
    participant User
    participant LoginPage
    participant AuthContext
    participant ApiClient
    participant Backend
    participant Dashboard

    User->>LoginPage: Enter credentials
    LoginPage->>AuthContext: login(username, password)
    AuthContext->>ApiClient: POST /api/v1/auth/login
    ApiClient->>Backend: HTTP Request with credentials
    Backend->>ApiClient: JWT token + user info
    ApiClient->>AuthContext: Login response
    AuthContext->>AuthContext: Store token in localStorage
    AuthContext->>AuthContext: Update user state
    AuthContext->>Dashboard: Redirect to dashboard
    Dashboard->>User: Show dashboard content
```

### OIDC Authentication Flow

```mermaid
sequenceDiagram
    participant User
    participant LoginPage
    participant AuthContext
    participant ApiClient
    participant Backend
    participant OIDCProvider
    participant CallbackPage

    User->>LoginPage: Click "Login with OIDC"
    LoginPage->>AuthContext: loginWithOIDC()
    AuthContext->>ApiClient: GET /api/v1/auth/oidc/auth-url
    ApiClient->>Backend: Request auth URL
    Backend->>ApiClient: OIDC auth URL + state
    AuthContext->>AuthContext: Store state parameter
    AuthContext->>OIDCProvider: Redirect to OIDC provider
    User->>OIDCProvider: Authenticate
    OIDCProvider->>CallbackPage: Redirect with code
    CallbackPage->>AuthContext: handleOIDCCallback(code, state)
    AuthContext->>ApiClient: POST /api/v1/auth/oidc/callback
    ApiClient->>Backend: Exchange code for token
    Backend->>ApiClient: JWT token + user info
    ApiClient->>AuthContext: Login response
    AuthContext->>AuthContext: Store token and user
    AuthContext->>User: Redirect to dashboard
```

## Role-Based Navigation Flow

```mermaid
graph TD
    subgraph "Authentication"
        LOGIN[User Login]
        ROLE_CHECK[Determine User Role]
    end

    subgraph "System Admin Navigation"
        SA_DASH[System Dashboard]
        SA_ORGS[Organizations Management]
        SA_USERS[User Management]
        SA_VDC[All VDCs]
        SA_VM[All VMs]
        SA_CATALOG[Global Catalog]
        SA_SETTINGS[System Settings]
    end

    subgraph "Organization Admin Navigation"
        OA_DASH[Organization Dashboard]
        OA_VDC[Organization VDCs]
        OA_VM[Organization VMs]
        OA_CATALOG[Organization Catalog]
        OA_SETTINGS[Org Settings]
    end

    subgraph "VDC Admin Navigation"
        VA_DASH[VDC Dashboard]
        VA_VM[VDC VMs]
        VA_CATALOG[Available Templates]
        VA_SETTINGS[VDC Settings]
    end

    subgraph "User Navigation"
        U_DASH[Personal Dashboard]
        U_VM[Assigned VMs]
        U_CATALOG[Browse Templates]
        U_SETTINGS[Profile Settings]
    end

    LOGIN --> ROLE_CHECK

    ROLE_CHECK -->|System Admin| SA_DASH
    SA_DASH --> SA_ORGS
    SA_DASH --> SA_USERS
    SA_DASH --> SA_VDC
    SA_DASH --> SA_VM
    SA_DASH --> SA_CATALOG
    SA_DASH --> SA_SETTINGS

    ROLE_CHECK -->|Org Admin| OA_DASH
    OA_DASH --> OA_VDC
    OA_DASH --> OA_VM
    OA_DASH --> OA_CATALOG
    OA_DASH --> OA_SETTINGS

    ROLE_CHECK -->|VDC Admin| VA_DASH
    VA_DASH --> VA_VM
    VA_DASH --> VA_CATALOG
    VA_DASH --> VA_SETTINGS

    ROLE_CHECK -->|User| U_DASH
    U_DASH --> U_VM
    U_DASH --> U_CATALOG
    U_DASH --> U_SETTINGS
```

## VM Management Workflow

```mermaid
graph TB
    subgraph "VM Lifecycle Management"
        VM_LIST[VM List View]
        VM_DETAIL[VM Detail View]
        VM_CREATE[Create VM Modal]
        VM_ACTIONS[VM Actions Menu]
    end

    subgraph "VM Creation Process"
        SELECT_VDC[Select VDC]
        SELECT_TEMPLATE[Select Template]
        CONFIG_RESOURCES[Configure Resources]
        VALIDATE_QUOTA[Validate Quotas]
        DEPLOY_VM[Deploy VM]
        MONITOR_STATUS[Monitor Deployment]
    end

    subgraph "VM Operations"
        POWER_ON[Power On]
        POWER_OFF[Power Off]
        RESTART[Restart]
        DELETE[Delete VM]
        CLONE[Clone VM]
        CONSOLE[Console Access]
    end

    subgraph "Bulk Operations"
        SELECT_MULTIPLE[Select Multiple VMs]
        BULK_POWER[Bulk Power Actions]
        BULK_DELETE[Bulk Delete]
        CONFIRM_ACTION[Confirm Action]
    end

    VM_LIST --> VM_DETAIL
    VM_LIST --> VM_CREATE
    VM_LIST --> VM_ACTIONS
    VM_LIST --> SELECT_MULTIPLE

    VM_CREATE --> SELECT_VDC
    SELECT_VDC --> SELECT_TEMPLATE
    SELECT_TEMPLATE --> CONFIG_RESOURCES
    CONFIG_RESOURCES --> VALIDATE_QUOTA
    VALIDATE_QUOTA --> DEPLOY_VM
    DEPLOY_VM --> MONITOR_STATUS

    VM_ACTIONS --> POWER_ON
    VM_ACTIONS --> POWER_OFF
    VM_ACTIONS --> RESTART
    VM_ACTIONS --> DELETE
    VM_ACTIONS --> CLONE
    VM_ACTIONS --> CONSOLE

    SELECT_MULTIPLE --> BULK_POWER
    SELECT_MULTIPLE --> BULK_DELETE
    BULK_POWER --> CONFIRM_ACTION
    BULK_DELETE --> CONFIRM_ACTION
```

## Organization Management Workflow

```mermaid
graph TD
    subgraph "Organization Overview"
        ORG_LIST[Organizations List]
        ORG_CREATE[Create Organization]
        ORG_DETAIL[Organization Detail]
    end

    subgraph "Organization Configuration"
        BASIC_INFO[Basic Information]
        ADMIN_GROUPS[Admin Groups]
        RESOURCE_QUOTAS[Resource Quotas]
        CATALOG_SOURCES[Catalog Sources]
    end

    subgraph "VDC Management"
        VDC_LIST[VDC List]
        VDC_CREATE[Create VDC]
        VDC_CONFIG[VDC Configuration]
        VDC_QUOTAS[VDC Quotas]
    end

    subgraph "User Management"
        USER_LIST[User List]
        USER_ASSIGN[Assign Users]
        USER_ROLES[Manage Roles]
    end

    subgraph "Monitoring"
        RESOURCE_USAGE[Resource Usage]
        ALERTS[Alert Management]
        TRENDS[Usage Trends]
    end

    ORG_LIST --> ORG_CREATE
    ORG_LIST --> ORG_DETAIL
    ORG_CREATE --> BASIC_INFO
    BASIC_INFO --> ADMIN_GROUPS
    ADMIN_GROUPS --> RESOURCE_QUOTAS
    RESOURCE_QUOTAS --> CATALOG_SOURCES

    ORG_DETAIL --> VDC_LIST
    VDC_LIST --> VDC_CREATE
    VDC_CREATE --> VDC_CONFIG
    VDC_CONFIG --> VDC_QUOTAS

    ORG_DETAIL --> USER_LIST
    USER_LIST --> USER_ASSIGN
    USER_ASSIGN --> USER_ROLES

    ORG_DETAIL --> RESOURCE_USAGE
    RESOURCE_USAGE --> ALERTS
    ALERTS --> TRENDS
```

## Component State Management Flow

```mermaid
graph TB
    subgraph "Global State (React Context)"
        AUTH_CONTEXT[Authentication Context]
        USER_STATE[User State]
        TOKEN_MANAGEMENT[Token Management]
    end

    subgraph "Page-Level State"
        PAGE_STATE[Page Component State]
        API_CALLS[API Integration]
        LOADING_STATES[Loading States]
        ERROR_HANDLING[Error Handling]
    end

    subgraph "Component State"
        LOCAL_STATE[Local Component State]
        FORM_STATE[Form Management]
        UI_STATE[UI Interactions]
    end

    subgraph "Custom Hooks"
        USE_AUTH[useAuth Hook]
        USE_API[useApi Hook]
        USE_HEALTH[useBackendHealth Hook]
    end

    AUTH_CONTEXT --> USER_STATE
    USER_STATE --> TOKEN_MANAGEMENT
    TOKEN_MANAGEMENT --> USE_AUTH

    PAGE_STATE --> API_CALLS
    API_CALLS --> LOADING_STATES
    LOADING_STATES --> ERROR_HANDLING
    API_CALLS --> USE_API

    LOCAL_STATE --> FORM_STATE
    FORM_STATE --> UI_STATE

    USE_AUTH --> PAGE_STATE
    USE_API --> PAGE_STATE
    USE_HEALTH --> PAGE_STATE
```

## Data Flow Architecture

```mermaid
graph LR
    subgraph "User Interface"
        PAGES[Page Components]
        COMPONENTS[UI Components]
        FORMS[Form Inputs]
    end

    subgraph "State Management"
        CONTEXT[React Context]
        HOOKS[Custom Hooks]
        STATE[Component State]
    end

    subgraph "API Layer"
        API_CLIENT[API Client]
        TOKEN_MGR[Token Manager]
        ERROR_HANDLER[Error Handler]
    end

    subgraph "Backend Services"
        REST_API[OVIM REST API]
        AUTH_SERVICE[Auth Service]
        DATABASE[PostgreSQL]
    end

    FORMS --> COMPONENTS
    COMPONENTS --> PAGES
    PAGES --> HOOKS
    HOOKS --> CONTEXT
    CONTEXT --> STATE

    HOOKS --> API_CLIENT
    API_CLIENT --> TOKEN_MGR
    API_CLIENT --> ERROR_HANDLER

    API_CLIENT --> REST_API
    REST_API --> AUTH_SERVICE
    REST_API --> DATABASE

    DATABASE --> REST_API
    REST_API --> API_CLIENT
    API_CLIENT --> HOOKS
    HOOKS --> PAGES
    PAGES --> COMPONENTS
```

## Error Handling Flow

```mermaid
graph TD
    subgraph "Error Sources"
        API_ERROR[API Errors]
        NETWORK_ERROR[Network Errors]
        AUTH_ERROR[Authentication Errors]
        COMPONENT_ERROR[Component Errors]
    end

    subgraph "Error Boundaries"
        GLOBAL_BOUNDARY[Global Error Boundary]
        PAGE_BOUNDARY[Page Error Boundary]
        COMPONENT_BOUNDARY[Component Error Boundary]
    end

    subgraph "Error Handling"
        ERROR_STATE[Error State Update]
        USER_NOTIFICATION[User Notification]
        RETRY_LOGIC[Retry Logic]
        FALLBACK_UI[Fallback UI]
    end

    subgraph "Recovery Actions"
        RETRY_ACTION[Retry Action]
        LOGIN_REDIRECT[Login Redirect]
        REFRESH_TOKEN[Refresh Token]
        REPORT_ERROR[Error Reporting]
    end

    API_ERROR --> ERROR_STATE
    NETWORK_ERROR --> ERROR_STATE
    AUTH_ERROR --> ERROR_STATE
    COMPONENT_ERROR --> GLOBAL_BOUNDARY

    GLOBAL_BOUNDARY --> FALLBACK_UI
    PAGE_BOUNDARY --> FALLBACK_UI
    COMPONENT_BOUNDARY --> FALLBACK_UI

    ERROR_STATE --> USER_NOTIFICATION
    USER_NOTIFICATION --> RETRY_LOGIC
    RETRY_LOGIC --> RETRY_ACTION

    AUTH_ERROR --> LOGIN_REDIRECT
    API_ERROR --> REFRESH_TOKEN
    COMPONENT_ERROR --> REPORT_ERROR
```

## Real-time Update Flow

```mermaid
sequenceDiagram
    participant UI as UI Component
    participant Hook as Custom Hook
    participant API as API Client
    participant Backend as OVIM Backend
    participant K8s as Kubernetes

    UI->>Hook: Request data with polling
    Hook->>API: Initial data fetch
    API->>Backend: GET /api/v1/resources
    Backend->>K8s: Query resource status
    K8s->>Backend: Resource data
    Backend->>API: JSON response
    API->>Hook: Data response
    Hook->>UI: Update UI with data

    Note over Hook: Start polling interval (30s)

    loop Every 30 seconds
        Hook->>API: Fetch updated data
        API->>Backend: GET /api/v1/resources
        Backend->>K8s: Query current status
        K8s->>Backend: Updated data
        Backend->>API: JSON response
        API->>Hook: Updated response
        Hook->>UI: Re-render with new data
    end

    UI->>Hook: User navigates away
    Hook->>Hook: Cleanup polling interval
```

## Deployment and Build Flow

```mermaid
graph TD
    subgraph "Development"
        DEV_START[npm start]
        DEV_SERVER[React Dev Server]
        HOT_RELOAD[Hot Module Reload]
        PROXY[API Proxy to Backend]
    end

    subgraph "Build Process"
        BUILD[npm run build]
        WEBPACK[Webpack Bundling]
        OPTIMIZE[Code Optimization]
        STATIC_FILES[Static File Generation]
    end

    subgraph "Docker Build"
        DOCKERFILE[Multi-stage Dockerfile]
        NODE_BUILD[Node.js Build Stage]
        NGINX_SERVE[Nginx Serve Stage]
        CONTAINER_IMAGE[Container Image]
    end

    subgraph "Kubernetes Deployment"
        DEPLOYMENT[UI Deployment]
        SERVICE[UI Service]
        CONFIGMAP[Nginx ConfigMap]
        INGRESS[Ingress/Route]
    end

    DEV_START --> DEV_SERVER
    DEV_SERVER --> HOT_RELOAD
    DEV_SERVER --> PROXY

    BUILD --> WEBPACK
    WEBPACK --> OPTIMIZE
    OPTIMIZE --> STATIC_FILES

    DOCKERFILE --> NODE_BUILD
    NODE_BUILD --> STATIC_FILES
    STATIC_FILES --> NGINX_SERVE
    NGINX_SERVE --> CONTAINER_IMAGE

    CONTAINER_IMAGE --> DEPLOYMENT
    DEPLOYMENT --> SERVICE
    SERVICE --> CONFIGMAP
    CONFIGMAP --> INGRESS
```

These diagrams provide a comprehensive visual representation of the OVIM UI application's workflows, from user authentication through component interactions to deployment processes.