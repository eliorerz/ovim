# OVIM UI Components Documentation

## Overview

This document provides comprehensive documentation for all UI components in the OVIM frontend application. Components are organized by category and include detailed information about props, usage patterns, and integration points.

## Component Categories

### Navigation Components
- [AppHeader](#appheader)
- [AppSidebar](#appsidebar)

### Page Components
- [LoadingPage](#loadingpage)
- [ErrorBoundary](#errorboundary)

### Data Visualization Components
- [ResourceAggregationChart](#resourceaggregationchart)
- [ResourceTrendChart](#resourcetrendchart)
- [CRDStatusDisplay](#crdstatusdisplay)

### Management Components
- [VMManagementPanel](#vmmanagementpanel)
- [AlertsPanel](#alertspanel)

---

## Navigation Components

### AppHeader

**File**: `src/components/AppHeader.tsx`

The main application header that provides user information, authentication controls, and global navigation elements.

#### Features
- User profile display with role information
- Logout functionality
- Responsive design for mobile and desktop
- Integration with authentication context

#### Props
```typescript
// No props - uses AuthContext internally
```

#### Usage
```tsx
import { AppHeader } from './components/AppHeader';

<Page header={<AppHeader />}>
  {/* Page content */}
</Page>
```

#### Key Functionality
- **User Display**: Shows authenticated user's username and role
- **Logout Action**: Provides secure logout with session cleanup
- **Responsive Behavior**: Adapts to different screen sizes
- **Authentication Integration**: Automatically updates based on auth state

#### Dependencies
- `@patternfly/react-core`: PageHeader, Brand, Avatar
- `AuthContext`: User information and logout functionality

---

### AppSidebar

**File**: `src/components/AppSidebar.tsx`

Dynamic navigation sidebar that adapts to user role and provides access to different application sections.

#### Features
- Role-based navigation filtering
- Active route highlighting
- Responsive collapse/expand behavior
- Icon-based navigation items

#### Props
```typescript
// No props - uses AuthContext and routing internally
```

#### Usage
```tsx
import { AppSidebar } from './components/AppSidebar';

<Page sidebar={<AppSidebar />}>
  {/* Page content */}
</Page>
```

#### Navigation Structure
```typescript
interface NavigationItem {
  id: string;
  title: string;
  path: string;
  icon: React.ComponentType;
  roles: UserRole[];  // Which roles can see this item
}
```

#### Role-Based Menu Items

**System Admin Menu**:
- Dashboard
- Organizations
- VDCs
- Virtual Machines
- Catalog
- Settings

**Organization Admin Menu**:
- Dashboard
- VDCs
- Virtual Machines
- Catalog
- Settings

**VDC Admin / User Menu**:
- Dashboard
- Virtual Machines
- Catalog
- Settings

#### Dependencies
- `@patternfly/react-core`: PageSidebar, Nav
- `@patternfly/react-icons`: Navigation icons
- `react-router-dom`: Navigation and active route detection

---

## Page Components

### LoadingPage

**File**: `src/components/LoadingPage.tsx`

Comprehensive loading and health check component that handles application initialization, backend connectivity, and error states.

#### Features
- Backend health monitoring
- Loading state management
- Error display with retry functionality
- Connection status indicators

#### Props
```typescript
interface LoadingPageProps {
  isLoading: boolean;
  error: string | null;
  lastChecked: string | null;
  onRetry: () => void;
}
```

#### Usage
```tsx
import { LoadingPage } from './components/LoadingPage';

<LoadingPage
  isLoading={healthLoading}
  error={healthError}
  lastChecked={lastHealthCheck}
  onRetry={refetchHealth}
/>
```

#### States Handled
1. **Initial Loading**: Application startup with health checks
2. **Backend Unavailable**: Connection errors with retry options
3. **Authentication Loading**: User session verification
4. **Health Check Failure**: Backend service issues

#### Visual Elements
- Spinner animation during loading
- Error messages with technical details
- Retry button for failed connections
- Last check timestamp display
- Service status indicators

#### Dependencies
- `@patternfly/react-core`: EmptyState, Spinner, Button
- `@patternfly/react-icons`: Status and error icons

---

### ErrorBoundary

**File**: `src/components/ErrorBoundary.tsx`

React error boundary component that catches JavaScript errors anywhere in the component tree and displays fallback UI.

#### Features
- Catches and displays React component errors
- Provides error details for debugging
- Offers application restart functionality
- Prevents complete application crashes

#### Props
```typescript
interface ErrorBoundaryProps {
  children: React.ReactNode;
  fallback?: React.ComponentType<{ error: Error }>;
}
```

#### Usage
```tsx
import { ErrorBoundary } from './components/ErrorBoundary';

<ErrorBoundary>
  <App />
</ErrorBoundary>
```

#### Error Handling
- **Component Errors**: Catches rendering errors in child components
- **JavaScript Errors**: Displays user-friendly error messages
- **Stack Traces**: Provides debugging information in development
- **Recovery Options**: Allows users to restart the application

#### Dependencies
- `@patternfly/react-core`: EmptyState, Button
- React error boundary lifecycle methods

---

## Data Visualization Components

### ResourceAggregationChart

**File**: `src/components/ResourceAggregationChart.tsx`

Comprehensive chart component for displaying aggregated resource usage across organizations, VDCs, and the entire system.

#### Features
- Multi-level resource aggregation (System → Organization → VDC)
- Interactive donut and bar charts
- Real-time usage percentages
- Drill-down capabilities

#### Props
```typescript
interface ResourceAggregationChartProps {
  data: SystemResourceAggregation;
  level: 'system' | 'organization' | 'vdc';
  onDrillDown?: (item: ResourceItem) => void;
  refreshInterval?: number;
}
```

#### Usage
```tsx
import { ResourceAggregationChart } from './components/ResourceAggregationChart';

<ResourceAggregationChart
  data={systemResourceData}
  level="system"
  onDrillDown={handleDrillDown}
  refreshInterval={30000}
/>
```

#### Chart Types
1. **Donut Charts**: Resource usage percentages (CPU, Memory, Storage)
2. **Bar Charts**: Comparative usage across organizations/VDCs
3. **Trend Indicators**: Usage growth/decline indicators
4. **Status Indicators**: Color-coded status based on thresholds

#### Data Structure
```typescript
interface SystemResourceAggregation {
  organizations: OrganizationAggregatedResources[];
  system_totals: ResourceTotals;
  last_updated: string;
}
```

#### Interactive Features
- **Hover Details**: Detailed tooltips with exact values
- **Click Navigation**: Drill-down to organization/VDC details
- **Legend Toggle**: Show/hide different resource types
- **Refresh Control**: Manual and automatic data updates

#### Dependencies
- `@patternfly/react-charts`: Donut, Bar charts
- `recharts`: Additional chart components
- Custom hooks for data fetching and updates

---

### ResourceTrendChart

**File**: `src/components/ResourceTrendChart.tsx`

Time-series visualization component for displaying resource usage trends over configurable time periods.

#### Features
- Multi-metric trending (CPU, Memory, Storage, Workloads)
- Configurable time periods (1h, 6h, 24h, 7d, 30d)
- Interactive legend and tooltips
- Threshold line indicators

#### Props
```typescript
interface ResourceTrendChartProps {
  data: ResourceTrendData;
  period: '1h' | '6h' | '24h' | '7d' | '30d';
  onPeriodChange: (period: string) => void;
  showThresholds?: boolean;
  height?: number;
}
```

#### Usage
```tsx
import { ResourceTrendChart } from './components/ResourceTrendChart';

<ResourceTrendChart
  data={trendData}
  period="24h"
  onPeriodChange={handlePeriodChange}
  showThresholds={true}
  height={400}
/>
```

#### Chart Features
- **Multi-Line Graphs**: Separate lines for each resource type
- **Time Period Selection**: Quick period switcher buttons
- **Zoom/Pan**: Interactive chart navigation
- **Threshold Lines**: Warning and critical threshold indicators
- **Data Points**: Hoverable points with exact values

#### Time Periods
- **1h**: 5-minute intervals, last hour
- **6h**: 15-minute intervals, last 6 hours
- **24h**: 1-hour intervals, last 24 hours
- **7d**: 6-hour intervals, last 7 days
- **30d**: 1-day intervals, last 30 days

#### Dependencies
- `recharts`: LineChart, XAxis, YAxis, Tooltip
- `@patternfly/react-core`: ToggleGroup for period selection

---

### CRDStatusDisplay

**File**: `src/components/CRDStatusDisplay.tsx`

Specialized component for displaying Kubernetes Custom Resource Definition (CRD) status information with detailed condition tracking.

#### Features
- CRD phase and condition display
- Color-coded status indicators
- Detailed condition information
- Last reconciliation tracking

#### Props
```typescript
interface CRDStatusDisplayProps {
  status: CRDStatus;
  resourceType: 'Organization' | 'VDC' | 'Catalog';
  onReconcile?: () => void;
  showDetails?: boolean;
}
```

#### Usage
```tsx
import { CRDStatusDisplay } from './components/CRDStatusDisplay';

<CRDStatusDisplay
  status={organization.status}
  resourceType="Organization"
  onReconcile={handleForceReconcile}
  showDetails={true}
/>
```

#### Status Information
```typescript
interface CRDStatus {
  phase: 'Pending' | 'Progressing' | 'Ready' | 'Failed' | 'Deleting';
  conditions: KubernetesCondition[];
  observedGeneration?: number;
  message?: string;
  reason?: string;
  lastReconciled?: string;
  retryCount?: number;
}
```

#### Visual Elements
- **Phase Badges**: Color-coded status indicators
- **Condition List**: Detailed condition information with timestamps
- **Progress Indicators**: For progressing operations
- **Error Details**: Expandable error information
- **Action Buttons**: Force reconcile, refresh options

#### Status Colors
- **Ready**: Green (success)
- **Pending**: Blue (info)
- **Progressing**: Orange (warning)
- **Failed**: Red (danger)
- **Deleting**: Gray (neutral)

#### Dependencies
- `@patternfly/react-core`: Label, ExpandableSection, Button
- `@patternfly/react-icons`: Status icons

---

## Management Components

### VMManagementPanel

**File**: `src/components/VMManagementPanel.tsx`

Comprehensive virtual machine management interface providing VM lifecycle operations, monitoring, and bulk management capabilities.

#### Features
- VM lifecycle management (start, stop, restart, delete)
- Bulk operations for multiple VMs
- Real-time status monitoring
- Resource usage visualization
- VM deployment from templates

#### Props
```typescript
interface VMManagementPanelProps {
  vdcId?: string;
  organizationId?: string;
  userRole: UserRole;
  onVMCreate?: (vm: VirtualMachine) => void;
  refreshInterval?: number;
}
```

#### Usage
```tsx
import { VMManagementPanel } from './components/VMManagementPanel';

<VMManagementPanel
  vdcId={currentVDC?.id}
  organizationId={currentOrg?.id}
  userRole={user.role}
  onVMCreate={handleVMCreated}
  refreshInterval={15000}
/>
```

#### Features by Role

**System Admin / Org Admin**:
- View all VMs in scope
- Deploy new VMs
- Bulk operations
- Advanced management options

**VDC Admin**:
- View VMs in assigned VDC
- Deploy VMs within quota
- Standard VM operations

**User**:
- View assigned VMs only
- Basic start/stop operations

#### VM Operations
- **Power Management**: Start, stop, restart VMs
- **Deployment**: Create VMs from templates
- **Bulk Actions**: Multi-VM operations
- **Monitoring**: Real-time status updates
- **Resource View**: CPU, memory, storage usage

#### Table Features
- **Sortable Columns**: Status, name, resources, created date
- **Filterable Content**: By status, VDC, organization
- **Selectable Rows**: For bulk operations
- **Action Menus**: Context-specific actions per VM

#### Dependencies
- `@patternfly/react-table`: Table, Tr, Td components
- `@patternfly/react-core`: Toolbar, Button, Modal
- VM API integration hooks

---

### AlertsPanel

**File**: `src/components/AlertsPanel.tsx`

System-wide alert management component that displays notifications, thresholds, and alert configuration options.

#### Features
- Real-time alert notifications
- Alert threshold management
- Bulk alert operations
- Severity-based filtering
- Alert acknowledgment and resolution

#### Props
```typescript
interface AlertsPanelProps {
  scope?: 'system' | 'organization' | 'vdc';
  scopeId?: string;
  userRole: UserRole;
  onAlertAction?: (action: AlertAction) => void;
}
```

#### Usage
```tsx
import { AlertsPanel } from './components/AlertsPanel';

<AlertsPanel
  scope="organization"
  scopeId={organizationId}
  userRole={user.role}
  onAlertAction={handleAlertAction}
/>
```

#### Alert Types
1. **Resource Alerts**: CPU, memory, storage threshold breaches
2. **System Alerts**: Component health and connectivity issues
3. **Security Alerts**: Authentication and authorization events
4. **Operational Alerts**: Deployment and configuration issues

#### Alert Management
- **Acknowledgment**: Mark alerts as seen
- **Resolution**: Mark alerts as resolved
- **Bulk Operations**: Multi-alert actions
- **Filtering**: By severity, type, status
- **Search**: Alert content search

#### Severity Levels
- **Critical**: Red - Immediate attention required
- **Warning**: Orange - Attention needed soon
- **Info**: Blue - Informational notices

#### Threshold Management
```typescript
interface AlertThreshold {
  id: string;
  name: string;
  resource_type: 'cpu' | 'memory' | 'storage';
  threshold_percentage: number;
  severity: 'info' | 'warning' | 'critical';
  enabled: boolean;
  scope: 'system' | 'organization' | 'vdc';
}
```

#### Dependencies
- `@patternfly/react-core`: NotificationDrawer, Badge, Button
- `@patternfly/react-icons`: Alert severity icons
- Alert API integration

---

## Component Integration Patterns

### Context Integration

Components integrate with React Context for shared state:

```tsx
// Authentication context usage
const { user, isAuthenticated, logout } = useAuth();

// API integration with custom hooks
const { data, loading, error, refetch } = useApi(apiEndpoint);
```

### Error Handling

Consistent error handling across components:

```tsx
try {
  await apiCall();
  // Success handling
} catch (error) {
  // Error state update
  setError(error.message);
}
```

### Loading States

Standardized loading patterns:

```tsx
if (loading) {
  return <LoadingSpinner />;
}

if (error) {
  return <ErrorMessage error={error} onRetry={refetch} />;
}

return <ComponentContent data={data} />;
```

### Responsive Design

Components use PatternFly responsive utilities:

```tsx
<Grid hasGutter>
  <GridItem xl={8} lg={12}>
    <MainContent />
  </GridItem>
  <GridItem xl={4} lg={12}>
    <SidePanel />
  </GridItem>
</Grid>
```

## Testing Patterns

### Component Testing

```tsx
import { render, screen, fireEvent } from '@testing-library/react';
import { ComponentName } from './ComponentName';

test('renders component correctly', () => {
  render(<ComponentName prop="value" />);
  expect(screen.getByText('Expected Text')).toBeInTheDocument();
});

test('handles user interaction', async () => {
  const mockHandler = jest.fn();
  render(<ComponentName onAction={mockHandler} />);

  fireEvent.click(screen.getByRole('button'));
  expect(mockHandler).toHaveBeenCalled();
});
```

### Integration Testing

```tsx
import { renderWithProviders } from '../utils/test-utils';

test('component integrates with context', () => {
  const mockUser = { id: '1', role: 'admin' };

  renderWithProviders(
    <ComponentName />,
    { user: mockUser }
  );

  // Test component behavior with context
});
```

This comprehensive component documentation provides developers with detailed information about each UI component, their usage patterns, and integration requirements.