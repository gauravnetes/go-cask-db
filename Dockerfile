# ==========================================
# Stage 1: The Builder
# ==========================================
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy the module file 
COPY go.mod ./

# Copy the source code
COPY cmd/ ./cmd/
COPY internal/ ./internal/

# Build a statically linked Go binary. 
# This strips out debugging info to make the file incredibly small.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o cask-server cmd/server/main.go


# ==========================================
# Stage 2: The Production Image
# ==========================================
FROM alpine:latest

WORKDIR /app

# Copy ONLY the compiled binary from the builder stage
COPY --from=builder /app/cask-server .

# Create the data directory so the database has somewhere to write its SSTables
RUN mkdir -p ./data

# Expose our custom TCP port to the outside world
EXPOSE 8080

# Start the database engine
CMD ["./cask-server"]