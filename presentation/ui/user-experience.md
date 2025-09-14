# OVIM UI User Experience Documentation

## Overview

This document provides comprehensive guidelines for the user experience design, interaction patterns, accessibility features, and usability considerations within the OVIM frontend application.

## Design System

### PatternFly Integration

OVIM UI is built on Red Hat's PatternFly design system, ensuring consistency with enterprise applications and OpenShift ecosystem.

#### Core Design Principles
- **Consistency**: Uniform patterns across all interfaces
- **Accessibility**: WCAG 2.1 AA compliance
- **Efficiency**: Streamlined workflows for common tasks
- **Clarity**: Clear information hierarchy and visual feedback
- **Responsiveness**: Optimal experience across all device sizes

#### Color Palette
```css
/* Primary Colors */
--pf-global--primary-color--100: #06c; /* Primary blue */
--pf-global--success-color--100: #3e8635; /* Success green */
--pf-global--warning-color--100: #f0ab00; /* Warning orange */
--pf-global--danger-color--100: #c9190b; /* Danger red */

/* Status Colors */
--pf-global--info-color--100: #73bcf7; /* Info blue */
--pf-global--disabled-color--100: #d2d2d2; /* Disabled gray */

/* Background Colors */
--pf-global--BackgroundColor--100: #fff; /* Main background */
--pf-global--BackgroundColor--200: #f5f5f5; /* Secondary background */
```

#### Typography
- **Primary Font**: Red Hat Text, Red Hat Display
- **Monospace Font**: Red Hat Mono
- **Font Sizes**: 12px (small), 14px (body), 16px (large), 18px+ (headings)

## User Interface Patterns

### Navigation Patterns

#### Main Navigation Structure
```
Application Header
├── Brand/Logo (OVIM)
├── User Profile Menu
│   ├── User Info Display
│   ├── Role Badge
│   └── Logout Action
└── Global Actions

Sidebar Navigation (Role-based)
├── Dashboard
├── Organizations (System Admin only)
├── VDCs (Admin roles)
├── Virtual Machines
├── Catalog
└── Settings
```

#### Breadcrumb Navigation
```
Home > Organizations > ACME Corp > Development VDC > VM Details
```

### Layout Patterns

#### Standard Page Layout
```
┌─────────────────────────────────────────┐
│                Header                   │
├────────────┬────────────────────────────┤
│            │         Page Title         │
│  Sidebar   │    Action Toolbar          │
│            ├────────────────────────────┤
│            │                            │
│            │      Main Content          │
│            │                            │
│            │                            │
└────────────┴────────────────────────────┘
```

#### Dashboard Layout
```
┌─────────────────────────────────────────┐
│          Summary Cards Row              │
├──────────────────┬──────────────────────┤
│                  │                      │
│   Primary Chart  │    Secondary Info    │
│                  │                      │
├──────────────────┴──────────────────────┤
│              Data Table                 │
│                                         │
└─────────────────────────────────────────┘
```

### Data Display Patterns

#### Table Design
- **Sortable Headers**: Click to sort by column
- **Filterable Content**: Search and filter controls
- **Selectable Rows**: Checkbox selection for bulk operations
- **Action Menus**: Kebab menus for row-specific actions
- **Pagination**: Page size options and navigation
- **Loading States**: Skeleton loaders during data fetch

#### Card Layout
```
┌─────────────────────────────────────────┐
│ Card Header                    [Actions]│
├─────────────────────────────────────────┤
│ Key Metric: Value                       │
│ Status: [Badge]                         │
│                                         │
│ Progress Bar: ████████░░ 80%            │
├─────────────────────────────────────────┤
│ Footer Info                    [Link]   │
└─────────────────────────────────────────┘
```

#### Chart Patterns
- **Donut Charts**: Resource usage percentages
- **Bar Charts**: Comparative metrics
- **Line Charts**: Trend data over time
- **Sparklines**: Inline trend indicators

### Form Patterns

