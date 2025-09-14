# OVIM Documentation Hub

## Overview

This documentation hub provides comprehensive coverage of the OpenShift Virtual Infrastructure Manager (OVIM) platform, including detailed information about the controller, backend API, and frontend UI components.

## Documentation Structure

### 📁 `/controller/` - Kubernetes Controller Documentation
- **[README.md](controller/README.md)**: Complete controller architecture and CRD documentation
- **Features**: CRD definitions, RBAC configuration, controller workflows
- **Audience**: Platform engineers, DevOps teams, Kubernetes administrators

### 📁 `/backend/` - Backend API Documentation
- **[README.md](backend/README.md)**: Comprehensive REST API documentation
- **Features**: API endpoints, authentication, workflows, error handling
- **Audience**: Backend developers, API consumers, integration teams

### 📁 `/ui/` - Frontend User Interface Documentation
- **[README.md](ui/README.md)**: Complete UI architecture and framework documentation
- **[components.md](ui/components.md)**: Detailed component documentation
- **[diagrams.md](ui/diagrams.md)**: User workflow and interaction diagrams
- **[authentication.md](ui/authentication.md)**: Authentication and state management
- **[user-experience.md](ui/user-experience.md)**: UX design and accessibility
- **[deployment.md](ui/deployment.md)**: Deployment and configuration guide
- **Audience**: Frontend developers, UX designers, DevOps teams

### 📁 `/diagrams/` - System Architecture Diagrams
- **[architecture-overview.md](diagrams/architecture-overview.md)**: Complete system architecture with Mermaid diagrams
- **Features**: Multi-tenant hierarchy, data flow, component interactions
- **Audience**: All stakeholders, architects, technical leadership

### 📁 `/api-reference/` - API Specification
- **[openapi-spec.yaml](api-reference/openapi-spec.yaml)**: Complete OpenAPI 3.0 specification
- **Features**: All endpoints, schemas, authentication, examples
- **Audience**: API consumers, integration developers, testing teams

## Quick Navigation Guide

### For Developers

#### Backend Developers
1. Start with [Backend API Documentation](backend/README.md)
2. Review [OpenAPI Specification](api-reference/openapi-spec.yaml)
3. Check [Controller Documentation](controller/README.md) for CRD integration
4. Review [Architecture Diagrams](diagrams/architecture-overview.md) for system context

#### Frontend Developers
1. Start with [UI Architecture](ui/README.md)
2. Review [Component Documentation](ui/components.md)
3. Study [Authentication & State Management](ui/authentication.md)
4. Check [User Experience Guidelines](ui/user-experience.md)
5. Follow [Deployment Guide](ui/deployment.md) for setup

#### Platform Engineers
1. Begin with [Controller Documentation](controller/README.md)
2. Review [Architecture Diagrams](diagrams/architecture-overview.md)
3. Study [Backend API Integration](backend/README.md)
4. Check [UI Deployment](ui/deployment.md) for complete stack deployment

### For Operators

