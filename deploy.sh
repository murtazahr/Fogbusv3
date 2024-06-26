#!/bin/bash

# Build the Docker image
docker build -t Fogbusv3 .

# Stop and remove any existing container
docker stop Fogbusv3 || true
docker rm Fogbusv3 || true

# Run the new container
docker run -d --name Fogbusv3 \
  -p 8080:8080 \
  -v $(pwd)/config.yaml:/app/config.yaml \
  -v $(pwd)/connection-profile.yaml:/app/connection-profile.yaml \
  fog-computing-node $@