#### Form Layout
```
┌─────────────────────────────────────────┐
│ Form Title                              │
├─────────────────────────────────────────┤
│ Required Field *                        │
│ [Text Input                        ]    │
│                                         │
│ Optional Field                          │
│ [Select Dropdown          ▼]            │
│                                         │
│ Description                             │
│ [Textarea                          ]    │
│ [                                  ]    │
├─────────────────────────────────────────┤
│                    [Cancel] [Submit]    │
└─────────────────────────────────────────┘
```

#### Validation Patterns
- **Inline Validation**: Real-time field validation
- **Error States**: Red borders and error messages
- **Success States**: Green checkmarks for valid fields
- **Required Indicators**: Asterisks for required fields
- **Help Text**: Contextual guidance below fields

## Role-Based User Experience

### System Administrator Experience

#### Dashboard Overview
- **System-wide Metrics**: Total resources across all organizations
- **Organization Overview**: Quick access to all organizations
- **Alert Summary**: Critical system alerts
- **Recent Activity**: Recent changes and deployments

#### Primary Workflows
1. **Organization Management**
   - Create/edit organizations
   - Assign admin groups
   - Set resource quotas
   - Monitor usage across orgs

2. **User Management**
   - Create/edit users
   - Assign roles and organizations
   - Monitor user activity

3. **System Monitoring**
   - View system health
   - Manage alerts and thresholds
   - Analyze usage trends

### Organization Administrator Experience

#### Dashboard Overview
- **Organization Metrics**: Resource usage within organization
- **VDC Overview**: Status and health of all VDCs
- **User Activity**: Organization member activities
- **Quota Management**: Resource allocation and usage

#### Primary Workflows
1. **VDC Management**
   - Create/configure VDCs
   - Set VDC quotas and limits
   - Monitor VDC health

2. **Resource Planning**
   - Analyze usage trends
   - Plan resource allocation
   - Manage catalog sources

3. **Team Management**
   - Assign VDC administrators
   - Monitor team activities

### VDC Administrator Experience

#### Dashboard Overview
- **VDC Metrics**: Current VDC resource usage
- **VM Overview**: Status of all VMs in VDC
- **Quick Actions**: Deploy new VMs, manage existing ones
- **Resource Limits**: Current usage vs. quotas

#### Primary Workflows
1. **VM Management**
   - Deploy VMs from templates
   - Manage VM lifecycle
   - Monitor VM performance

2. **Resource Monitoring**
   - Track resource usage
   - Optimize resource allocation
   - Plan capacity needs

### User Experience

#### Dashboard Overview
- **Personal VMs**: VMs assigned to user
- **Quick Actions**: Start/stop personal VMs
- **Resource Usage**: Personal resource consumption
- **Available Templates**: Templates for VM deployment

#### Primary Workflows
1. **VM Operations**
   - Start/stop assigned VMs
   - Access VM consoles
   - Monitor VM status

2. **Self-Service Deployment**
   - Browse available templates
   - Deploy VMs within limits
   - Manage personal workloads

## Interaction Patterns

### Loading States

#### Page Loading
```
┌─────────────────────────────────────────┐
│         Loading Page Content            │
│                                         │
│             [Spinner]                   │
│         Please wait...                  │
│                                         │
│    Checking backend connectivity        │
└─────────────────────────────────────────┘
```

#### Table Loading
```
┌─────────────────────────────────────────┐
│ Header 1    Header 2    Header 3        │
├─────────────────────────────────────────┤
│ ████████    ████████    ████████        │
│ ███████     ████████    ███████         │
│ ████████    ███████     ████████        │
└─────────────────────────────────────────┘
```

#### Chart Loading
```
┌─────────────────────────────────────────┐
│                                         │
│             [Chart Icon]                │
│          Loading chart data             │
│                                         │
└─────────────────────────────────────────┘
```

### Error States

#### Page Error
```
┌─────────────────────────────────────────┐
│            [Error Icon]                 │
│                                         │
│      Something went wrong               │
│                                         │
│   Unable to load page content           │
│                                         │
│           [Retry Button]                │
└─────────────────────────────────────────┘
```

#### Form Validation Error
```
┌─────────────────────────────────────────┐
│ Organization Name *                     │
│ [Text Input (with red border)     ]    │
│ ⚠ Organization name is required         │
└─────────────────────────────────────────┘
```

