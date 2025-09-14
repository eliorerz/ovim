# OVIM UI Deployment & Configuration

## Overview

This document provides comprehensive guidance for deploying, configuring, and maintaining the OVIM UI in various environments, from development to production Kubernetes clusters.

## Development Environment

### Prerequisites

#### System Requirements
- **Node.js**: Version 18+ (LTS recommended)
- **npm**: Version 8+ (comes with Node.js)
- **Git**: For version control
- **Code Editor**: VS Code recommended with TypeScript extensions

#### Hardware Requirements
- **RAM**: Minimum 4GB, recommended 8GB+
- **Storage**: 1GB for dependencies
- **CPU**: Modern multi-core processor

### Local Development Setup

#### Initial Setup
```bash
# Clone the UI repository
cd /path/to/ovim-updated-second-feature
cd ovim-ui

# Install dependencies
npm install

# Verify installation
npm audit
```

#### Environment Configuration

Create `.env.local` file for development:
```bash
# .env.local
REACT_APP_API_URL=https://localhost:8443
REACT_APP_ENVIRONMENT=development
NODE_ENV=development

# Optional: Enable development features
REACT_APP_DEBUG=true
REACT_APP_MOCK_API=false
```

#### Development Scripts

```bash
# Start development server
npm start
# Access: http://localhost:3000
# Hot reload enabled

# Run tests
npm test
# Interactive test runner

# Run tests with coverage
npm test -- --coverage --watchAll=false

# Type checking
npm run type-check

# Linting
npm run lint
npm run lint:fix

# Build for testing
npm run build
```

#### Development Server Configuration

The development server includes:
- **Hot Module Replacement**: Instant updates on code changes
- **API Proxy**: Requests to `/api/*` proxied to backend
- **Error Overlay**: Visual error reporting
- **Source Maps**: Debug support

#### Proxy Configuration
```json
// package.json
{
  "proxy": "https://localhost:8443"
}
```

This proxies API requests to the OVIM backend during development.

### IDE Configuration

#### VS Code Settings
```json
// .vscode/settings.json
{
  "typescript.preferences.includePackageJsonAutoImports": "off",
  "editor.codeActionsOnSave": {
    "source.fixAll.eslint": true
  },
  "editor.formatOnSave": true,
  "editor.defaultFormatter": "esbenp.prettier-vscode"
}
```

#### Recommended Extensions
- ES7+ React/Redux/React-Native snippets
- TypeScript Hero
- ESLint
- Prettier
- Auto Rename Tag

## Build Process

### Production Build

#### Build Command
```bash
npm run build
```

#### Build Process Steps
1. **Type Checking**: Validates TypeScript types
2. **Linting**: Checks code quality
3. **Bundle Creation**: Webpack creates optimized bundles
4. **Asset Optimization**: Images, CSS, and JS optimization
5. **Service Worker**: Generates SW for caching (if enabled)

#### Build Output Structure
```
build/
├── static/
│   ├── css/
│   │   ├── main.[hash].css
│   │   └── main.[hash].css.map
│   ├── js/
│   │   ├── main.[hash].js
│   │   ├── main.[hash].js.map
│   │   ├── [chunk].[hash].chunk.js
│   │   └── runtime-main.[hash].js
│   └── media/
│       └── [assets].[hash].[ext]
├── index.html
├── favicon.ico
├── manifest.json
└── robots.txt
```

#### Build Optimization
- **Code Splitting**: Route-based and component-based
- **Tree Shaking**: Eliminates unused code
- **Minification**: JavaScript and CSS compression
- **Asset Optimization**: Image compression and optimization
- **Cache Busting**: File hashing for cache invalidation

### Build Analysis

#### Bundle Analysis
```bash
# Install bundle analyzer
npm install --save-dev webpack-bundle-analyzer

# Generate bundle report
npm run build
npx webpack-bundle-analyzer build/static/js/*.js
```

