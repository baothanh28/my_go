#!/bin/bash

# Configuration
NETWORK_NAME="myapp_network"

echo "Checking for Docker network: ${NETWORK_NAME}..."

# Check if the network already exists
if docker network ls --format '{{.Name}}' | grep -q "^${NETWORK_NAME}$"; then
  echo "✅ Network '${NETWORK_NAME}' already exists."
else
  # Create the external bridge network
  echo "🛠️ Creating network '${NETWORK_NAME}'..."
  if docker network create "${NETWORK_NAME}"; then
    echo "✅ Network '${NETWORK_NAME}' created successfully."
  else
    echo "❌ Error creating network '${NETWORK_NAME}'. Please check your Docker installation and permissions."
    exit 1
  fi
fi

# Note: To use this network, ensure your docker-compose.yml files 
# define it as an external network at the root level:
# networks:
#   myapp_network:
#     external: true
# and that all containers that need to communicate are explicitly attached to it.
