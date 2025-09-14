# OVIM Architecture Diagrams

## Overall System Architecture

```mermaid
graph TB
    subgraph "External Users"
        UI[Web UI]
        CLI[CLI Tools]
        API_CLIENT[API Clients]
    end

    subgraph "OVIM Platform"
        subgraph "API Layer"
            LB[Load Balancer]
            API[OVIM API Server]
            AUTH[Authentication Service]
        end

        subgraph "Control Plane"
            CTRL[OVIM Controller]
            WEBHOOK[Admission Webhooks]
        end

        subgraph "Storage Layer"
            PG[(PostgreSQL)]
            ETCD[(etcd)]
        end
    end

    subgraph "Kubernetes/OpenShift Cluster"
        subgraph "System Namespaces"
            OVIM_NS[ovim-system]
            KUBE_NS[kube-system]
        end

        subgraph "Organization Namespaces"
            ORG1_NS[acme-corp]
            ORG2_NS[other-org]
        end

        subgraph "VDC Workload Namespaces"
            VDC1_NS[acme-corp-dev]
            VDC2_NS[acme-corp-prod]
            VDC3_NS[other-org-test]
        end

        subgraph "Infrastructure"
            KV[KubeVirt]
            CDI[CDI]
            OCP[OpenShift]
        end
    end

    UI --> LB
    CLI --> LB
    API_CLIENT --> LB
    LB --> API
    API --> AUTH
    API --> PG
    API --> ETCD
    CTRL --> ETCD
    CTRL --> ORG1_NS
    CTRL --> ORG2_NS
    CTRL --> VDC1_NS
    CTRL --> VDC2_NS
    CTRL --> VDC3_NS
    WEBHOOK --> ETCD
    CTRL --> KV
    CTRL --> CDI
    CTRL --> OCP
```

## Multi-Tenant Resource Hierarchy

```mermaid
graph TD
    CLUSTER[Kubernetes Cluster]

    subgraph "Organization Level"
        ORG1[Organization: acme-corp]
        ORG2[Organization: other-org]
    end

    subgraph "Organization Namespace: acme-corp"
        ORG1_NS[Namespace: acme-corp]
        VDC1_CRD[VDC CRD: development]
        VDC2_CRD[VDC CRD: production]
        CAT1_CRD[Catalog CRD: vm-templates]
        RBAC1[Org Admin RoleBindings]
    end

    subgraph "VDC Workload Namespaces"
        VDC1_WL[Namespace: acme-corp-dev]
        VDC2_WL[Namespace: acme-corp-prod]

        subgraph "VDC1 Resources"
            QUOTA1[ResourceQuota]
            LIMIT1[LimitRange]
            NP1[NetworkPolicy]
            VM1[VirtualMachine]
            POD1[Pods]
            SVC1[Services]
            RBAC_VDC1[VDC Admin RoleBindings]
        end

        subgraph "VDC2 Resources"
            QUOTA2[ResourceQuota]
            LIMIT2[LimitRange]
            NP2[NetworkPolicy]
            VM2[VirtualMachine]
            POD2[Pods]
            SVC2[Services]
            RBAC_VDC2[VDC Admin RoleBindings]
        end
    end

    CLUSTER --> ORG1
    CLUSTER --> ORG2
    ORG1 --> ORG1_NS
    ORG1_NS --> VDC1_CRD
    ORG1_NS --> VDC2_CRD
    ORG1_NS --> CAT1_CRD
    ORG1_NS --> RBAC1

    VDC1_CRD --> VDC1_WL
    VDC2_CRD --> VDC2_WL

    VDC1_WL --> QUOTA1
    VDC1_WL --> LIMIT1
    VDC1_WL --> NP1
    VDC1_WL --> VM1
    VDC1_WL --> POD1
    VDC1_WL --> SVC1
    VDC1_WL --> RBAC_VDC1

    VDC2_WL --> QUOTA2
    VDC2_WL --> LIMIT2
    VDC2_WL --> NP2
    VDC2_WL --> VM2
    VDC2_WL --> POD2
    VDC2_WL --> SVC2
    VDC2_WL --> RBAC_VDC2
```

## Controller Architecture

