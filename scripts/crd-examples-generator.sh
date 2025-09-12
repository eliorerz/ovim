#!/bin/bash

# CRD Examples Generator
# Generates example YAML files for OVIM CRDs
# Usage: ./scripts/crd-examples-generator.sh [options]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
EXAMPLES_DIR="$PROJECT_ROOT/config/crd/examples"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

show_help() {
    cat << EOF
CRD Examples Generator

USAGE:
    $0 [OPTIONS]

OPTIONS:
    -h, --help          Show this help message
    -o, --output DIR    Output directory (default: config/crd/examples)

DESCRIPTION:
    Generates example YAML files for all OVIM CRDs:
    - Organization examples (basic and advanced)
    - VirtualDataCenter examples (development and production)
    - Catalog examples (different source types)

EXAMPLES:
    $0                                  # Generate to default location
    $0 -o /tmp/examples                # Generate to custom directory

EOF
}

generate_organization_examples() {
    local output_dir="$1"
    
    log_info "Generating Organization examples..."
    
    # Basic Organization
    cat > "$output_dir/organization-basic.yaml" << 'EOF'
apiVersion: ovim.io/v1
kind: Organization
metadata:
  name: acme-corp
spec:
  displayName: "Acme Corporation"
  description: "Main corporate organization"
  admins:
    - "acme-admins"
    - "platform-team"
  isEnabled: true
EOF

    # Advanced Organization with catalogs
    cat > "$output_dir/organization-advanced.yaml" << 'EOF'
apiVersion: ovim.io/v1
kind: Organization
metadata:
  name: engineering-org
spec:
  displayName: "Engineering Organization"
  description: "Software engineering teams with multiple catalogs"
  admins:
    - "engineering-leads"
    - "platform-admins"
    - "devops-team"
  isEnabled: true
  catalogs:
    - name: "vm-templates"
      type: "vm-template"
      enabled: true
    - name: "app-stacks"
      type: "application-stack"
      enabled: true
    - name: "mixed-catalog"
      type: "mixed"
      enabled: false
EOF
}

generate_vdc_examples() {
    local output_dir="$1"
    
    log_info "Generating VirtualDataCenter examples..."
    
    # Development VDC
    cat > "$output_dir/vdc-development.yaml" << 'EOF'
apiVersion: ovim.io/v1
kind: VirtualDataCenter
metadata:
  name: development
  namespace: org-acme-corp
spec:
  organizationRef: acme-corp
  displayName: "Development Environment"
  description: "Development and testing workloads"
  quota:
    cpu: "50"
    memory: "200Gi"
    storage: "5Ti"
    pods: 100
    virtualMachines: 25
  limitRange:
    minCpu: 100      # 0.1 core in millicores
    maxCpu: 4000     # 4 cores in millicores
    minMemory: 256   # 256 MiB
    maxMemory: 16384 # 16 GiB in MiB
  networkPolicy: default
EOF

    # Production VDC with advanced configuration
    cat > "$output_dir/vdc-production.yaml" << 'EOF'
apiVersion: ovim.io/v1
kind: VirtualDataCenter
metadata:
  name: production
  namespace: org-acme-corp
spec:
  organizationRef: acme-corp
  displayName: "Production Environment"
  description: "Production workloads with strict resource limits and network isolation"
  quota:
    cpu: "200"
    memory: "1000Gi"
    storage: "50Ti"
    pods: 500
    virtualMachines: 100
  limitRange:
    minCpu: 100      # 0.1 core in millicores
    maxCpu: 16000    # 16 cores in millicores
    minMemory: 512   # 512 MiB
    maxMemory: 65536 # 64 GiB in MiB
  networkPolicy: isolated
  customNetworkConfig:
    allowedNamespaces: ["monitoring", "logging", "backup"]
    allowedPorts:
      - port: 80
        protocol: "TCP"
      - port: 443
        protocol: "TCP"
      - port: 9090
        protocol: "TCP"
  catalogRestrictions: ["vm-templates", "app-stacks"]
EOF

    # Staging VDC
    cat > "$output_dir/vdc-staging.yaml" << 'EOF'
apiVersion: ovim.io/v1
kind: VirtualDataCenter
metadata:
  name: staging
  namespace: org-acme-corp
spec:
  organizationRef: acme-corp
  displayName: "Staging Environment"
  description: "Pre-production testing and validation"
  quota:
    cpu: "100"
    memory: "500Gi"
    storage: "20Ti"
    pods: 200
    virtualMachines: 50
  limitRange:
    minCpu: 100
    maxCpu: 8000
    minMemory: 256
    maxMemory: 32768
  networkPolicy: custom
  customNetworkConfig:
    allowedNamespaces: ["monitoring"]
    allowedPorts:
      - port: 80
        protocol: "TCP"
      - port: 443
        protocol: "TCP"
EOF
}