### Success States

#### Action Success
```
┌─────────────────────────────────────────┐
│ ✓ Organization created successfully     │
└─────────────────────────────────────────┘
```

#### Form Success
```
┌─────────────────────────────────────────┐
│ Organization Name *                     │
│ [Text Input (with green border)   ] ✓  │
└─────────────────────────────────────────┘
```

### Modal Patterns

#### Confirmation Modal
```
┌─────────────────────────────────────────┐
│ Delete Virtual Machine                  │
├─────────────────────────────────────────┤
│ Are you sure you want to delete         │
│ "web-server-1"? This action cannot      │
│ be undone.                              │
│                                         │
│ All data on this VM will be lost.       │
├─────────────────────────────────────────┤
│                    [Cancel] [Delete]    │
└─────────────────────────────────────────┘
```

#### Create/Edit Modal
```
┌─────────────────────────────────────────┐
│ Create Virtual Machine           [×]    │
├─────────────────────────────────────────┤
│ VM Name *                               │
│ [Text Input                        ]    │
│                                         │
│ Template *                              │
│ [Select Template               ▼]       │
│                                         │
│ Resources                               │
│ CPU: [2] Memory: [4GB] Storage: [20GB]  │
├─────────────────────────────────────────┤
│                    [Cancel] [Create]    │
└─────────────────────────────────────────┘
```

## Accessibility Features

### Keyboard Navigation

#### Tab Order
1. Skip to main content link
2. Header navigation elements
3. Sidebar navigation items
4. Main content focus areas
5. Action buttons
6. Form elements
7. Footer elements

#### Keyboard Shortcuts
- **Tab**: Navigate forward
- **Shift+Tab**: Navigate backward
- **Enter/Space**: Activate buttons and links
- **Escape**: Close modals and dropdowns
- **Arrow Keys**: Navigate within menus and tables

### Screen Reader Support

#### ARIA Labels
```html
<!-- Navigation -->
<nav aria-label="Main navigation">
  <ul role="list">
    <li role="listitem">
      <a href="/dashboard" aria-current="page">Dashboard</a>
    </li>
  </ul>
</nav>

<!-- Tables -->
<table role="table" aria-label="Virtual Machines">
  <thead>
    <tr>
      <th scope="col">Name</th>
      <th scope="col">Status</th>
    </tr>
  </thead>
</table>

<!-- Buttons -->
<button aria-label="Delete virtual machine web-server-1">
  Delete
</button>
```

#### Live Regions
```html
<!-- Status updates -->
<div aria-live="polite" aria-atomic="true">
  VM deployment in progress...
</div>

<!-- Error messages -->
<div aria-live="assertive" role="alert">
  Failed to create virtual machine
</div>
```

### Visual Accessibility

#### Color Contrast
- **Normal Text**: Minimum 4.5:1 contrast ratio
- **Large Text**: Minimum 3:1 contrast ratio
- **Interactive Elements**: Clear focus indicators

#### Focus Management
- **Visible Focus**: Clear outline on focused elements
- **Focus Trapping**: Modal dialogs trap focus
- **Focus Restoration**: Return focus after modal close

#### Text Sizing
- **Responsive Text**: Scales with browser zoom
- **Readable Fonts**: Minimum 14px for body text
- **Line Height**: 1.5x font size for readability

## Responsive Design

### Breakpoints
```css
/* Mobile First Approach */
@media (min-width: 576px) { /* Small tablets */ }
@media (min-width: 768px) { /* Tablets */ }
@media (min-width: 992px) { /* Small desktops */ }
@media (min-width: 1200px) { /* Large desktops */ }
```

### Mobile Experience

#### Navigation Adaptation
```
Mobile (< 768px):
┌─────────────────┐
│ [☰] OVIM   [👤] │
├─────────────────┤
│                 │
│   Main Content  │
│                 │
└─────────────────┘

Collapsed sidebar accessed via hamburger menu
```

#### Table Adaptation
```
Mobile Tables:
┌─────────────────┐
│ VM Name         │
│ Status: Running │
│ CPU: 2 cores    │
│ [Actions ▼]     │
├─────────────────┤
│ Next VM...      │
└─────────────────┘

Cards replace table rows for better mobile UX
```