```mermaid
graph TB
    subgraph "Controller Manager"
        MGR[Controller Manager]
        CACHE[Shared Cache]
        CLIENT[Kubernetes Client]
    end

    subgraph "Controllers"
        ORG_CTRL[Organization Controller]
        VDC_CTRL[VDC Controller]
        VM_CTRL[VM Controller]
        CAT_CTRL[Catalog Controller]
        METRICS_CTRL[Metrics Controller]
        RBAC_CTRL[RBAC Sync Controller]
    end

    subgraph "Watched Resources"
        ORG_CRD[Organization CRDs]
        VDC_CRD[VDC CRDs]
        VM_CRD[VM CRDs]
        CAT_CRD[Catalog CRDs]
        NS[Namespaces]
        QUOTA[ResourceQuotas]
        RBAC[RoleBindings]
    end

    subgraph "Managed Resources"
        CREATE_NS[Create Namespaces]
        CREATE_QUOTA[Create ResourceQuotas]
        CREATE_RBAC[Create RoleBindings]
        CREATE_NP[Create NetworkPolicies]
        SYNC_STATUS[Update Status]
        COLLECT_METRICS[Collect Metrics]
    end

    MGR --> ORG_CTRL
    MGR --> VDC_CTRL
    MGR --> VM_CTRL
    MGR --> CAT_CTRL
    MGR --> METRICS_CTRL
    MGR --> RBAC_CTRL

    MGR --> CACHE
    MGR --> CLIENT

    ORG_CTRL --> ORG_CRD
    VDC_CTRL --> VDC_CRD
    VM_CTRL --> VM_CRD
    CAT_CTRL --> CAT_CRD
    METRICS_CTRL --> NS
    RBAC_CTRL --> RBAC

    ORG_CTRL --> CREATE_NS
    ORG_CTRL --> CREATE_RBAC
    VDC_CTRL --> CREATE_NS
    VDC_CTRL --> CREATE_QUOTA
    VDC_CTRL --> CREATE_RBAC
    VDC_CTRL --> CREATE_NP
    VM_CTRL --> SYNC_STATUS
    METRICS_CTRL --> COLLECT_METRICS
    RBAC_CTRL --> CREATE_RBAC
```

## API Request Flow

```mermaid
sequenceDiagram
    participant Client
    participant LoadBalancer
    participant API as API Server
    participant Auth as Auth Service
    participant DB as PostgreSQL
    participant K8s as Kubernetes API
    participant Ctrl as Controller

    Client->>LoadBalancer: HTTP Request
    LoadBalancer->>API: Forward Request
    API->>API: CORS & Logging Middleware

    alt Authentication Required
        API->>Auth: Validate JWT Token
        Auth->>DB: Lookup User Session
        DB->>Auth: User & Role Info
        Auth->>API: Authentication Result
    end

    alt Authorization Check
        API->>API: Check Role Permissions
        API->>API: Check Resource Access
    end

    alt CRUD Operations
        API->>DB: Query/Update Database
        DB->>API: Result
        API->>K8s: Create/Update CRD
        K8s->>Ctrl: Trigger Controller
        Ctrl->>K8s: Manage Resources
    end

    API->>Client: JSON Response
```

## Authentication & Authorization Flow

```mermaid
graph TD
    subgraph "Authentication Methods"
        PWD[Username/Password]
        OIDC[OIDC Provider]
        JWT[JWT Token]
    end

    subgraph "Authentication Service"
        AUTH_SVC[Auth Service]
        TOKEN_MGR[Token Manager]
        OIDC_MGR[OIDC Manager]
    end

    subgraph "Authorization Engine"
        RBAC_ENGINE[RBAC Engine]
        ROLE_CHECK[Role Verification]
        RESOURCE_CHECK[Resource Access Check]
    end

    subgraph "User Roles"
        SYS_ADMIN[System Admin]
        ORG_ADMIN[Organization Admin]
        VDC_ADMIN[VDC Admin]
        USER[User]
    end

    subgraph "Resources"
        ORGS[Organizations]
        VDCS[VDCs]
        VMS[Virtual Machines]
        CATS[Catalogs]
    end

    PWD --> AUTH_SVC
    OIDC --> OIDC_MGR
    JWT --> TOKEN_MGR
    OIDC_MGR --> AUTH_SVC
    TOKEN_MGR --> AUTH_SVC

    AUTH_SVC --> RBAC_ENGINE
    RBAC_ENGINE --> ROLE_CHECK
    RBAC_ENGINE --> RESOURCE_CHECK

    ROLE_CHECK --> SYS_ADMIN
    ROLE_CHECK --> ORG_ADMIN
    ROLE_CHECK --> VDC_ADMIN
    ROLE_CHECK --> USER

    SYS_ADMIN --> ORGS
    SYS_ADMIN --> VDCS
    SYS_ADMIN --> VMS
    SYS_ADMIN --> CATS

    ORG_ADMIN --> VDCS
    ORG_ADMIN --> VMS
    ORG_ADMIN --> CATS

    VDC_ADMIN --> VMS

    USER --> VMS
```

