# Use Go 1.24 as the base image for building
FROM golang:1.24@sha256:20a022e5112a144aa7b7aeb3f22ebf2cdaefcc4aac0d64e8deeee8cdc18b9c0f AS builder

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the Go source code
COPY cmd/ cmd

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
