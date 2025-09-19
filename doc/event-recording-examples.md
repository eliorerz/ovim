# OVIM Event Recording Examples

## Overview

This document provides practical examples of event recording in the OVIM system, including API usage, common scenarios, troubleshooting approaches, and integration patterns.

## API Usage Examples

### Retrieving Recent Events

Get the 10 most recent events for dashboard display:

```bash
curl -H "Authorization: Bearer $JWT_TOKEN" \
  "https://ovim-api.example.com/api/v1/events/recent?limit=10"
```

**Response:**
```json
{
  "events": [
    {
      "id": "evt-123456",
      "name": "org-my-company.17d2e5c8a1b2f9e7",
      "namespace": "ovim-system",
      "type": "Normal",
      "reason": "OrganizationCreated",
      "message": "Organization 'my-company' created successfully by admin",
      "component": "ovim-api",
      "involved_object_kind": "Organization",
      "involved_object_name": "my-company",
      "first_timestamp": "2024-01-15T14:30:00Z",
      "last_timestamp": "2024-01-15T14:30:00Z",
      "count": 1
    },
    {
      "id": "evt-789012",
      "name": "vdc-dev-environment.17d2e5c8a1b2f9e8",
      "namespace": "org-my-company",
      "type": "Normal",
      "reason": "VDCCreated",
      "message": "VDC 'dev-environment' created in organization 'my-company'",
      "component": "ovim-api",
      "involved_object_kind": "VirtualDataCenter",
      "involved_object_name": "dev-environment",
      "first_timestamp": "2024-01-15T14:35:00Z",
      "last_timestamp": "2024-01-15T14:35:00Z",
      "count": 1
    }
  ],
  "count": 2
}
```

### Filtering Events by Type

Get warning events from the last hour:

```bash
curl -H "Authorization: Bearer $JWT_TOKEN" \
  "https://ovim-api.example.com/api/v1/events?type=Warning&limit=50"
```

### Filtering Events by Component

Get events from the ovim-controller:

```bash
curl -H "Authorization: Bearer $JWT_TOKEN" \
  "https://ovim-api.example.com/api/v1/events?component=ovim-controller"
```

### Filtering Events by Namespace

Get events from a specific organization namespace:

```bash
curl -H "Authorization: Bearer $JWT_TOKEN" \
  "https://ovim-api.example.com/api/v1/events?namespace=org-my-company"
```

### Paginated Event Retrieval

Get events with pagination for large deployments:

```bash
curl -H "Authorization: Bearer $JWT_TOKEN" \
  "https://ovim-api.example.com/api/v1/events?page=2&limit=25"
```

## Common Event Scenarios

### Organization Lifecycle Events

#### Organization Creation
```json
{
  "type": "Normal",
  "reason": "OrganizationCreated",
  "message": "Organization 'acme-corp' created successfully by system admin 'admin'",
  "component": "ovim-api",
  "involved_object_kind": "Organization",
  "involved_object_name": "acme-corp",
  "namespace": "ovim-system"
}
```

#### Organization Update
```json
{
  "type": "Normal",
  "reason": "OrganizationUpdated",
  "message": "Organization 'acme-corp' configuration updated by org admin 'jane.doe'",
  "component": "ovim-api",
  "involved_object_kind": "Organization",
  "involved_object_name": "acme-corp",
  "namespace": "ovim-system"
}
```

#### Organization Deletion
```json
{
  "type": "Normal",
  "reason": "OrganizationDeleted",
  "message": "Organization 'acme-corp' deleted by system admin 'admin'",
  "component": "ovim-api",
  "involved_object_kind": "Organization",
  "involved_object_name": "acme-corp",
  "namespace": "ovim-system"
}
```

### VDC Lifecycle Events

#### VDC Creation
```json
{
  "type": "Normal",
  "reason": "VDCCreated",
  "message": "VDC 'production' created in organization 'acme-corp' by org admin 'jane.doe'",
  "component": "ovim-api",
  "involved_object_kind": "VirtualDataCenter",
  "involved_object_name": "production",
  "namespace": "org-acme-corp"
}
```

#### VDC Resource Quota Exceeded
```json
{
  "type": "Warning",
  "reason": "QuotaExceeded",
  "message": "VDC 'production' CPU quota exceeded: requested 16 cores, available 4 cores",
  "component": "ovim-api",
  "involved_object_kind": "VirtualDataCenter",
  "involved_object_name": "production",
  "namespace": "org-acme-corp"
}
```