## VM Provisioning Workflow

```mermaid
sequenceDiagram
    participant User
    participant API as API Server
    participant DB as Database
    participant K8s as Kubernetes
    participant KV as KubeVirt
    participant CDI
    participant Storage

    User->>API: POST /api/v1/vms
    API->>API: Validate Request
    API->>API: Check VDC Quotas
    API->>DB: Store VM Record

    API->>K8s: Create DataVolume CRD
    K8s->>CDI: Process DataVolume
    CDI->>Storage: Create PVC
    CDI->>Storage: Import/Clone Image
    Storage->>CDI: Ready
    CDI->>K8s: Update Status

    API->>K8s: Create VirtualMachine CRD
    K8s->>KV: Process VM
    KV->>K8s: Create VMI
    KV->>K8s: Create Pod
    K8s->>Storage: Mount PVC
    K8s->>KV: Pod Running
    KV->>K8s: Update VM Status

    API->>DB: Update VM Status
    API->>User: VM Created Response

    Note over User, Storage: VM is now running in VDC namespace
```

## Catalog Content Management

```mermaid
graph TD
    subgraph "Catalog Sources"
        GIT[Git Repository]
        OCI[OCI Registry]
        S3[S3 Bucket]
        HTTP[HTTP Server]
        LOCAL[Local Files]
    end

    subgraph "Catalog Controller"
        SYNC[Sync Controller]
        PARSER[Content Parser]
        VALIDATOR[Content Validator]
        FILTER[Content Filter]
    end

    subgraph "Content Processing"
        METADATA[Extract Metadata]
        TEMPLATES[Process Templates]
        IMAGES[Image References]
        TAGS[Tag Processing]
    end

    subgraph "Storage"
        DB_CAT[(Catalog Database)]
        K8S_TEMPLATES[K8s Templates]
        VM_TEMPLATES[VM Templates]
    end

    subgraph "Access Control"
        ORG_FILTER[Organization Filter]
        VDC_FILTER[VDC Filter]
        RBAC_FILTER[RBAC Filter]
    end

    GIT --> SYNC
    OCI --> SYNC
    S3 --> SYNC
    HTTP --> SYNC
    LOCAL --> SYNC

    SYNC --> PARSER
    PARSER --> VALIDATOR
    VALIDATOR --> FILTER

    FILTER --> METADATA
    FILTER --> TEMPLATES
    FILTER --> IMAGES
    FILTER --> TAGS

    METADATA --> DB_CAT
    TEMPLATES --> K8S_TEMPLATES
    TEMPLATES --> VM_TEMPLATES

    DB_CAT --> ORG_FILTER
    K8S_TEMPLATES --> VDC_FILTER
    VM_TEMPLATES --> RBAC_FILTER
```

## Resource Quota Management

```mermaid
graph TB
    subgraph "Organization Level"
        ORG_QUOTA[Organization Quotas]
        ORG_USAGE[Organization Usage]
    end

    subgraph "VDC Level"
        VDC1_QUOTA[VDC1 Quotas]
        VDC2_QUOTA[VDC2 Quotas]
        VDC1_USAGE[VDC1 Usage]
        VDC2_USAGE[VDC2 Usage]
    end

    subgraph "Workload Level"
        VM1[VM1 Resources]
        VM2[VM2 Resources]
        POD1[Pod1 Resources]
        POD2[Pod2 Resources]
    end

    subgraph "Enforcement Points"
        ADMISSION[Admission Controller]
        SCHEDULER[Scheduler]
        KUBELET[Kubelet]
    end

    subgraph "Monitoring"
        METRICS[Metrics Collector]
        ALERTS[Alert Manager]
        DASHBOARD[Dashboard]
    end

    ORG_QUOTA --> VDC1_QUOTA
    ORG_QUOTA --> VDC2_QUOTA

    VDC1_QUOTA --> VM1
    VDC1_QUOTA --> POD1
    VDC2_QUOTA --> VM2
    VDC2_QUOTA --> POD2

    VM1 --> VDC1_USAGE
    POD1 --> VDC1_USAGE
    VM2 --> VDC2_USAGE
    POD2 --> VDC2_USAGE

    VDC1_USAGE --> ORG_USAGE
    VDC2_USAGE --> ORG_USAGE

    ADMISSION --> VDC1_QUOTA
    ADMISSION --> VDC2_QUOTA
    SCHEDULER --> VDC1_QUOTA
    SCHEDULER --> VDC2_QUOTA
    KUBELET --> VDC1_QUOTA
    KUBELET --> VDC2_QUOTA

    METRICS --> VDC1_USAGE
    METRICS --> VDC2_USAGE
    METRICS --> ORG_USAGE

    METRICS --> ALERTS
    METRICS --> DASHBOARD
```

