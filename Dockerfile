# Use the official Golang image as the build stage
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

# Set the working directory
WORKDIR /app

# Copy the Go module files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

ARG TARGETOS
ARG TARGETARCH
# Build the Go app
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -o aws-ebs-az-aware-webhook

# Use a minimal base image for the final build
FROM alpine:latest

# Set the working directory in the final image
WORKDIR /root

# Copy the compiled binary from the builder stage
COPY --from=builder /app/aws-ebs-az-aware-webhook .

# Expose the port the app runs on
EXPOSE 8080

# Command to run the executable
CMD ["./aws-ebs-az-aware-webhook"]