#### VDC Deletion with Active VMs
```json
{
  "type": "Warning",
  "reason": "DeletionBlocked",
  "message": "VDC 'staging' deletion blocked: 3 active VMs must be removed first",
  "component": "ovim-api",
  "involved_object_kind": "VirtualDataCenter",
  "involved_object_name": "staging",
  "namespace": "org-acme-corp"
}
```

### VM Lifecycle Events

#### VM Creation Success
```json
{
  "type": "Normal",
  "reason": "VMCreated",
  "message": "VM 'web-server-01' created successfully from template 'rhel8-base'",
  "component": "ovim-api",
  "involved_object_kind": "VirtualMachine",
  "involved_object_name": "web-server-01",
  "namespace": "vdc-acme-corp-production"
}
```

#### VM Provisioning Failure
```json
{
  "type": "Warning",
  "reason": "ProvisioningFailed",
  "message": "VM 'database-server' provisioning failed: insufficient memory in VDC 'production'",
  "component": "kubevirt-provisioner",
  "involved_object_kind": "VirtualMachine",
  "involved_object_name": "database-server",
  "namespace": "vdc-acme-corp-production"
}
```

#### VM Power State Changes
```json
{
  "type": "Normal",
  "reason": "VMStarted",
  "message": "VM 'web-server-01' started by user 'developer@acme-corp.com'",
  "component": "ovim-api",
  "involved_object_kind": "VirtualMachine",
  "involved_object_name": "web-server-01",
  "namespace": "vdc-acme-corp-production"
}
```

### Authentication and Security Events

#### Failed Authentication
```json
{
  "type": "Warning",
  "reason": "AuthenticationFailed",
  "message": "Failed login attempt for user 'attacker@external.com' from IP 192.168.1.100",
  "component": "ovim-api",
  "involved_object_kind": "User",
  "involved_object_name": "unknown",
  "namespace": "ovim-system"
}
```

#### Permission Denied
```json
{
  "type": "Warning",
  "reason": "PermissionDenied",
  "message": "User 'user@acme-corp.com' attempted to access organization 'competitor-corp'",
  "component": "ovim-api",
  "involved_object_kind": "User",
  "involved_object_name": "user@acme-corp.com",
  "namespace": "ovim-system"
}
```

## Kubectl Integration Examples

### View All OVIM-Related Events

```bash
kubectl get events --all-namespaces --field-selector involvedObject.apiVersion=ovim.io/v1
```

### View Events for Specific Organization

```bash
kubectl get events -n org-my-company --sort-by='.lastTimestamp'
```

### Watch Events in Real-Time

```bash
kubectl get events -n ovim-system --watch
```

### Filter Events by Reason

```bash
kubectl get events --field-selector reason=VDCCreated --all-namespaces
```

### Get Event Details

```bash
kubectl describe event org-my-company.17d2e5c8a1b2f9e7 -n ovim-system
```

## Monitoring and Alerting Examples

### Prometheus Queries for Event Metrics

Count events by type over the last hour:
```promql
increase(kubernetes_event_count{type="Warning"}[1h])
```

Track failed VM creations:
```promql
increase(kubernetes_event_count{reason="ProvisioningFailed"}[5m])
```

Monitor authentication failures:
```promql
increase(kubernetes_event_count{reason="AuthenticationFailed"}[1h])
```

### Grafana Dashboard Queries

Recent warning events:
```promql
topk(10, increase(kubernetes_event_count{type="Warning"}[1h]))
```

Event rate by component:
```promql
rate(kubernetes_event_count[5m]) by (component)
```

### Alert Manager Rules

```yaml
groups:
- name: ovim_events
  rules:
  - alert: HighFailureRate
    expr: rate(kubernetes_event_count{type="Warning"}[5m]) > 0.1
    for: 2m
    labels:
      severity: warning
    annotations:
      summary: High rate of warning events detected
      description: "{{ $value }} warning events per second over the last 5 minutes"

  - alert: AuthenticationFailures
    expr: increase(kubernetes_event_count{reason="AuthenticationFailed"}[1h]) > 5
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: Multiple authentication failures
      description: "{{ $value }} failed authentication attempts in the last hour"
```

## Troubleshooting with Events

### Debugging VM Creation Issues

