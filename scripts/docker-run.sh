#!/bin/bash

# Script untuk rebuild dan run Docker container
# Author: Ahmad Nazir Arrobi

set -e  # Exit on error

IMAGE_NAME="sql-to-go"
CONTAINER_NAME="sql-to-go-container"
PORT="8080"

echo "🚀 Starting Docker rebuild and run process..."
echo ""

# 1. Stop dan remove container lama jika ada
echo "🛑 Stopping and removing old container..."
if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    docker stop ${CONTAINER_NAME} 2>/dev/null || true
    docker rm ${CONTAINER_NAME} 2>/dev/null || true
    echo "✅ Old container removed"
else
    echo "ℹ️  No existing container found"
fi
echo ""

# 2. Remove image lama
echo "🗑️  Removing old Docker image..."
if docker images --format '{{.Repository}}:{{.Tag}}' | grep -q "^${IMAGE_NAME}:latest$"; then
    docker rmi ${IMAGE_NAME}:latest -f
    echo "✅ Old image removed"
else
    echo "ℹ️  No existing image found"
fi
echo ""

# 3. Build image baru
echo "🔨 Building new Docker image..."
docker build -t ${IMAGE_NAME}:latest .
echo "✅ Image built successfully"
echo ""

# 4. Run container baru
echo "🚢 Running new container..."
docker run -d \
    -p ${PORT}:${PORT} \
    --name ${CONTAINER_NAME} \
    --restart unless-stopped \
    ${IMAGE_NAME}:latest

echo "✅ Container started successfully"
echo ""

# 5. Check container status
echo "📊 Container status:"
docker ps --filter "name=${CONTAINER_NAME}" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
echo ""

# 6. Show logs
echo "📝 Container logs (last 10 lines):"
sleep 2  # Wait for container to start
docker logs --tail 10 ${CONTAINER_NAME}
echo ""

echo "🎉 Done! Application is running at http://localhost:${PORT}"
echo ""
echo "Useful commands:"
echo "  View logs:     docker logs -f ${CONTAINER_NAME}"
echo "  Stop:          docker stop ${CONTAINER_NAME}"
echo "  Restart:       docker restart ${CONTAINER_NAME}"
echo "  Remove:        docker rm -f ${CONTAINER_NAME}"
