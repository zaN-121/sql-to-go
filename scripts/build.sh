#!/bin/bash

echo "🔨 Building SQL to Go Converter..."

# Build the binary
go build -o sql-to-go main.go converter.go

if [ $? -eq 0 ]; then
    echo "✅ Build successful!"
    echo "📦 Binary: ./sql-to-go"
    echo ""
    echo "To run the server:"
    echo "  ./sql-to-go"
    echo ""
    echo "Then open: http://localhost:8080"
else
    echo "❌ Build failed!"
    exit 1
fi