#### System Administrators
1. Review [Controller RBAC Configuration](controller/README.md#rbac-configuration)
2. Study [Backend API Security](backend/README.md#security-features)
3. Check [UI Deployment Security](ui/deployment.md#security-configuration)
4. Review [Architecture Overview](diagrams/architecture-overview.md) for operational context

#### DevOps Teams
1. Start with [Deployment Documentation](ui/deployment.md)
2. Review [Backend Deployment](backend/README.md#development--testing)
3. Study [Controller Installation](controller/README.md#installation-and-configuration)
4. Check [System Architecture](diagrams/architecture-overview.md) for infrastructure planning

### For Users

#### End Users
1. Review [User Experience Documentation](ui/user-experience.md)
2. Check [UI Workflow Diagrams](ui/diagrams.md) for interaction patterns
3. Study role-based features in [Backend API](backend/README.md#authorization-levels)

#### API Consumers
1. Start with [OpenAPI Specification](api-reference/openapi-spec.yaml)
2. Review [Backend API Documentation](backend/README.md)
3. Check [Authentication Flows](ui/authentication.md) for integration guidance

## Key Features Documented

### Multi-Tenant Architecture
- **Organizations**: Top-level tenant boundaries with admin groups
- **VDCs**: Resource pools within organizations with quotas and isolation
- **RBAC**: Four-tier permission system (System Admin, Org Admin, VDC Admin, User)

### Virtual Machine Management
- **Lifecycle**: Complete VM deployment, management, and monitoring
- **Templates**: Catalog-based VM provisioning with OpenShift integration
- **Resources**: CPU, memory, storage allocation with quota enforcement

### Security & Authentication
- **JWT Authentication**: Token-based authentication with configurable expiration
- **OIDC Integration**: Enterprise authentication provider support
- **Role-Based Access**: Hierarchical permissions with resource isolation
- **TLS Security**: End-to-end encryption with certificate management

### Monitoring & Observability
- **Resource Metrics**: Real-time usage tracking across all levels
- **Health Monitoring**: Component health checks and status reporting
- **Alert Management**: Configurable thresholds and notifications
- **Audit Logging**: Complete action tracking and compliance

### Integration Points
- **Kubernetes**: Native CRD and controller integration
- **OpenShift**: Template catalog and project management
- **KubeVirt**: Virtual machine provisioning and lifecycle
- **PostgreSQL**: Persistent data storage and session management

## Technology Stack Overview

### Backend Technologies
- **Go 1.21+**: Primary backend language
- **Gin Framework**: HTTP web framework
- **controller-runtime**: Kubernetes controller framework
- **PostgreSQL**: Primary database
- **JWT**: Authentication tokens
- **OpenAPI 3.0**: API specification

### Frontend Technologies
- **React 18**: Modern UI framework
- **TypeScript**: Type-safe JavaScript
- **PatternFly**: Enterprise design system
- **React Router**: Client-side routing
- **Axios**: HTTP client library

### Infrastructure Technologies
- **Kubernetes**: Container orchestration
- **OpenShift**: Enterprise Kubernetes platform
- **KubeVirt**: Virtual machine management
- **CDI**: Container Data Importer
- **Nginx**: Web server and reverse proxy
- **Container**: Docker/Podman containerization

## Getting Started

### Quick Setup for Development
1. **Backend**: Follow [Backend Development Guide](backend/README.md#development--testing)
2. **UI**: Follow [UI Development Setup](ui/deployment.md#development-environment)
3. **Controller**: Follow [Controller Development](controller/README.md#installation-and-configuration)

### Production Deployment
1. **Review Architecture**: Study [system diagrams](diagrams/architecture-overview.md)
2. **Deploy Backend**: Follow [backend deployment](backend/README.md#integration-points)
3. **Deploy UI**: Follow [UI deployment guide](ui/deployment.md#kubernetes-deployment)
4. **Configure RBAC**: Set up [controller RBAC](controller/README.md#rbac-configuration)

## Contributing

When contributing to OVIM, please:

1. **Review Relevant Documentation**: Understand the component you're working on
2. **Follow Patterns**: Use established patterns documented in each section
3. **Update Documentation**: Keep documentation in sync with code changes
4. **Test Integration**: Verify changes work across the full stack

## Support and Troubleshooting

### Documentation Issues
- Check the specific component documentation for troubleshooting sections
- Review [deployment guides](ui/deployment.md#troubleshooting) for common issues
- Study [architecture diagrams](diagrams/architecture-overview.md) for system understanding

### Technical Support
- Backend issues: See [Backend Documentation](backend/README.md)
- UI issues: See [UI Documentation](ui/README.md)
- Controller issues: See [Controller Documentation](controller/README.md)
- Integration issues: Review [Architecture Overview](diagrams/architecture-overview.md)

## Version Information

This documentation covers OVIM version 1.0.0 and includes:
- Complete system architecture documentation
- Comprehensive API reference (OpenAPI 3.0)
- Detailed component documentation
- Deployment and operational guides
- User experience and design guidelines

For the most up-to-date information, always refer to the latest documentation in this repository.

---

**Note**: This documentation is generated and maintained alongside the OVIM codebase. For questions or clarifications, please refer to the specific component documentation or contact the development team.