#### Performance Metrics
```bash
# Lighthouse CI for performance auditing
npm install -g @lhci/cli
lhci autorun
```

## Container Deployment

### Docker Configuration

#### Multi-Stage Dockerfile
```dockerfile
# Build stage
FROM node:18-alpine as builder

# Set working directory
WORKDIR /app

# Copy package files
COPY package*.json ./

# Install dependencies
RUN npm ci --only=production --silent

# Copy source code
COPY . .

# Build application
RUN npm run build

# Production stage
FROM nginx:alpine

# Copy custom nginx configuration
COPY nginx.conf /etc/nginx/nginx.conf

# Copy built application
COPY --from=builder /app/build /usr/share/nginx/html

# Create nginx user and set permissions
RUN addgroup -g 1001 -S nginx && \
    adduser -S -D -H -u 1001 -h /var/cache/nginx -s /sbin/nologin -G nginx -g nginx nginx && \
    chown -R nginx:nginx /var/cache/nginx && \
    chown -R nginx:nginx /var/log/nginx && \
    chown -R nginx:nginx /etc/nginx/conf.d && \
    touch /var/run/nginx.pid && \
    chown -R nginx:nginx /var/run/nginx.pid

# Switch to non-root user
USER nginx

# Expose ports
EXPOSE 8080 8443

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -f https://localhost:8443/healthz || exit 1

# Start nginx
CMD ["nginx", "-g", "daemon off;"]
```

#### Local Development Dockerfile
```dockerfile
# Dockerfile.local - for local development with hot reload
FROM node:18-alpine

WORKDIR /app

# Copy package files
COPY package*.json ./

# Install all dependencies (including dev)
RUN npm install

# Copy source code
COPY . .

# Expose development port
EXPOSE 3000

# Start development server
CMD ["npm", "start"]
```

### Nginx Configuration

#### Production Nginx Config
```nginx
# nginx.conf
events {
    worker_connections 1024;
}

http {
    include       /etc/nginx/mime.types;
    default_type  application/octet-stream;

    # Logging
    log_format main '$remote_addr - $remote_user [$time_local] "$request" '
                    '$status $body_bytes_sent "$http_referer" '
                    '"$http_user_agent" "$http_x_forwarded_for"';

    access_log /var/log/nginx/access.log main;
    error_log /var/log/nginx/error.log warn;

    # Performance
    sendfile on;
    tcp_nopush on;
    tcp_nodelay on;
    keepalive_timeout 65;
    types_hash_max_size 2048;

    # Gzip compression
    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_proxied any;
    gzip_comp_level 6;
    gzip_types
        application/atom+xml
        application/javascript
        application/json
        application/ld+json
        application/manifest+json
        application/rss+xml
        application/vnd.geo+json
        application/vnd.ms-fontobject
        application/x-font-ttf
        application/x-web-app-manifest+json
        application/xhtml+xml
        application/xml
        font/opentype
        image/bmp
        image/svg+xml
        image/x-icon
        text/cache-manifest
        text/css
        text/plain
        text/vcard
        text/vnd.rim.location.xloc
        text/vtt
        text/x-component
        text/x-cross-domain-policy;

    # Security headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header Referrer-Policy "no-referrer-when-downgrade" always;
    add_header Content-Security-Policy "default-src 'self' http: https: data: blob: 'unsafe-inline'" always;

    # HTTP to HTTPS redirect
    server {
        listen 8080;
        listen [::]:8080;
        server_name _;
        return 301 https://$server_name:8443$request_uri;
    }

    # HTTPS server
    server {
        listen 8443 ssl http2;
        listen [::]:8443 ssl http2;
        server_name _;

        # SSL configuration
        ssl_certificate /etc/nginx/ssl/server.crt;
        ssl_certificate_key /etc/nginx/ssl/server.key;
        ssl_session_timeout 1d;
        ssl_session_cache shared:MozTLS:10m;
        ssl_session_tickets off;

        # Modern SSL configuration
        ssl_protocols TLSv1.2 TLSv1.3;
        ssl_ciphers ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384;
        ssl_prefer_server_ciphers off;

        # HSTS
        add_header Strict-Transport-Security "max-age=63072000" always;

        # Document root
        root /usr/share/nginx/html;
        index index.html index.htm;

        # Handle client routing (React Router)
        location / {
            try_files $uri $uri/ /index.html;
        }

        # API proxy to OVIM backend
        location /api/ {
            proxy_pass https://ovim-server:8443;
            proxy_ssl_verify off;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
            proxy_connect_timeout 30s;
            proxy_send_timeout 30s;
            proxy_read_timeout 30s;
        }

        # Health check proxy
        location /health {
            proxy_pass https://ovim-server:8443/health;
            proxy_ssl_verify off;
            proxy_connect_timeout 5s;
            proxy_send_timeout 5s;
            proxy_read_timeout 5s;
        }

        # Static assets with long expiration
        location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2|ttf|eot)$ {
            expires 1y;
            add_header Cache-Control "public, immutable";
        }

        # Health check endpoint
        location /healthz {
            access_log off;
            return 200 "healthy\n";
            add_header Content-Type text/plain;
        }
    }
}
```