## Network Architecture

```mermaid
graph TB
    subgraph "External Access"
        INTERNET[Internet]
        VPN[VPN]
        CORP_NET[Corporate Network]
    end

    subgraph "Ingress Layer"
        LB[Load Balancer]
        INGRESS[Ingress Controller]
        ROUTES[OpenShift Routes]
    end

    subgraph "Service Layer"
        API_SVC[API Service]
        UI_SVC[UI Service]
        DB_SVC[Database Service]
    end

    subgraph "Organization Networks"
        ORG1_NET[Org1 Network]
        ORG2_NET[Org2 Network]
    end

    subgraph "VDC Networks"
        VDC1_NET[VDC1 Network]
        VDC2_NET[VDC2 Network]
        VDC3_NET[VDC3 Network]

        subgraph "Network Policies"
            NP1[VDC1 NetworkPolicy]
            NP2[VDC2 NetworkPolicy]
            NP3[VDC3 NetworkPolicy]
        end
    end

    subgraph "Workload Networks"
        VM_NET[VM Networks]
        POD_NET[Pod Networks]
        SVC_NET[Service Networks]
    end

    INTERNET --> LB
    VPN --> LB
    CORP_NET --> LB

    LB --> INGRESS
    INGRESS --> ROUTES

    ROUTES --> API_SVC
    ROUTES --> UI_SVC
    API_SVC --> DB_SVC

    API_SVC --> ORG1_NET
    API_SVC --> ORG2_NET

    ORG1_NET --> VDC1_NET
    ORG1_NET --> VDC2_NET
    ORG2_NET --> VDC3_NET

    VDC1_NET --> NP1
    VDC2_NET --> NP2
    VDC3_NET --> NP3

    NP1 --> VM_NET
    NP1 --> POD_NET
    NP1 --> SVC_NET
    NP2 --> VM_NET
    NP2 --> POD_NET
    NP2 --> SVC_NET
    NP3 --> VM_NET
    NP3 --> POD_NET
    NP3 --> SVC_NET
```

## Data Flow Architecture

```mermaid
graph LR
    subgraph "Data Sources"
        USER_INPUT[User Input]
        K8S_EVENTS[K8s Events]
        METRICS_API[Metrics API]
        LOGS[Application Logs]
    end

    subgraph "Data Processing"
        API_LAYER[API Layer]
        VALIDATION[Validation Layer]
        BUSINESS_LOGIC[Business Logic]
        CONTROLLER_LOGIC[Controller Logic]
    end

    subgraph "Data Storage"
        PG_DB[(PostgreSQL)]
        ETCD_DB[(etcd)]
        METRICS_DB[(Metrics Store)]
        LOG_STORE[(Log Store)]
    end

    subgraph "Data Consumers"
        WEB_UI[Web UI]
        CLI_TOOLS[CLI Tools]
        DASHBOARDS[Dashboards]
        ALERTS[Alerts]
        REPORTS[Reports]
    end

    USER_INPUT --> API_LAYER
    K8S_EVENTS --> CONTROLLER_LOGIC
    METRICS_API --> BUSINESS_LOGIC
    LOGS --> BUSINESS_LOGIC

    API_LAYER --> VALIDATION
    VALIDATION --> BUSINESS_LOGIC
    BUSINESS_LOGIC --> CONTROLLER_LOGIC

    BUSINESS_LOGIC --> PG_DB
    CONTROLLER_LOGIC --> ETCD_DB
    BUSINESS_LOGIC --> METRICS_DB
    BUSINESS_LOGIC --> LOG_STORE

    PG_DB --> WEB_UI
    PG_DB --> CLI_TOOLS
    METRICS_DB --> DASHBOARDS
    METRICS_DB --> ALERTS
    PG_DB --> REPORTS
    ETCD_DB --> CONTROLLER_LOGIC
```

These diagrams provide a comprehensive visual representation of the OVIM architecture, showing how all components interact and how data flows through the system. Each diagram focuses on a specific aspect of the architecture while maintaining the overall context of the multi-tenant virtual infrastructure management platform.