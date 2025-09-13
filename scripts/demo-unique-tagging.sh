#!/bin/bash

# Demo script to show the unique tagging system for OVIM containers
# This demonstrates different tagging strategies for tracking deployments

set -euo pipefail

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
PURPLE='\033[0;35m'
NC='\033[0m' # No Color

echo -e "${CYAN}🏷️  OVIM Unique Tagging System Demo${NC}"
echo "=========================================="
echo ""

echo -e "${BLUE}📋 Current Version Information:${NC}"
make version
echo ""

echo -e "${YELLOW}🔖 Tagging Strategies Available:${NC}"
echo ""

echo -e "${BLUE}1. Latest Tag (Default):${NC}"
echo "   Command: make deploy-stack"
echo "   Tags: latest"
echo "   Use case: Development, always use newest"
echo ""

echo -e "${BLUE}2. Custom Tag:${NC}"
echo "   Command: OVIM_IMAGE_TAG=v1.0.0 make deploy-stack"
echo "   Tags: v1.0.0, latest"
echo "   Use case: Release versions"
echo ""

echo -e "${BLUE}3. Unique Timestamp Tags:${NC}"
echo "   Command: make deploy-stack-unique"
echo "   Tags: YYYYMMDD-HHMMSS-gitcommit, latest"
echo "   Use case: Tracking individual deployments"
echo "   Example: 20250912-221600-c4e5714"
echo ""

echo -e "${BLUE}4. Manual Unique Tag:${NC}"
echo "   Command: ./scripts/deploy-stack.sh --use-unique-tag"
echo "   Tags: Auto-generated timestamp-git, latest"
echo "   Use case: Script-based deployments"
echo ""

echo -e "${GREEN}🚀 Benefits of Unique Tagging:${NC}"
echo "✅ Track every deployment individually"
echo "✅ Easy rollback to specific versions"  
echo "✅ No confusion between deployments"
echo "✅ Git commit traceability"
echo "✅ Timestamp for deployment time tracking"
echo ""

echo -e "${PURPLE}📊 Image Registry Structure:${NC}"
echo "quay.io/eerez/ovim:"
echo "  ├── latest (always latest build)"
echo "  ├── 20250912-221600-c4e5714 (unique build)"
echo "  ├── 20250912-223045-c4e5714 (another unique build)"
echo "  └── v1.0.0 (release tag)"
echo ""
echo "quay.io/eerez/ovim-ui:"
echo "  ├── latest (always latest build)"
echo "  ├── 20250912-221600-c4e5714 (matching server build)"
echo "  ├── 20250912-223045-c4e5714 (another matching build)"
echo "  └── v1.0.0 (matching release tag)"
echo ""

echo -e "${CYAN}💡 Usage Examples:${NC}"
echo ""
echo -e "${YELLOW}Deploy with unique tracking:${NC}"
echo "make deploy-stack-unique"
echo ""
echo -e "${YELLOW}Deploy specific version:${NC}"
echo "OVIM_IMAGE_TAG=20250912-221600-c4e5714 make deploy-stack"
echo ""
echo -e "${YELLOW}Build and push with unique tags:${NC}"
echo "make container-push-all"
echo "# Creates: latest, YYYYMMDD-HHMMSS-commit"
echo ""
echo -e "${YELLOW}Check available images:${NC}"
echo "podman images | grep quay.io/eerez"
echo ""

echo -e "${GREEN}🎯 Deployment Tracking Benefits:${NC}"
echo "• Each deployment has a unique identifier"
echo "• Git commit ensures code traceability"
echo "• Timestamp provides deployment chronology"
echo "• Easy to identify and rollback specific deployments"
echo "• Perfect for CI/CD pipeline integration"
echo ""

echo -e "${CYAN}🎉 Unique tagging system ready for production!${NC}"