### Container Build and Registry

#### Build Commands
```bash
# Build production image
docker build -t ovim-ui:latest .

# Build development image
docker build -f Dockerfile.local -t ovim-ui:dev .

# Build with specific tag
docker build -t quay.io/eerez/ovim-ui:v1.0.0 .
```

#### Registry Operations
```bash
# Tag for registry
docker tag ovim-ui:latest quay.io/eerez/ovim-ui:latest

# Push to registry
docker push quay.io/eerez/ovim-ui:latest

# Pull from registry
docker pull quay.io/eerez/ovim-ui:latest
```

## Kubernetes Deployment

### Deployment Manifests

#### UI Deployment
```yaml
# ovim-ui-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ovim-ui
  namespace: ovim-system
  labels:
    app.kubernetes.io/name: ovim
    app.kubernetes.io/component: ui
    app.kubernetes.io/version: "1.0.0"
spec:
  replicas: 2
  selector:
    matchLabels:
      app.kubernetes.io/name: ovim
      app.kubernetes.io/component: ui
  template:
    metadata:
      labels:
        app.kubernetes.io/name: ovim
        app.kubernetes.io/component: ui
        app.kubernetes.io/version: "1.0.0"
    spec:
      securityContext:
        runAsNonRoot: true
        seccompProfile:
          type: RuntimeDefault
      containers:
        - name: ui
          image: quay.io/eerez/ovim-ui:latest
          imagePullPolicy: Always
          securityContext:
            runAsNonRoot: true
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities:
              drop:
                - ALL
            seccompProfile:
              type: RuntimeDefault
          env:
            - name: NODE_ENV
              value: "production"
            - name: REACT_APP_API_URL
              value: "/api"
            - name: REACT_APP_ENVIRONMENT
              value: "production"
          ports:
            - name: http
              containerPort: 8080
              protocol: TCP
            - name: https
              containerPort: 8443
              protocol: TCP
          livenessProbe:
            httpGet:
              path: /healthz
              port: https
              scheme: HTTPS
            initialDelaySeconds: 30
            periodSeconds: 10
            timeoutSeconds: 5
            successThreshold: 1
            failureThreshold: 3
          readinessProbe:
            httpGet:
              path: /healthz
              port: https
              scheme: HTTPS
            initialDelaySeconds: 5
            periodSeconds: 5
            timeoutSeconds: 3
            successThreshold: 1
            failureThreshold: 3
          volumeMounts:
            - name: nginx-config
              mountPath: /etc/nginx/nginx.conf
              subPath: nginx.conf
              readOnly: true
            - name: nginx-cache
              mountPath: /var/cache/nginx
            - name: nginx-run
              mountPath: /var/run
            - name: nginx-logs
              mountPath: /var/log/nginx
          resources:
            limits:
              memory: 256Mi
              cpu: 200m
            requests:
              memory: 128Mi
              cpu: 50m
      volumes:
        - name: nginx-config
          configMap:
            name: ovim-ui-config
        - name: nginx-cache
          emptyDir: {}
        - name: nginx-run
          emptyDir: {}
        - name: nginx-logs
          emptyDir: {}
```

