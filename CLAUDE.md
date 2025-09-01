# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a **MCP (Model Context Protocol) XLSM Server v2.0** - a universal server for analyzing complex Excel/XLSM files with 200+ sheets and up to 500MB in size. The server provides 3 main MCP tools for file analysis, navigation mapping, and data querying with intelligent chunking, token management, and performance optimization.

**Critical: The server supports both HTTP and stdio modes for different integration scenarios.**

## Development Commands

### Build and Run
```bash
# Build the server
make build

# Run in HTTP mode (default)
make run-dev

# Run in stdio mode for MCP integration
./mcp-xlsm-server --stdio --config config.yaml

# Hot reload development
make dev-watch
```

### Testing
```bash
# Run all tests
make test

# Run tests with coverage
make coverage

# Run integration tests
make test-integration

# Run benchmarks
make bench
```

### Code Quality
```bash
# Format code
make fmt

# Run linter
make lint

# Download/update dependencies
make deps
```

## Core Architecture

### Dual-Mode Server Architecture
The server (`cmd/main.go`) operates in two modes:
- **HTTP mode** (default): Standard REST API server on port 3001
- **stdio mode** (`--stdio` flag): MCP-compatible stdin/stdout communication for Claude Code integration

### MCP Protocol Implementation
The server implements the complete MCP protocol:
- **`initialize`** - Required MCP handshake method
- **`list_tools`** - Returns available tools
- **`analyze_file`** (`internal/server/tools.go`) - File metadata analysis with automatic chunking
- **`build_navigation_map`** (`internal/server/navigation.go`) - Index building with pagination  
- **`query_data`** (`internal/server/query.go`) - Multi-sheet data querying with windowing

### Key Components
- **`internal/models/types.go`** - All data structures and response types for the MCP protocol
- **`internal/cursor/cursor.go`** - Opaque cursor management for pagination (Base64 encoded)
- **`internal/token/counter.go`** - Precise token counting using tiktoken-go for different models
- **`internal/cache/smart_cache.go`** - LRU cache with hot data tracking and automatic cleanup
- **`internal/compression/manager.go`** - Adaptive compression (gzip/brotli) based on token constraints
- **`internal/index/manager.go`** - Multi-level indexing (BTree, Inverted, Spatial, Bloom Filter)

### Server Configuration
- **Configuration**: `config.yaml` - contains all server, performance, cache, and monitoring settings
- **Config loading**: Uses `pkg/config/config.go` with `LoadFromPath()` for custom config files
- **HTTP mode**: Port 3001, health endpoint `/health`, metrics endpoint `/metrics`
- **stdio mode**: No HTTP server, communicates via stdin/stdout, logs to stderr

### Excel File Processing
- Uses **Excelize v2** library for XLSM file manipulation
- Supports streaming for files >10MB (`performance.stream_threshold`)
- Smart chunking based on model token limits (auto-detects Sonnet, Claude, GPT models)
- Memory limits per tool configurable in `config.yaml`

## Integration with Claude Code

### MCP Server Installation
The server integrates with Claude Code using stdio mode:

```bash
# Manual integration
claude mcp add mcp-xlsm /path/to/mcp-xlsm-server --scope user -- --stdio --config /path/to/config.yaml

# Or use the automated script
./install-mcp.sh
```

**Important**: The server MUST be run with `--stdio` flag for Claude Code compatibility. The HTTP mode is for standalone testing and monitoring only.

### Available Test Data
- **Test file**: `/Volumes/SSD/MCP/COMBINE INTERACTIF 02 2025.xlsm`
- **244 sheets**, 26.3MB file with complex financial data
- **Pre-analyzed sheets**: FROUDIS (sheet 86), CHAMDIS (sheet 81)

## Performance Targets

- **Analyze 244 sheets**: < 3s (target), < 5s (max acceptable)
- **Build navigation index**: < 7s (target), < 10s (max acceptable)  
- **Query with index**: < 300ms (target), < 500ms (max acceptable)
- **Window 1000 rows**: < 500ms (target), < 1s (max acceptable)

## Dependencies and Libraries

### Core Dependencies (go.mod)
- **github.com/xuri/excelize/v2** - Excel/XLSM file manipulation
- **github.com/pkoukk/tiktoken-go** - Precise token counting for different LLM models
- **go.uber.org/zap** - Structured logging
- **github.com/andybalholm/brotli** - Brotli compression
- **github.com/google/btree** - BTree indexing
- **github.com/bits-and-blooms/bloom/v3** - Bloom filters for efficient lookups
- **github.com/hashicorp/golang-lru** - LRU cache implementation
- **gopkg.in/yaml.v3** - YAML configuration parsing

### Development Tools
- **golangci-lint** - Code linting
- **air** - Hot reload development server
- **gosec** - Security scanning

## Token Management Strategy

The server implements intelligent token management:
- **Auto-detects model type** (Sonnet-4, Claude, GPT) from request context
- **Adaptive windowing** - different limits per model type
- **Compression strategies** - light/medium/aggressive based on token pressure  
- **Smart chunking** - balances between token limits and processing efficiency

## Data Structures

Key types are defined in `internal/models/types.go`:
- **CursorData** - Opaque pagination cursors with versioning
- **AnalyzeFileResponse** - Complete file analysis with metadata, chunks, patterns
- **NavigationIndex** - Multi-level index structure for efficient sheet navigation
- **QueryDataResponse** - Search results with performance metrics and adaptive response

## Configuration Management

All configuration is centralized in `config.yaml`:
- **Server settings** - host, port, timeouts, file size limits
- **Performance tuning** - worker pools, buffer sizes, streaming thresholds
- **Rate limiting** - per-tool rate limits and memory constraints
- **Cache configuration** - memory limits, TTL, cleanup intervals
- **Monitoring setup** - Prometheus metrics, tracing, logging levels

## Critical Implementation Notes

### stdio Mode Requirements
- **No stdout logging** in stdio mode to avoid interfering with MCP communication
- **stderr logging only** for debug information
- **JSON-RPC protocol** over stdin/stdout
- **MCP initialize handshake** required before tool calls

### Command Line Flags
- `--stdio`: Enable stdio mode for MCP integration
- `--config <path>`: Specify custom configuration file path
- No flags: Default HTTP mode on configured port

### Request Routing
The `routeRequest()` method in `internal/server/server.go` handles both HTTP and stdio requests, with special handling for the MCP `initialize` method required for stdio mode compatibility.