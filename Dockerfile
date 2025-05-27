# Build the manager binary
FROM --platform=$BUILDPLATFORM golang:1.24 AS builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY internal/ internal/
COPY rabbitmqclient/ rabbitmqclient/

# Build
ARG TARGETOS
ARG TARGETARCH
ENV GOOS=$TARGETOS
ENV GOARCH=$TARGETARCH

ARG FIPS_MODE=off
ENV GOFIPS140=$FIPS_MODE

# Build
RUN CGO_ENABLED=0 GO111MODULE=on go build -a -tags timetzdata -o manager main.go

# ---------------------------------------
FROM alpine:latest AS etc-builder

RUN echo "messaging-topology-operator:x:1001:" > /etc/group && \
    echo "messaging-topology-operator:x:1001:1001::/home/messaging-topology-operator:/usr/sbin/nologin" > /etc/passwd

RUN apk add -U --no-cache ca-certificates

# ---------------------------------------
FROM scratch
WORKDIR /
COPY --from=builder /workspace/manager .
COPY --from=etc-builder /etc/passwd /etc/group /etc/
COPY --from=etc-builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
USER 1001:1001

ENTRYPOINT ["/manager"]
