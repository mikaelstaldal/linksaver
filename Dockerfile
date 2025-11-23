# Use Go 1.25 as the base image for building
FROM golang:1.25@sha256:698183780de28062f4ef46f82a79ec0ae69d2d22f7b160cf69f71ea8d98bf25d AS builder

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the Go source code
COPY cmd/ cmd
COPY empty-efs.go ui/efs.go
COPY empty/ ui/

# Build the application
RUN go build -v -o /app/linksaver ./cmd/linksaver

FROM chromedp/headless-shell:latest@sha256:24b6acd183756b9cdc9b2c951141cefbc645a9b6a18341975babf0911a30c7e5

ENV CHROMEDP="wss://localhost:9222"

WORKDIR /data

COPY --from=builder /app/linksaver /linksaver/linkserver
COPY run.sh /linksaver/run.sh
COPY ui/ /linksaver/ui

# Expose the default port
EXPOSE 8080

ENTRYPOINT [ "/linksaver/run.sh" ]
