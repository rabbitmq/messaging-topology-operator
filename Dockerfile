# Build the manager binary
FROM golang:1.17 as builder

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

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -tags timetzdata -o manager main.go

# ---------------------------------------
FROM alpine:latest as etc-builder

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
