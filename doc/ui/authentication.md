# OVIM UI Authentication & State Management

## Overview

This document provides comprehensive coverage of authentication mechanisms, state management patterns, and security considerations within the OVIM frontend application.

## Authentication Architecture

### Authentication Context

The authentication system is built around a React Context that provides centralized authentication state management across the entire application.

#### AuthContext Interface

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

#### User Data Structure

```typescript
interface User {
  id: string;
  username: string;
  email: string;
  role: 'system_admin' | 'org_admin' | 'vdc_admin' | 'user';
  org_id?: string;
  created_at: string;
  updated_at: string;
}
```

#### Authentication Info

```typescript
interface AuthInfo {
  local_auth_enabled: boolean;
  oidc_enabled: boolean;
  oidc_issuer?: string;
  supported_methods: string[];
}
```

### Authentication Methods

#### 1. Username/Password Authentication

**Implementation**: `AuthContext.login()`

```typescript
const login = async (username: string, password: string) => {
  try {
    const response = await apiService.login(username, password);
    setUser(response.user);

    // Store user data for session restoration
    localStorage.setItem('ovim_user', JSON.stringify(response.user));
  } catch (error) {
    throw error; // Re-throw for UI error handling
  }
};
```

**Process Flow**:
1. User submits credentials via LoginPage
2. AuthContext calls API client login method
3. API client sends POST request to `/api/v1/auth/login`
4. Backend validates credentials and returns JWT token
5. Token stored in localStorage by API client
6. User data stored in localStorage for session restoration
7. AuthContext updates user state
8. Application redirects to dashboard

#### 2. OIDC Authentication

**Implementation**: `AuthContext.loginWithOIDC()`

```typescript
const loginWithOIDC = async () => {
  try {
    const { auth_url, state } = await apiService.getOIDCAuthURL();

    // Store state for validation
    localStorage.setItem('oidc_state', state);

    // Redirect to the OIDC provider
    window.location.href = auth_url;
  } catch (error) {
    throw new Error('Failed to initiate OIDC authentication');
  }
};
```

**OIDC Flow**:
1. User clicks "Login with OIDC" button
2. Frontend requests OIDC auth URL from backend
3. Backend generates auth URL with state parameter
4. User redirected to OIDC provider
5. User authenticates with OIDC provider
6. OIDC provider redirects back with authorization code
7. Frontend handles callback with code and state
8. Backend exchanges code for JWT token
9. Token and user data stored locally
10. User redirected to dashboard

#### 3. OIDC Callback Handling

```typescript
const handleOIDCCallback = async (code: string, state: string) => {
  try {
    // Verify state parameter
    const storedState = localStorage.getItem('oidc_state');
    if (storedState !== state) {
      throw new Error('Invalid state parameter');
    }

    // Clear stored state
    localStorage.removeItem('oidc_state');

    // Exchange code for tokens
    const response = await apiService.handleOIDCCallback(code, state);
    setUser(response.user);

    // Store user data for session restoration
    localStorage.setItem('ovim_user', JSON.stringify(response.user));
  } catch (error) {
    throw error;
  }
};
```

### Session Management

#### Token Storage and Validation

**Token Storage**: JWT tokens are stored in localStorage with the key `ovim_token`

**Token Validation**: On application initialization, the AuthContext:
1. Retrieves token from localStorage
2. Makes authenticated API request to verify token validity
3. If valid, restores user session
4. If invalid, clears stored data and redirects to login

```typescript
useEffect(() => {
  const initializeAuth = async () => {
    try {
      // Fetch auth info to determine available methods
      const authInfo = await apiService.getAuthInfo();
      setAuthInfo(authInfo);
    } catch (error) {
      console.warn('Failed to fetch auth info:', error);
    }

    const token = localStorage.getItem('ovim_token');
    if (token) {
      try {
        // Verify token is still valid
        await apiService.getOrganizations();

        // Restore user data
        const userData = localStorage.getItem('ovim_user');
        if (userData) {
          setUser(JSON.parse(userData));
        }
      } catch (error) {
        // Token is invalid, clear it
        localStorage.removeItem('ovim_token');
        localStorage.removeItem('ovim_user');
        apiService.setToken(null);
      }
    }
    setIsLoading(false);
  };

  initializeAuth();
}, []);
```

#### Logout Process

```typescript
const logout = async () => {
  try {
    await apiService.logout();
  } catch (error) {
    // Even if logout fails on server, clear local state
    console.warn('Logout request failed, but clearing local session:', error);
  } finally {
    setUser(null);
    localStorage.removeItem('ovim_user');
    // API client automatically clears token
  }
};
```