### Tablet Experience

#### Hybrid Layout
```
Tablet (768px - 992px):
┌────────────────────────┐
│      Header            │
├────────────────────────┤
│                        │
│    Content Area        │
│  (Full width usage)    │
│                        │
└────────────────────────┘

Sidebar collapses, navigation moves to header
```

### Desktop Experience

#### Full Layout
```
Desktop (> 992px):
┌────────────────────────┐
│         Header         │
├─────┬──────────────────┤
│ Nav │   Content Area   │
│     │                  │
│     │                  │
└─────┴──────────────────┘

Full sidebar and content area
```

## Performance Considerations

### Loading Optimization

#### Progressive Loading
1. **Critical Path**: Load authentication and navigation first
2. **Above Fold**: Priority load visible content
3. **Lazy Loading**: Load components as needed
4. **Background Loading**: Prefetch likely next actions

#### Skeleton Loading
```jsx
const SkeletonTable = () => (
  <Table>
    {Array.from({ length: 5 }, (_, i) => (
      <Tr key={i}>
        <Td><Skeleton width="80%" /></Td>
        <Td><Skeleton width="60%" /></Td>
        <Td><Skeleton width="40%" /></Td>
      </Tr>
    ))}
  </Table>
);
```

### Interaction Optimization

#### Debounced Search
```jsx
const useDebounced = (value: string, delay: number) => {
  const [debouncedValue, setDebouncedValue] = useState(value);

  useEffect(() => {
    const handler = setTimeout(() => {
      setDebouncedValue(value);
    }, delay);

    return () => clearTimeout(handler);
  }, [value, delay]);

  return debouncedValue;
};
```

#### Optimistic Updates
```jsx
const handleVMStart = async (vmId: string) => {
  // Immediate UI update
  setVMs(prev => prev.map(vm =>
    vm.id === vmId
      ? { ...vm, status: 'starting' }
      : vm
  ));

  try {
    await apiService.startVM(vmId);
  } catch (error) {
    // Revert on error
    setVMs(prev => prev.map(vm =>
      vm.id === vmId
        ? { ...vm, status: 'stopped' }
        : vm
    ));
  }
};
```

## Error Prevention and Recovery

### Input Validation

#### Client-Side Validation
```jsx
const validateOrgName = (name: string): string | null => {
  if (!name.trim()) return 'Organization name is required';
  if (name.length < 3) return 'Name must be at least 3 characters';
  if (!/^[a-zA-Z0-9-]+$/.test(name)) return 'Only letters, numbers, and hyphens allowed';
  return null;
};
```

#### Real-Time Feedback
```jsx
const [nameError, setNameError] = useState<string | null>(null);

const handleNameChange = (value: string) => {
  setName(value);
  const error = validateOrgName(value);
  setNameError(error);
};
```

### Graceful Degradation

#### Offline Handling
```jsx
const useOnlineStatus = () => {
  const [isOnline, setIsOnline] = useState(navigator.onLine);

  useEffect(() => {
    const handleOnline = () => setIsOnline(true);
    const handleOffline = () => setIsOnline(false);

    window.addEventListener('online', handleOnline);
    window.addEventListener('offline', handleOffline);

    return () => {
      window.removeEventListener('online', handleOnline);
      window.removeEventListener('offline', handleOffline);
    };
  }, []);

  return isOnline;
};
```

#### Retry Mechanisms
```jsx
const useRetry = (fn: () => Promise<void>, maxRetries = 3) => {
  const [retryCount, setRetryCount] = useState(0);

  const retry = async () => {
    try {
      await fn();
      setRetryCount(0);
    } catch (error) {
      if (retryCount < maxRetries) {
        setTimeout(() => {
          setRetryCount(prev => prev + 1);
          retry();
        }, 1000 * Math.pow(2, retryCount)); // Exponential backoff
      }
    }
  };

  return { retry, retryCount };
};
```

This comprehensive user experience documentation ensures that OVIM provides an intuitive, accessible, and efficient interface for all user roles while maintaining consistency with enterprise design standards.