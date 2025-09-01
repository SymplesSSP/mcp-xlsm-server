#!/bin/bash

# Ensure we're in the right directory
cd /Volumes/SSD/MCP/mcp-xlsm-server

# Run the server with proper signal handling
exec ./mcp-xlsm-server --stdio --config /Volumes/SSD/MCP/mcp-xlsm-server/config-stdio.yaml