# OVIM Frontend UI Documentation

## Overview

The OVIM (OpenShift Virtual Infrastructure Manager) Frontend UI is a modern React-based web application that provides a comprehensive interface for managing multi-tenant virtual infrastructure. Built with TypeScript and PatternFly components, it offers a responsive and accessible experience for different user roles across organizations and Virtual Data Centers (VDCs).

## Technology Stack

### Core Framework
- **React 18.3.1** - Modern React with hooks and functional components
- **TypeScript 4.9.5** - Type-safe JavaScript for better development experience
- **React Router 6.30.1** - Client-side routing and navigation

### UI Component Library
- **PatternFly React Core 5.4.14** - Enterprise-grade design system
- **PatternFly React Icons 5.4.2** - Comprehensive icon library
- **PatternFly React Table 5.4.16** - Data table components
- **PatternFly React Charts 8.3.1** - Data visualization components

### Data Fetching & State Management
- **Axios 1.7.0** - HTTP client for API communication
- **React Context API** - Global state management for authentication
- **Custom React Hooks** - Reusable state logic and API integration

### Additional Libraries
- **Recharts 3.2.0** - Additional charting capabilities
- **React Scripts 5.1.0** - Build tooling and development server

### Development Tools
- **ESLint** - Code linting and style enforcement
- **TypeScript ESLint** - TypeScript-specific linting rules
- **Jest & React Testing Library** - Unit and integration testing

## Architecture Overview

```
┌─────────────────────────────────────────┐
│                Browser                  │
├─────────────────────────────────────────┤
│             React Router                │
├─────────────────────────────────────────┤
│          Authentication Layer           │
├─────────────────────────────────────────┤
│            Page Components              │
├─────────────────────────────────────────┤
│         Shared UI Components            │
├─────────────────────────────────────────┤
│        Custom Hooks & Services          │
├─────────────────────────────────────────┤
│             API Client                  │
├─────────────────────────────────────────┤
│      OVIM Backend REST API              │
└─────────────────────────────────────────┘
```

### Key Architectural Principles

1. **Component-Based Architecture**: Modular, reusable components following React best practices
2. **Type Safety**: Comprehensive TypeScript interfaces for all data structures
3. **Role-Based Access Control**: UI adapts based on user permissions and role
4. **Responsive Design**: Mobile-first approach with PatternFly responsive utilities
5. **Error Handling**: Comprehensive error boundaries and user feedback
6. **Performance**: Code splitting, lazy loading, and optimized re-renders

## Application Structure

### Directory Layout

```
src/
├── components/          # Reusable UI components
│   ├── AlertsPanel.tsx         # System alerts and notifications
│   ├── AppHeader.tsx           # Main application header
│   ├── AppSidebar.tsx          # Navigation sidebar
│   ├── CRDStatusDisplay.tsx    # Kubernetes CRD status component
│   ├── ErrorBoundary.tsx       # Error handling boundary
│   ├── LoadingPage.tsx         # Loading states and health checks
│   ├── ResourceAggregationChart.tsx  # Resource usage charts
│   ├── ResourceTrendChart.tsx  # Resource trend visualization
│   └── VMManagementPanel.tsx   # VM control interface
├── contexts/           # React Context providers
│   └── AuthContext.tsx         # Authentication state management
├── hooks/              # Custom React hooks
│   ├── useApi.ts              # API integration hook
│   ├── useAuth.ts             # Authentication hook
│   └── useBackendHealth.ts    # Health monitoring hook
├── pages/              # Main application pages
│   ├── CatalogPage.tsx        # Template catalog browser
│   ├── DashboardPage.tsx      # System overview dashboard
│   ├── LoginPage.tsx          # Authentication page
│   ├── OrganizationDetailPage.tsx  # Organization management
│   ├── OrganizationsPage.tsx  # Organization listing
│   ├── SettingsPage.tsx       # Application settings
│   ├── VDCDetailPage.tsx      # VDC management interface
│   ├── VDCListPage.tsx        # VDC listing (legacy)
│   ├── VDCsPage.tsx           # VDC overview
│   └── VirtualMachinesPage.tsx # VM management
├── services/           # External service integrations
│   └── api.ts                 # REST API client
├── utils/              # Utility functions
├── App.tsx             # Main application component
├── index.tsx           # Application entry point
├── index.css           # Global styles
└── setupTests.ts       # Test configuration
```