generate_catalog_examples() {
    local output_dir="$1"
    
    log_info "Generating Catalog examples..."
    
    # OCI Registry Catalog
    cat > "$output_dir/catalog-oci-registry.yaml" << 'EOF'
apiVersion: ovim.io/v1
kind: Catalog
metadata:
  name: vm-templates
  namespace: org-acme-corp
spec:
  organizationRef: acme-corp
  displayName: "Corporate VM Templates"
  description: "Approved VM templates from corporate registry"
  type: vm-template
  source:
    type: oci
    url: "registry.corp.com/vm-templates"
    refreshInterval: "1h"
    insecureSkipTLSVerify: false
  filters:
    includePatterns: ["*.yaml", "*.yml"]
    excludePatterns: ["*-dev-*", "*-test-*"]
    tags: ["approved", "security-scanned"]
  permissions:
    allowedVDCs: ["production", "staging"]
    allowedGroups: ["developers", "ops-team"]
    readOnly: true
  isEnabled: true
EOF

    # Git Repository Catalog
    cat > "$output_dir/catalog-git-repo.yaml" << 'EOF'
apiVersion: ovim.io/v1
kind: Catalog
metadata:
  name: app-stacks
  namespace: org-acme-corp
spec:
  organizationRef: acme-corp
  displayName: "Application Stack Templates"
  description: "Curated application stacks from Git repository"
  type: application-stack
  source:
    type: git
    url: "https://github.com/company/app-templates.git"
    branch: "main"
    path: "/templates"
    credentials: "git-credentials-secret"
    refreshInterval: "30m"
  filters:
    includePatterns: ["*.yaml", "*.yml", "*.json"]
    excludePatterns: ["**/test/**", "**/.git/**"]
    tags: ["production-ready", "validated"]
  permissions:
    allowedVDCs: ["production", "staging", "development"]
    allowedGroups: ["developers", "platform-team", "qa-team"]
    readOnly: true
  isEnabled: true
EOF

    # S3 Bucket Catalog
    cat > "$output_dir/catalog-s3-bucket.yaml" << 'EOF'
apiVersion: ovim.io/v1
kind: Catalog
metadata:
  name: mixed-templates
  namespace: org-acme-corp
spec:
  organizationRef: acme-corp
  displayName: "Mixed Template Catalog"
  description: "Combined VM and application templates from S3"
  type: mixed
  source:
    type: s3
    url: "s3://company-templates/catalog"
    path: "/templates"
    credentials: "s3-credentials-secret"
    refreshInterval: "2h"
  filters:
    includePatterns: ["templates/**/*.yaml"]
    excludePatterns: ["**/deprecated/**"]
  permissions:
    allowedVDCs: [] # Empty means all VDCs can access
    allowedGroups: ["all-developers"]
    readOnly: true
  isEnabled: true
EOF

    # HTTP/HTTPS Catalog
    cat > "$output_dir/catalog-http-source.yaml" << 'EOF'
apiVersion: ovim.io/v1
kind: Catalog
metadata:
  name: public-templates
  namespace: org-acme-corp
spec:
  organizationRef: acme-corp
  displayName: "Public Template Catalog"
  description: "Public templates from HTTP source"
  type: vm-template
  source:
    type: http
    url: "https://templates.example.com/catalog"
    refreshInterval: "4h"
    insecureSkipTLSVerify: false
  filters:
    includePatterns: ["*.yaml"]
    tags: ["public", "community"]
  permissions:
    allowedVDCs: ["development"]
    readOnly: true
  isEnabled: true
EOF
}