### Security Considerations

#### Token Security
- **Storage**: Tokens stored in localStorage (consider httpOnly cookies for enhanced security)
- **Transmission**: All API requests use HTTPS
- **Expiration**: Automatic logout on token expiry
- **Validation**: Regular token validation against backend

#### State Parameter Validation (OIDC)
- **CSRF Protection**: State parameter prevents CSRF attacks
- **State Storage**: Temporary storage in localStorage during OIDC flow
- **Validation**: State verified on callback before token exchange

#### Error Handling
- **Authentication Errors**: Proper error messages without exposing sensitive info
- **Session Expiry**: Graceful handling of expired sessions
- **Network Errors**: Retry mechanisms for transient failures

## State Management Architecture

### Global State (React Context)

#### AuthContext Provider

The AuthProvider wraps the entire application and provides authentication state:

```typescript
export const AuthProvider: React.FC<AuthProviderProps> = ({ children }) => {
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [authInfo, setAuthInfo] = useState<AuthInfo | null>(null);

  // ... authentication methods

  const value = {
    user,
    isAuthenticated: !!user,
    isLoading,
    authInfo,
    login,
    loginWithOIDC,
    handleOIDCCallback,
    logout
  };

  return (
    <AuthContext.Provider value={value}>
      {children}
    </AuthContext.Provider>
  );
};
```

#### useAuth Hook

Centralized access to authentication state:

```typescript
export const useAuth = () => {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
};
```

### Page-Level State Management

#### API Integration Patterns

Custom hooks for API data management:

```typescript
// Generic API hook pattern
const useApi = <T>(
  endpoint: string,
  dependencies: any[] = []
): {
  data: T | null;
  loading: boolean;
  error: string | null;
  refetch: () => void;
} => {
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchData = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const result = await apiService.request<T>(endpoint);
      setData(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error');
    } finally {
      setLoading(false);
    }
  }, [endpoint, ...dependencies]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  return { data, loading, error, refetch: fetchData };
};
```

#### Backend Health Monitoring

```typescript
export const useBackendHealth = () => {
  const [isHealthy, setIsHealthy] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [lastChecked, setLastChecked] = useState<string | null>(null);

  const checkHealth = useCallback(async () => {
    try {
      setIsLoading(true);
      setError(null);
      await apiService.checkHealth();
      setIsHealthy(true);
      setLastChecked(new Date().toISOString());
    } catch (err) {
      setIsHealthy(false);
      setError(err instanceof Error ? err.message : 'Backend unavailable');
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    checkHealth();

    // Set up periodic health checks
    const interval = setInterval(checkHealth, 30000); // 30 seconds

    return () => clearInterval(interval);
  }, [checkHealth]);

  return { isHealthy, isLoading, error, lastChecked, refetch: checkHealth };
};
```

### Component-Level State Management

#### Form State Management

```typescript
// Example: Organization creation form
const CreateOrganizationModal: React.FC = () => {
  const [formData, setFormData] = useState<CreateOrganizationRequest>({
    name: '',
    display_name: '',
    description: '',
    admins: [],
    is_enabled: true
  });

  const [isSubmitting, setIsSubmitting] = useState(false);
  const [validationErrors, setValidationErrors] = useState<Record<string, string>>({});

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    // Validate form
    const errors = validateForm(formData);
    if (Object.keys(errors).length > 0) {
      setValidationErrors(errors);
      return;
    }

    try {
      setIsSubmitting(true);
      await apiService.createOrganization(formData);
      onSuccess();
    } catch (error) {
      // Handle submission error
    } finally {
      setIsSubmitting(false);
    }
  };

  // ... render form
};
```

#### UI State Management

```typescript
// Example: Table selection and filtering
const VirtualMachinesPage: React.FC = () => {
  const [selectedVMs, setSelectedVMs] = useState<string[]>([]);
  const [filters, setFilters] = useState({
    status: '',
    vdcId: '',
    search: ''
  });
  const [sortBy, setSortBy] = useState<'name' | 'created_at' | 'status'>('name');
  const [sortOrder, setSortOrder] = useState<'asc' | 'desc'>('asc');

  const handleSelectVM = (vmId: string, selected: boolean) => {
    setSelectedVMs(prev =>
      selected
        ? [...prev, vmId]
        : prev.filter(id => id !== vmId)
    );
  };

  const handleBulkAction = async (action: 'start' | 'stop' | 'delete') => {
    try {
      await apiService.bulkVMOperation(selectedVMs, action);
      setSelectedVMs([]); // Clear selection
      // Refresh VM list
    } catch (error) {
      // Handle error
    }
  };

  // ... render table with selection and actions
};
```