#### UI Service
```yaml
# ovim-ui-service.yaml
apiVersion: v1
kind: Service
metadata:
  name: ovim-ui
  namespace: ovim-system
  labels:
    app.kubernetes.io/name: ovim
    app.kubernetes.io/component: ui
spec:
  type: ClusterIP
  ports:
    - name: http
      port: 80
      targetPort: 8080
      protocol: TCP
    - name: https
      port: 443
      targetPort: 8443
      protocol: TCP
  selector:
    app.kubernetes.io/name: ovim
    app.kubernetes.io/component: ui
```

#### ConfigMap for Nginx
```yaml
# ovim-ui-configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: ovim-ui-config
  namespace: ovim-system
  labels:
    app.kubernetes.io/name: ovim
    app.kubernetes.io/component: ui
data:
  nginx.conf: |
    # [Include full nginx.conf content from above]
```

### Ingress Configuration

#### OpenShift Route
```yaml
# ovim-ui-route.yaml
apiVersion: route.openshift.io/v1
kind: Route
metadata:
  name: ovim-ui
  namespace: ovim-system
  labels:
    app.kubernetes.io/name: ovim
    app.kubernetes.io/component: ui
spec:
  host: ovim.apps.cluster-domain.com
  to:
    kind: Service
    name: ovim-ui
    weight: 100
  port:
    targetPort: https
  tls:
    termination: passthrough
    insecureEdgeTerminationPolicy: Redirect
  wildcardPolicy: None
```

#### Kubernetes Ingress
```yaml
# ovim-ui-ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ovim-ui
  namespace: ovim-system
  labels:
    app.kubernetes.io/name: ovim
    app.kubernetes.io/component: ui
  annotations:
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    nginx.ingress.kubernetes.io/backend-protocol: "HTTPS"
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
spec:
  ingressClassName: nginx
  tls:
    - hosts:
        - ovim.example.com
      secretName: ovim-ui-tls
  rules:
    - host: ovim.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: ovim-ui
                port:
                  number: 443
```

### Deployment Scripts

#### Deployment Automation
```bash
#!/bin/bash
# deploy-ui.sh

set -e

# Configuration
NAMESPACE=${OVIM_NAMESPACE:-ovim-system}
IMAGE_TAG=${OVIM_IMAGE_TAG:-latest}
UI_IMAGE=${OVIM_UI_IMAGE:-quay.io/eerez/ovim-ui}

echo "Deploying OVIM UI..."
echo "Namespace: $NAMESPACE"
echo "Image: $UI_IMAGE:$IMAGE_TAG"

# Create namespace if it doesn't exist
kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -

# Apply ConfigMap
kubectl apply -f config/ui/ovim-ui-configmap.yaml -n $NAMESPACE

# Update deployment with new image
kubectl set image deployment/ovim-ui ui=$UI_IMAGE:$IMAGE_TAG -n $NAMESPACE

# Wait for rollout
kubectl rollout status deployment/ovim-ui -n $NAMESPACE --timeout=300s

# Verify deployment
kubectl get pods -l app.kubernetes.io/component=ui -n $NAMESPACE

echo "OVIM UI deployment completed successfully"
```