generate_complete_example() {
    local output_dir="$1"
    
    log_info "Generating complete deployment example..."
    
    cat > "$output_dir/complete-deployment.yaml" << 'EOF'
# Complete OVIM CRD Deployment Example
# This file demonstrates a full organization setup with VDCs and catalogs

---
apiVersion: ovim.io/v1
kind: Organization
metadata:
  name: tech-company
spec:
  displayName: "Technology Company"
  description: "Main technology organization with multiple environments"
  admins:
    - "platform-admins"
    - "ops-team"
    - "security-team"
  isEnabled: true

---
apiVersion: ovim.io/v1
kind: VirtualDataCenter
metadata:
  name: production
  namespace: org-tech-company
spec:
  organizationRef: tech-company
  displayName: "Production Environment"
  description: "Production workloads with high availability"
  quota:
    cpu: "500"
    memory: "2000Gi"
    storage: "100Ti"
    pods: 1000
    virtualMachines: 200
  limitRange:
    minCpu: 100
    maxCpu: 32000
    minMemory: 512
    maxMemory: 131072
  networkPolicy: isolated
  catalogRestrictions: ["vm-templates"]

---
apiVersion: ovim.io/v1
kind: VirtualDataCenter
metadata:
  name: development
  namespace: org-tech-company
spec:
  organizationRef: tech-company
  displayName: "Development Environment"
  description: "Development and testing workloads"
  quota:
    cpu: "100"
    memory: "400Gi"
    storage: "20Ti"
    pods: 200
    virtualMachines: 50
  limitRange:
    minCpu: 100
    maxCpu: 8000
    minMemory: 256
    maxMemory: 32768
  networkPolicy: default

---
apiVersion: ovim.io/v1
kind: Catalog
metadata:
  name: vm-templates
  namespace: org-tech-company
spec:
  organizationRef: tech-company
  displayName: "VM Template Catalog"
  description: "Approved VM templates for all environments"
  type: vm-template
  source:
    type: oci
    url: "registry.tech-company.com/vm-templates"
    refreshInterval: "1h"
  permissions:
    allowedVDCs: ["production", "development"]
    readOnly: true
  isEnabled: true

---
apiVersion: ovim.io/v1
kind: Catalog
metadata:
  name: app-stacks
  namespace: org-tech-company
spec:
  organizationRef: tech-company
  displayName: "Application Stacks"
  description: "Pre-configured application stack templates"
  type: application-stack
  source:
    type: git
    url: "https://github.com/tech-company/app-stacks.git"
    branch: "main"
    refreshInterval: "30m"
  permissions:
    allowedVDCs: ["development"]
    readOnly: true
  isEnabled: true
EOF
}

main() {
    local output_dir="$EXAMPLES_DIR"
    
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_help
                exit 0
                ;;
            -o|--output)
                output_dir="$2"
                shift 2
                ;;
            *)
                log_error "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done
    
    log_info "Creating examples directory: $output_dir"
    mkdir -p "$output_dir"
    
    generate_organization_examples "$output_dir"
    generate_vdc_examples "$output_dir"
    generate_catalog_examples "$output_dir"
    generate_complete_example "$output_dir"
    
    log_info "CRD examples generated successfully in $output_dir"
    log_info "Generated files:"
    find "$output_dir" -name "*.yaml" -exec basename {} \; | sort | sed 's/^/  - /'
}

main "$@"