### State Update Patterns

#### Optimistic Updates

```typescript
const updateOrganization = async (id: string, updates: Partial<Organization>) => {
  // Optimistic update
  setOrganizations(prev =>
    prev.map(org =>
      org.id === id ? { ...org, ...updates } : org
    )
  );

  try {
    const updatedOrg = await apiService.updateOrganization(id, updates);
    // Update with server response
    setOrganizations(prev =>
      prev.map(org =>
        org.id === id ? updatedOrg : org
      )
    );
  } catch (error) {
    // Revert optimistic update
    setOrganizations(prev =>
      prev.map(org =>
        org.id === id ? originalOrganization : org
      )
    );
    throw error;
  }
};
```

#### Polling for Real-time Updates

```typescript
const usePolling = <T>(
  fetchFunction: () => Promise<T>,
  interval: number,
  dependencies: any[] = []
) => {
  const [data, setData] = useState<T | null>(null);

  useEffect(() => {
    let timeoutId: NodeJS.Timeout;

    const poll = async () => {
      try {
        const result = await fetchFunction();
        setData(result);
      } catch (error) {
        console.error('Polling error:', error);
      } finally {
        timeoutId = setTimeout(poll, interval);
      }
    };

    poll(); // Initial fetch

    return () => {
      if (timeoutId) {
        clearTimeout(timeoutId);
      }
    };
  }, dependencies);

  return data;
};
```

### Error State Management

#### Global Error Boundary

```typescript
interface ErrorBoundaryState {
  hasError: boolean;
  error?: Error;
}

class ErrorBoundary extends React.Component<
  React.PropsWithChildren<{}>,
  ErrorBoundaryState
> {
  constructor(props: React.PropsWithChildren<{}>) {
    super(props);
    this.state = { hasError: false };
  }

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, errorInfo: React.ErrorInfo) {
    console.error('Error caught by boundary:', error, errorInfo);
    // Report error to monitoring service
  }

  render() {
    if (this.state.hasError) {
      return (
        <EmptyState>
          <EmptyStateIcon icon={ExclamationTriangleIcon} />
          <Title headingLevel="h2" size="lg">
            Something went wrong
          </Title>
          <EmptyStateBody>
            The application encountered an unexpected error.
          </EmptyStateBody>
          <Button onClick={() => window.location.reload()}>
            Restart Application
          </Button>
        </EmptyState>
      );
    }

    return this.props.children;
  }
}
```

#### Component-Level Error Handling

```typescript
const DataComponent: React.FC = () => {
  const { data, loading, error, refetch } = useApi<DataType>('/api/data');

  if (loading) {
    return <Spinner />;
  }

  if (error) {
    return (
      <EmptyState>
        <EmptyStateIcon icon={ExclamationCircleIcon} />
        <Title headingLevel="h3" size="md">
          Failed to load data
        </Title>
        <EmptyStateBody>
          {error}
        </EmptyStateBody>
        <Button onClick={refetch}>
          Try Again
        </Button>
      </EmptyState>
    );
  }

  return <DataDisplay data={data} />;
};
```

## Integration Patterns

### API Client Integration

The API client provides centralized HTTP communication with automatic token management:

```typescript
class ApiClient {
  private baseUrl: string;
  private token: string | null = null;

  setToken(token: string | null) {
    this.token = token;
    if (token) {
      localStorage.setItem('ovim_token', token);
    } else {
      localStorage.removeItem('ovim_token');
    }
  }

  private async request<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
    const url = `${this.baseUrl}${endpoint}`;
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...(options.headers as Record<string, string>),
    };

    if (this.token) {
      headers.Authorization = `Bearer ${this.token}`;
    }

    const response = await fetch(url, { ...options, headers });

    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(`API Error ${response.status}: ${errorText}`);
    }

    return response.json();
  }
}
```

### Role-Based UI Rendering

```typescript
const ProtectedComponent: React.FC<{ requiredRole: UserRole }> = ({
  requiredRole,
  children
}) => {
  const { user } = useAuth();

  if (!user || !hasPermission(user.role, requiredRole)) {
    return (
      <EmptyState>
        <Title headingLevel="h3" size="md">
          Access Denied
        </Title>
        <EmptyStateBody>
          You don't have permission to view this content.
        </EmptyStateBody>
      </EmptyState>
    );
  }

  return <>{children}</>;
};

// Usage
<ProtectedComponent requiredRole="system_admin">
  <SystemAdminPanel />
</ProtectedComponent>
```

This comprehensive authentication and state management documentation provides developers with the knowledge needed to understand, maintain, and extend the OVIM UI's security and data management systems.