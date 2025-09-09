# OVIM Integration Tests

This directory contains comprehensive integration tests for the OVIM (OpenShift Virtual Infrastructure Manager) system.

## Test Overview

The integration tests cover the following components:

### API Integration Tests (`api_test.go`)
- **Authentication Flow**: Login, token validation, unauthorized access
- **Organization Management**: CRUD operations, role-based access control
- **VDC Management**: Virtual Data Center operations and resource quotas
- **VM Catalog**: Template listing and retrieval
- **VM Lifecycle**: Virtual machine creation, management, and deletion
- **Role-Based Access Control**: System admin, org admin, and regular user permissions
- **Health Endpoint**: Service health checks

### Database Integration Tests (`database_test.go`)
- **Memory Storage**: In-memory storage implementation testing
- **PostgreSQL Storage**: Database storage implementation testing
- **CRUD Operations**: Create, Read, Update, Delete for all entities
- **Data Integrity**: Relationship validation and constraint testing
- **Storage Health**: Connection and ping testing

## Test Structure

### Test Users
- **System Admin**: `admin` / `adminpassword`
  - Full system access
  - Organization management
  - Cross-organization visibility

- **Organization Admin**: `orgadmin` / `adminpassword`
  - Organization-scoped permissions
  - VM creation and management
  - VDC access within organization

- **Regular User**: `user` / `userpassword`
  - Read-only catalog access
  - VM visibility within organization
  - Restricted administrative access

### Test Data
- **Organizations**: Acme Corporation, Development Team
- **VDCs**: Resource-constrained virtual data centers
- **Templates**: RHEL 9.2, Ubuntu 22.04 LTS, CentOS Stream 9
- **VMs**: Created and managed during test execution

## Running Integration Tests

### Prerequisites
- Go 1.19+ installed
- PostgreSQL database running (for database tests)
- Test dependencies installed (`go mod tidy`)

### Running Tests

#### All Integration Tests
```bash
go test ./test/integration/...
```

#### API Tests Only
```bash
go test ./test/integration/ -run TestAuthentication
go test ./test/integration/ -run TestOrganization
go test ./test/integration/ -run TestVDC
go test ./test/integration/ -run TestVMCatalog
go test ./test/integration/ -run TestVMLifecycle
go test ./test/integration/ -run TestRoleBasedAccessControl
```

#### Database Tests Only
```bash
# Memory storage only
go test ./test/integration/ -run TestMemoryStorageIntegration

# PostgreSQL storage (requires running database)
go test ./test/integration/ -run TestPostgreSQLStorageIntegration
```

#### Quick Tests (Skip PostgreSQL)
```bash
go test ./test/integration/ -short
```

### Database Setup for PostgreSQL Tests

Create a test database:
```sql
CREATE DATABASE ovim_test;
GRANT ALL PRIVILEGES ON DATABASE ovim_test TO ovim;
```

Or use Docker/Podman:
```bash
podman run --name postgres-test -e POSTGRES_USER=ovim -e POSTGRES_PASSWORD=ovim123 -e POSTGRES_DB=ovim_test -p 5433:5432 -d postgres:16-alpine
```

Update the DSN in `database_test.go` if using different connection parameters.

## Test Scenarios Covered

### Authentication & Authorization
- ✅ Successful login with valid credentials
- ✅ Failed login with invalid credentials
- ✅ JWT token generation and validation
- ✅ Role-based endpoint access control
- ✅ Unauthorized access rejection

### Organization Management
- ✅ List all organizations (system admin)
- ✅ Get specific organization details
- ✅ Access control for different user roles
- ✅ Organization-scoped data filtering

### Virtual Data Center Operations
- ✅ VDC listing and filtering by organization
- ✅ Resource quota validation and retrieval
- ✅ VDC-organization relationship integrity
- ✅ Namespace and metadata handling

### VM Template Catalog
- ✅ Template listing for all user types
- ✅ Template detail retrieval
- ✅ Metadata and specification validation
- ✅ Image URL and OS information accuracy

### Virtual Machine Lifecycle
- ✅ VM creation with proper validation
- ✅ Organization and VDC association
- ✅ Template-based resource allocation
- ✅ Status tracking and updates
- ✅ Access control for VM operations
- ✅ Metadata and custom field handling

### Database Operations
- ✅ CRUD operations for all entities
- ✅ Relationship integrity and constraints
- ✅ Custom data types (StringMap for JSONB)
- ✅ Storage interface compatibility
- ✅ Connection health and error handling

## Integration with CI/CD

These tests are designed to run in automated environments:

```bash
# Fast tests for quick feedback
make test-integration-fast

# Full tests including database
make test-integration-full

# Coverage report
make test-integration-coverage
```

## Troubleshooting

### Common Issues

1. **PostgreSQL Connection Failed**
   - Ensure PostgreSQL is running
   - Check connection parameters in test
   - Verify test database exists and user has permissions

2. **Authentication Failures**
   - Confirm seeded test data is loaded
   - Check JWT secret configuration
   - Verify password hashing compatibility

3. **API Test Failures**
   - Ensure all dependencies are available
   - Check for port conflicts
   - Verify test isolation (clean state between tests)

### Debugging

Run tests with verbose output:
```bash
go test -v ./test/integration/...
```

Run specific test with debugging:
```bash
go test -v -run TestAuthenticationFlow ./test/integration/
```

## Contributing

When adding new features:

1. Add corresponding integration tests
2. Update test data as needed
3. Ensure tests are isolated and repeatable
4. Add documentation for new test scenarios
5. Update this README with new test coverage

## Performance Considerations

- Tests use in-memory storage by default for speed
- PostgreSQL tests run only when not in `-short` mode
- Test data is minimal but representative
- Cleanup ensures no test pollution between runs