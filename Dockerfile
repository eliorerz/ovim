FROM golang:1.21-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ovim-backend .

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata
WORKDIR /root/

COPY --from=builder /app/ovim-backend .

EXPOSE 8080

ENV OVIM_PORT=8080
ENV OVIM_ENVIRONMENT=production

CMD ["./ovim-backend"]