## Authentication & Authorization

### Authentication Flow

1. **Initial Load**: Check for stored JWT token in localStorage
2. **Token Validation**: Verify token validity with backend health check
3. **Auth Info Retrieval**: Fetch available authentication methods (JWT/OIDC)
4. **Login Process**: Support both username/password and OIDC flows
5. **Session Management**: Automatic token refresh and logout handling

### User Roles & Permissions

The UI adapts its interface based on four primary user roles:

#### System Admin
- **Full Access**: All organizations, users, and system settings
- **UI Features**: Complete dashboard, all management panels
- **Navigation**: All menu items visible
- **Actions**: Create/edit/delete all resources

#### Organization Admin
- **Scoped Access**: Single organization and its VDCs
- **UI Features**: Organization-specific dashboard and resources
- **Navigation**: VDC management, organization settings
- **Actions**: Manage VDCs, users within organization

#### VDC Admin
- **Limited Access**: Specific VDC and its workloads
- **UI Features**: VDC dashboard, VM management
- **Navigation**: VM operations, limited settings
- **Actions**: Deploy/manage VMs within assigned VDC

#### User
- **Restricted Access**: Assigned VMs and resources
- **UI Features**: Personal dashboard, VM status
- **Navigation**: Basic VM operations
- **Actions**: View/control assigned VMs

### Authentication Context

```typescript
interface AuthContextType {
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  authInfo: AuthInfo | null;
  login: (username: string, password: string) => Promise<void>;
  loginWithOIDC: () => Promise<void>;
  handleOIDCCallback: (code: string, state: string) => Promise<void>;
  logout: () => Promise<void>;
}
```

## API Integration

### REST API Client

The application uses a centralized API client (`services/api.ts`) that provides:

- **Type-Safe Endpoints**: TypeScript interfaces for all API operations
- **Authentication Handling**: Automatic JWT token management
- **Error Handling**: Consistent error processing and user feedback
- **Request/Response Transformation**: Backend snake_case to frontend camelCase

### Key API Categories

1. **Authentication**: Login, logout, OIDC integration
2. **Organizations**: CRUD operations, resource management
3. **VDCs**: Virtual Data Center management and monitoring
4. **Virtual Machines**: VM lifecycle and operations
5. **Templates**: Catalog browsing and template management
6. **Users**: User management (System Admin only)
7. **Dashboard**: Aggregated metrics and system health
8. **Alerts**: Notification management and thresholds

### API Client Features

- **Automatic Token Management**: Stores and injects JWT tokens
- **Health Monitoring**: Backend connectivity verification
- **Error Standardization**: Consistent error response handling
- **Request Transformation**: Automatic data format conversion

## Component Architecture

### Page Components

Page components represent complete application views and handle:
- Route-level state management
- Data fetching and error handling
- Layout and navigation context
- User permission enforcement

### Shared Components

Reusable components provide consistent UI patterns:
- **AlertsPanel**: System-wide alert management
- **ResourceCharts**: Data visualization for resource usage
- **CRDStatusDisplay**: Kubernetes resource status indicators
- **VMManagementPanel**: VM control interface

### Custom Hooks

Custom hooks encapsulate reusable logic:
- **useAuth**: Authentication state and operations
- **useApi**: API integration with loading states
- **useBackendHealth**: System health monitoring

## State Management

### Authentication State
- Managed through React Context
- Persistent across browser sessions
- Automatic token validation and refresh

### Component State
- Local state for UI interactions
- API response caching in custom hooks
- Optimistic updates for better UX

### Error State
- Global error boundary for crash protection
- Component-level error handling
- User-friendly error messages

## Navigation & Routing

### Route Structure

```
/                          → Redirect to /dashboard
/dashboard                 → System overview (all users)
/organizations             → Organization list (System Admin)
/organizations/:id         → Organization detail (System Admin)
/organizations/:orgId/vdcs/:vdcId → VDC detail (nested route)
/vdcs                      → VDC list (Admin roles)
/vdcs/:id                  → VDC detail (Admin roles)
/catalog                   → Template catalog (all users)
/virtual-machines          → VM management (all users)
/settings                  → Application settings (all users)
*                          → Redirect to /dashboard
```

### Navigation Guards

