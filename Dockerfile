FROM golang:1.24-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build server, controller, and spoke agent
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ovim_server ./cmd/ovim-server
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ovim_controller ./cmd/controller
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ovim_spoke_agent ./cmd/ovim-spoke-agent

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app/

# Copy all binaries to /usr/local/bin for system-wide access
COPY --from=builder /app/ovim_server /usr/local/bin/
COPY --from=builder /app/ovim_controller /usr/local/bin/
COPY --from=builder /app/ovim_spoke_agent /usr/local/bin/

# Make binaries executable and ensure proper ownership
RUN chmod +x /usr/local/bin/ovim_server /usr/local/bin/ovim_controller /usr/local/bin/ovim_spoke_agent && \
    chown root:root /usr/local/bin/ovim_server /usr/local/bin/ovim_controller /usr/local/bin/ovim_spoke_agent

# Expose default ports
EXPOSE 8080 8443

ENV OVIM_PORT=8443
ENV OVIM_ENVIRONMENT=production

# Default to server, but can be overridden
CMD ["ovim_server"]