#### Update Script
```bash
#!/bin/bash
# update-ui.sh

set -e

NAMESPACE=${OVIM_NAMESPACE:-ovim-system}
NEW_TAG=${1:-latest}

if [ -z "$1" ]; then
  echo "Usage: $0 <image-tag>"
  exit 1
fi

echo "Updating OVIM UI to tag: $NEW_TAG"

# Update deployment
kubectl set image deployment/ovim-ui ui=quay.io/eerez/ovim-ui:$NEW_TAG -n $NAMESPACE

# Wait for rollout
kubectl rollout status deployment/ovim-ui -n $NAMESPACE

echo "Update completed"
```

## Environment Configuration

### Environment Variables

#### Development Environment
```bash
# .env.development
REACT_APP_API_URL=https://localhost:8443
REACT_APP_ENVIRONMENT=development
REACT_APP_DEBUG=true
REACT_APP_MOCK_API=false
```

#### Staging Environment
```bash
# .env.staging
REACT_APP_API_URL=/api
REACT_APP_ENVIRONMENT=staging
REACT_APP_DEBUG=false
REACT_APP_ANALYTICS_ENABLED=false
```

#### Production Environment
```bash
# .env.production
REACT_APP_API_URL=/api
REACT_APP_ENVIRONMENT=production
REACT_APP_DEBUG=false
REACT_APP_ANALYTICS_ENABLED=true
REACT_APP_ERROR_REPORTING=true
```

### Configuration Management

#### Runtime Configuration
```typescript
// src/config/runtime.ts
interface RuntimeConfig {
  apiUrl: string;
  environment: string;
  debug: boolean;
  features: {
    analytics: boolean;
    errorReporting: boolean;
    betaFeatures: boolean;
  };
}

export const runtimeConfig: RuntimeConfig = {
  apiUrl: process.env.REACT_APP_API_URL || '/api',
  environment: process.env.REACT_APP_ENVIRONMENT || 'development',
  debug: process.env.REACT_APP_DEBUG === 'true',
  features: {
    analytics: process.env.REACT_APP_ANALYTICS_ENABLED === 'true',
    errorReporting: process.env.REACT_APP_ERROR_REPORTING === 'true',
    betaFeatures: process.env.REACT_APP_BETA_FEATURES === 'true'
  }
};
```

#### Feature Flags
```typescript
// src/utils/featureFlags.ts
export const featureFlags = {
  newDashboard: runtimeConfig.features.betaFeatures,
  advancedMetrics: runtimeConfig.environment === 'production',
  debugPanel: runtimeConfig.debug
};

export const useFeatureFlag = (flag: keyof typeof featureFlags): boolean => {
  return featureFlags[flag];
};
```

## Monitoring and Observability

### Health Checks

#### Application Health
```typescript
// Health check endpoint implementation
export const healthCheck = {
  endpoint: '/healthz',
  checks: [
    'nginx_status',
    'static_files',
    'backend_connectivity'
  ]
};
```

#### Kubernetes Probes
```yaml
# Liveness probe - restart if unhealthy
livenessProbe:
  httpGet:
    path: /healthz
    port: https
    scheme: HTTPS
  initialDelaySeconds: 30
  periodSeconds: 10
  timeoutSeconds: 5
  failureThreshold: 3

# Readiness probe - remove from service if not ready
readinessProbe:
  httpGet:
    path: /healthz
    port: https
    scheme: HTTPS
  initialDelaySeconds: 5
  periodSeconds: 5
  timeoutSeconds: 3
  failureThreshold: 3
```

### Logging

#### Nginx Access Logs
```nginx
log_format json_combined escape=json
  '{'
    '"time_local":"$time_local",'
    '"remote_addr":"$remote_addr",'
    '"remote_user":"$remote_user",'
    '"request":"$request",'
    '"status": "$status",'
    '"body_bytes_sent":"$body_bytes_sent",'
    '"request_time":"$request_time",'
    '"http_referrer":"$http_referer",'
    '"http_user_agent":"$http_user_agent"'
  '}';

access_log /var/log/nginx/access.log json_combined;
```