1. **Check for VM creation events:**
```bash
kubectl get events -n vdc-my-company-dev --field-selector involvedObject.name=my-vm
```

2. **Look for provisioning failures:**
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "https://ovim-api/api/v1/events?type=Warning&component=kubevirt-provisioner"
```

3. **Check resource constraints:**
```bash
kubectl get events -n vdc-my-company-dev --field-selector reason=QuotaExceeded
```

### Investigating Permission Issues

1. **Check authentication events:**
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "https://ovim-api/api/v1/events?reason=AuthenticationFailed"
```

2. **Look for permission denials:**
```bash
kubectl get events --field-selector reason=PermissionDenied --all-namespaces
```

### Monitoring System Health

1. **Get all warning events from the last hour:**
```bash
kubectl get events --field-selector type=Warning --all-namespaces \
  --sort-by='.lastTimestamp' | grep $(date -d '1 hour ago' '+%Y-%m-%d')
```

2. **Check controller events:**
```bash
kubectl get events -n ovim-system --field-selector involvedObject.kind=Organization
```

## Event Integration Patterns

### UI Dashboard Integration

JavaScript example for fetching recent events:

```javascript
async function fetchRecentEvents() {
  try {
    const response = await fetch('/api/v1/events/recent?limit=5', {
      headers: {
        'Authorization': `Bearer ${authToken}`,
        'Content-Type': 'application/json'
      }
    });

    const data = await response.json();
    return data.events;
  } catch (error) {
    console.error('Failed to fetch events:', error);
    return [];
  }
}

// Update UI with events
function updateEventsPanel(events) {
  const panel = document.getElementById('events-panel');
  panel.innerHTML = events.map(event => `
    <div class="event-item ${event.type.toLowerCase()}">
      <span class="event-time">${formatTime(event.last_timestamp)}</span>
      <span class="event-message">${event.message}</span>
      <span class="event-component">${event.component}</span>
    </div>
  `).join('');
}
```

### Log Aggregation Integration

Fluentd configuration for collecting OVIM events:

```yaml
<source>
  @type kubernetes_events
  tag kubernetes.events
  de_dot_separator _
</source>

<filter kubernetes.events>
  @type grep
  <regexp>
    key $.involvedObject.apiVersion
    pattern ^ovim\.io/
  </regexp>
</filter>

<match kubernetes.events>
  @type elasticsearch
  host elasticsearch.logging.svc.cluster.local
  port 9200
  index_name ovim-events
</match>
```

### Webhook Integration

Example webhook payload for external integration:

```json
{
  "webhook_type": "ovim_event",
  "timestamp": "2024-01-15T14:30:00Z",
  "event": {
    "id": "evt-123456",
    "type": "Warning",
    "reason": "QuotaExceeded",
    "message": "VDC 'production' CPU quota exceeded",
    "component": "ovim-api",
    "organization": "acme-corp",
    "vdc": "production",
    "involved_object": {
      "kind": "VirtualDataCenter",
      "name": "production",
      "namespace": "org-acme-corp"
    }
  }
}
```

## Performance Optimization

### Efficient Event Querying

1. **Use appropriate page sizes:**
```bash
# Good: Reasonable page size
curl "https://ovim-api/api/v1/events?limit=50"

# Avoid: Excessive page size
curl "https://ovim-api/api/v1/events?limit=1000"
```

2. **Filter events at the source:**
```bash
# Good: Server-side filtering
curl "https://ovim-api/api/v1/events?type=Warning&component=ovim-api"

# Avoid: Client-side filtering of all events
curl "https://ovim-api/api/v1/events?limit=200" | jq '.events[] | select(.type=="Warning")'
```

3. **Use recent events endpoint for dashboards:**
```bash
# Good: Optimized endpoint
curl "https://ovim-api/api/v1/events/recent?limit=10"

# Avoid: Full query for recent events
curl "https://ovim-api/api/v1/events?limit=10&page=1"
```

### Event Retention Considerations

- Kubernetes events have a default TTL of 1 hour
- For longer retention, implement database event storage
- Use log aggregation systems for historical analysis
- Archive critical events to external storage systems

## Related Documentation

- [Event Recording System Architecture](event-recording.md)
- [Backend API Documentation](backend/README.md#events-api)
- [UI Components Guide](ui/components.md#events-panel)
- [Monitoring and Alerting Setup](../docs/monitoring.md)