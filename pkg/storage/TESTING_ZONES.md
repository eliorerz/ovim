# Zone Storage Testing

This document describes how to run tests for the ACM zones integration.

## Running Tests

### Memory Storage Tests (Default)

Run zone tests using in-memory storage (no database required):

```bash
# Run all zone-related tests
go test ./pkg/storage -v -run "Zone"

# Run specific test suites
go test ./pkg/storage -v -run TestZoneOperations
go test ./pkg/storage -v -run TestOrganizationZoneQuotaOperations
go test ./pkg/storage -v -run TestZoneUtilization
go test ./pkg/storage -v -run TestOrganizationZoneAccess
go test ./pkg/storage -v -run TestZoneModelHelpers
```

### PostgreSQL Storage Tests

To run PostgreSQL-specific tests, you need a test database:

1. Set up a test PostgreSQL database
2. Set the `TEST_DATABASE_URL` environment variable:

```bash
export TEST_DATABASE_URL="postgres://user:password@localhost:5432/ovim_test?sslmode=disable"
go test ./pkg/storage -v -run TestPostgresZoneOperations
```

### All Storage Tests

Run all storage tests (both memory and PostgreSQL where available):

```bash
go test ./pkg/storage -v
```

## Test Coverage

The zone storage tests cover:

### Zone Operations
- **CRUD Operations**: Create, Read, Update, Delete zones
- **Validation**: Input validation and error handling
- **Timestamps**: Creation and update time tracking
- **Concurrency**: Thread-safe operations

### Organization Zone Quota Operations
- **CRUD Operations**: Manage organization access and quotas per zone
- **Relationships**: Foreign key constraints and data relationships
- **Access Control**: Organization permissions within zones

### Zone Utilization
- **Calculation**: Real-time utilization from VDC deployments
- **Aggregation**: Resource usage across multiple VDCs
- **Views**: Database view integration for complex queries

### Organization Zone Access
- **Access Queries**: Get organization access rights per zone
- **Usage Tracking**: Current resource usage per organization per zone
- **Filtering**: Organization-specific and zone-specific queries

### Zone Model Helpers
- **Capacity Calculation**: Available vs allocated resources
- **Health Checks**: Zone availability status
- **Utilization Percentages**: Resource usage calculations
- **VDC Accommodation**: Capacity planning for new VDCs

### Error Scenarios
- **Invalid Input**: Null/empty values
- **Not Found**: Non-existent resources
- **Constraints**: Database constraint violations
- **Concurrency**: Race condition handling

## Test Data

Tests use realistic test data:
- Zones with AWS/OpenShift-like configurations
- Organizations with proper resource quotas
- VDCs with various deployment states
- Resource measurements in standard units (GB, CPU cores)

## Performance Testing

Basic performance tests are included:
- Zone operation benchmarks
- Batch operation testing
- Concurrent access patterns

Run benchmarks with:

```bash
go test ./pkg/storage -bench=BenchmarkZone -benchmem
```

## Integration with Existing Tests

Zone tests integrate seamlessly with existing storage tests:
- Uses the same test patterns and utilities
- Follows existing error handling conventions
- Maintains compatibility with both storage implementations
- Provides comprehensive coverage for the new zone functionality