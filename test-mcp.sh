#!/bin/bash

# Test MCP server with multiple requests
(
echo '{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {"protocolVersion": "2024-11-05", "capabilities": {}}}'
sleep 1
echo '{"jsonrpc": "2.0", "id": 2, "method": "list_tools", "params": {}}'
sleep 5
) | ./mcp-xlsm-server --stdio --config config-stdio.yaml