#### Application Logging
```typescript
// Client-side error tracking
window.addEventListener('error', (event) => {
  if (runtimeConfig.features.errorReporting) {
    console.error('Global error:', event.error);
    // Send to monitoring service
  }
});

window.addEventListener('unhandledrejection', (event) => {
  if (runtimeConfig.features.errorReporting) {
    console.error('Unhandled promise rejection:', event.reason);
    // Send to monitoring service
  }
});
```

### Metrics Collection

#### Performance Metrics
```typescript
// Performance monitoring
import { getCLS, getFID, getFCP, getLCP, getTTFB } from 'web-vitals';

const sendToAnalytics = (metric: any) => {
  if (runtimeConfig.features.analytics) {
    // Send metrics to analytics service
    console.log('Performance metric:', metric);
  }
};

getCLS(sendToAnalytics);
getFID(sendToAnalytics);
getFCP(sendToAnalytics);
getLCP(sendToAnalytics);
getTTFB(sendToAnalytics);
```

## Security Configuration

### Content Security Policy
```nginx
add_header Content-Security-Policy "
  default-src 'self';
  script-src 'self' 'unsafe-inline' 'unsafe-eval';
  style-src 'self' 'unsafe-inline';
  img-src 'self' data: blob:;
  font-src 'self' data:;
  connect-src 'self' wss: https:;
  frame-ancestors 'none';
  base-uri 'self';
  form-action 'self';
" always;
```

### HTTPS Configuration
```nginx
# SSL/TLS configuration
ssl_protocols TLSv1.2 TLSv1.3;
ssl_ciphers ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256;
ssl_prefer_server_ciphers off;
ssl_session_timeout 1d;
ssl_session_cache shared:SSL:50m;
ssl_session_tickets off;

# HSTS
add_header Strict-Transport-Security "max-age=63072000" always;
```

### Runtime Security
```typescript
// Prevent common attacks
if (typeof window !== 'undefined') {
  // Disable right-click in production
  if (runtimeConfig.environment === 'production') {
    document.addEventListener('contextmenu', (e) => e.preventDefault());
  }

  // Disable developer tools detection
  const devtools = { open: false };
  setInterval(() => {
    if (devtools.open) {
      // Handle dev tools detection
    }
  }, 500);
}
```

## Troubleshooting

### Common Issues

#### Build Failures
```bash
# Clear cache and reinstall
rm -rf node_modules package-lock.json
npm install

# Check Node.js version
node --version
npm --version

# Update dependencies
npm update
```

#### Container Issues
```bash
# Check container logs
kubectl logs deployment/ovim-ui -n ovim-system

# Debug container
kubectl exec -it deployment/ovim-ui -n ovim-system -- /bin/sh

# Check nginx configuration
kubectl exec -it deployment/ovim-ui -n ovim-system -- nginx -t
```

#### Network Issues
```bash
# Test API connectivity
kubectl port-forward svc/ovim-ui 8443:443 -n ovim-system
curl -k https://localhost:8443/healthz

# Check ingress
kubectl describe ingress ovim-ui -n ovim-system
```

### Debugging Commands

#### Application Debugging
```bash
# Check application status
kubectl get pods -l app.kubernetes.io/component=ui -n ovim-system

# View recent logs
kubectl logs deployment/ovim-ui -n ovim-system --tail=100

# Monitor logs in real-time
kubectl logs deployment/ovim-ui -n ovim-system -f

# Describe deployment
kubectl describe deployment ovim-ui -n ovim-system
```

#### Performance Debugging
```bash
# Check resource usage
kubectl top pods -l app.kubernetes.io/component=ui -n ovim-system

# Monitor resource usage
watch kubectl top pods -l app.kubernetes.io/component=ui -n ovim-system
```

This comprehensive deployment documentation provides everything needed to successfully deploy, configure, and maintain the OVIM UI in various environments.