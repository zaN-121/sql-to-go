# --- Stage 1: Builder ---
FROM golang:1.23-alpine AS builder

# Install git (kadang diperlukan jika ada dependencies dari github)
RUN apk add --no-cache git

WORKDIR /app

# Copy go.mod dulu untuk memanfaatkan Docker caching
COPY go.mod ./

# Download dependencies
RUN go mod download

# Copy source code yang diperlukan
COPY main.go ./
COPY converter.go ./
COPY web ./web/

# Build Binary
# CGO_ENABLED=0: Membuat binary statis (bisa jalan di alpine/scratch)
# -ldflags="-w -s": Menghapus debug info agar binary lebih kecil
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o sql-to-go main.go converter.go

# --- Stage 2: Runner ---
FROM alpine:latest

# Install sertifikat CA (penting jika app perlu request HTTPS keluar)
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy binary dari stage builder
COPY --from=builder /app/sql-to-go .

# Expose port
EXPOSE 7860

# Jalankan
CMD ["./sql-to-go"]