- **Authentication Required**: All routes except login
- **Role-Based Access**: Routes filtered by user permissions
- **Health Checks**: Backend connectivity verification

### Sidebar Navigation

Dynamic navigation menu that adapts to user role:
- **System Admin**: All menu items
- **Organization Admin**: Organization-scoped items
- **VDC Admin**: VDC-scoped items
- **User**: Basic navigation only

## Data Flow

### Authentication Flow
1. User accesses application
2. AuthProvider checks localStorage for token
3. Token validated against backend
4. User redirected to appropriate dashboard
5. Navigation and features filtered by role

### Resource Management Flow
1. User selects resource type (Organization/VDC/VM)
2. Component fetches data via API client
3. Loading states displayed during fetch
4. Data rendered with role-appropriate actions
5. User actions trigger API calls
6. Optimistic updates provide immediate feedback
7. Error handling for failed operations

### Real-time Updates
- Health monitoring with periodic backend checks
- Resource usage updates through polling
- Alert notifications for system events

## Deployment Architecture

### Development Environment
- **React Dev Server**: Hot reload and debugging
- **Proxy Configuration**: API requests proxied to backend
- **Environment Variables**: Configuration through `.env` files

### Production Deployment
- **Nginx Container**: Serves static files and handles routing
- **TLS Termination**: HTTPS with certificate management
- **API Proxy**: Backend API requests proxied through Nginx
- **Health Checks**: Kubernetes liveness and readiness probes

### Container Architecture

```dockerfile
# Multi-stage build
FROM node:18-alpine as builder
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production

FROM nginx:alpine
COPY --from=builder /app/build /usr/share/nginx/html
COPY nginx.conf /etc/nginx/nginx.conf
EXPOSE 8080 8443
```

### Kubernetes Deployment

- **Deployment**: 2 replicas for high availability
- **Service**: ClusterIP with HTTP/HTTPS ports
- **ConfigMap**: Nginx configuration
- **Security**: Non-root container, read-only filesystem
- **Resources**: CPU and memory limits defined

## Performance Considerations

### Bundle Optimization
- **Code Splitting**: Route-based lazy loading
- **Tree Shaking**: Unused code elimination
- **Asset Optimization**: Image and font optimization

### Runtime Performance
- **React.memo**: Component memoization for expensive renders
- **useCallback**: Function memoization for stable references
- **Debounced Inputs**: Search and filter optimization
- **Virtual Scrolling**: Large dataset handling

### Caching Strategy
- **API Response Caching**: Custom hooks with stale-while-revalidate
- **Image Caching**: Browser cache headers
- **Static Asset Caching**: Long-term caching with versioning

## Security Features

### Input Validation
- **TypeScript Interfaces**: Compile-time type checking
- **Form Validation**: Input sanitization and validation
- **XSS Protection**: Automatic escaping in React

### Authentication Security
- **JWT Token Storage**: Secure localStorage handling
- **Token Expiration**: Automatic logout on expiry
- **CSRF Protection**: SameSite cookie configuration

### Communication Security
- **HTTPS Only**: Forced HTTPS redirects
- **Content Security Policy**: XSS and injection protection
- **CORS Configuration**: Strict origin validation

## Testing Strategy

### Unit Testing
- **Component Testing**: React Testing Library
- **Hook Testing**: Custom hook verification
- **Service Testing**: API client mocking

### Integration Testing
- **Page Flow Testing**: Complete user workflows
- **API Integration**: Backend integration scenarios
- **Authentication Flow**: Login/logout processes

### E2E Testing
- **User Journey Testing**: Complete application flows
- **Cross-Browser Testing**: Compatibility verification
- **Performance Testing**: Load and response time testing

## Development Workflow

### Getting Started
```bash
# Install dependencies
npm install

# Start development server
npm start

# Run tests
npm test

# Build for production
npm run build
```

### Code Quality
```bash
# Lint TypeScript code
npm run lint

# Fix linting issues
npm run lint:fix

# Type checking
npm run type-check
```

### Environment Configuration
- **REACT_APP_API_URL**: Backend API endpoint
- **REACT_APP_ENVIRONMENT**: Development/production mode
- **NODE_ENV**: Build environment

This comprehensive UI documentation provides developers and operators with the knowledge needed to understand, maintain, and extend the OVIM frontend application effectively.