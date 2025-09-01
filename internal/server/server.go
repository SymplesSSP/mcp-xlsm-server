package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"go.uber.org/zap"

	"mcp-xlsm-server/internal/cache"
	"mcp-xlsm-server/internal/compression"
	"mcp-xlsm-server/pkg/config"
)

type Server struct {
	config      *config.Config
	logger      *zap.Logger
	toolHandler *ToolHandler
	cache       *cache.SmartCache
	compressor  *compression.Manager
	httpServer  *http.Server
}

type MCPRequest struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
	ID     interface{}            `json:"id"`
}

type MCPResponse struct {
	Result interface{} `json:"result,omitempty"`
	Error  *MCPError   `json:"error,omitempty"`
	ID     interface{} `json:"id"`
}

type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func New(cfg *config.Config) (*Server, error) {
	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	// Initialize tool handler
	toolHandler, err := NewToolHandler()
	if err != nil {
		return nil, fmt.Errorf("failed to create tool handler: %w", err)
	}

	// Initialize cache
	cacheSize := int64(100) // 100MB default
	smartCache, err := cache.NewSmartCache(cacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	// Initialize compression manager
	compressor := compression.NewManager(toolHandler.tokenCounter)

	// Create HTTP server
	mux := http.NewServeMux()
	server := &Server{
		config:      cfg,
		logger:      logger,
		toolHandler: toolHandler,
		cache:       smartCache,
		compressor:  compressor,
	}

	// Setup routes
	mux.HandleFunc("/", server.handleMCPRequest)
	mux.HandleFunc("/health", server.handleHealth)
	mux.HandleFunc("/metrics", server.handleMetrics)

	server.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      mux,
		ReadTimeout:  cfg.Server.RequestTimeout,
		WriteTimeout: cfg.Server.RequestTimeout,
	}

	return server, nil
}

func (s *Server) Start(ctx context.Context) error {
	s.logger.Info("Starting MCP XLSM server",
		zap.String("address", s.httpServer.Addr),
		zap.String("version", "2.0.0"),
	)

	// Start background services
	go s.startBackgroundServices(ctx)

	return s.httpServer.ListenAndServe()
}

func (s *Server) StartStdio(ctx context.Context) error {
	// In stdio mode, we don't log to stdout to avoid interfering with MCP communication
	// Log to stderr instead
	s.logger = s.logger.With(zap.String("mode", "stdio"))
	
	// Start background services
	go s.startBackgroundServices(ctx)
	
	// Create stdin reader
	scanner := bufio.NewScanner(os.Stdin)
	
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		line := scanner.Text()
		if line == "" {
			continue
		}
		
		// Parse MCP request
		var mcpReq MCPRequest
		if err := json.Unmarshal([]byte(line), &mcpReq); err != nil {
			s.sendStdioError(mcpReq.ID, -32700, "Parse error")
			continue
		}
		
		// Log request to stderr in stdio mode
		fmt.Fprintf(os.Stderr, "Handling MCP request: method=%s id=%v\n", mcpReq.Method, mcpReq.ID)
		
		// Route to appropriate handler
		result, err := s.routeRequest(ctx, &mcpReq)
		
		// Send response
		response := MCPResponse{
			ID:     mcpReq.ID,
			Result: result,
		}
		
		if err != nil {
			response.Error = &MCPError{
				Code:    -32000,
				Message: err.Error(),
			}
			response.Result = nil
		}
		
		// Send response to stdout
		if jsonResp, marshalErr := json.Marshal(response); marshalErr == nil {
			fmt.Println(string(jsonResp))
		} else {
			s.sendStdioError(mcpReq.ID, -32603, "Internal error")
		}
	}
	
	return scanner.Err()
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down server")
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) sendStdioError(id interface{}, code int, message string) {
	response := MCPResponse{
		Error: &MCPError{
			Code:    code,
			Message: message,
		},
		ID: id,
	}
	
	if jsonResp, err := json.Marshal(response); err == nil {
		fmt.Println(string(jsonResp))
	}
}

func (s *Server) handleMCPRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse MCP request
	var mcpReq MCPRequest
	if err := json.NewDecoder(r.Body).Decode(&mcpReq); err != nil {
		s.sendError(w, mcpReq.ID, -32700, "Parse error")
		return
	}

	// Log request
	s.logger.Info("Handling MCP request",
		zap.String("method", mcpReq.Method),
		zap.Any("id", mcpReq.ID),
	)

	// Route to appropriate handler
	result, err := s.routeRequest(r.Context(), &mcpReq)
	if err != nil {
		s.logger.Error("Request failed",
			zap.String("method", mcpReq.Method),
			zap.Error(err),
		)
		s.sendError(w, mcpReq.ID, -32603, err.Error())
		return
	}

	// Send response
	response := MCPResponse{
		Result: result,
		ID:     mcpReq.ID,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("Failed to encode response", zap.Error(err))
	}
}

func (s *Server) routeRequest(ctx context.Context, req *MCPRequest) (interface{}, error) {
	switch req.Method {
	case "analyze_file":
		return s.toolHandler.AnalyzeFile(ctx, req.Params)

	case "build_navigation_map":
		return s.toolHandler.BuildNavigationMap(ctx, req.Params)

	case "query_data":
		return s.toolHandler.QueryData(ctx, req.Params)

	case "list_tools":
		return s.listTools(), nil

	case "get_server_info":
		return s.getServerInfo(), nil

	case "initialize":
		return s.initialize(req.Params), nil
	default:
		return nil, fmt.Errorf("unknown method: %s", req.Method)
	}
}

func (s *Server) initialize(params map[string]interface{}) interface{} {
	return map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		"serverInfo": map[string]interface{}{
			"name":    "mcp-xlsm-server",
			"version": "2.0.0",
		},
	}
}

func (s *Server) listTools() interface{} {
	return map[string]interface{}{
		"tools": []map[string]interface{}{
			{
				"name":        "analyze_file",
				"description": "Analyze XLSM file metadata and structure with automatic chunking",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"filepath": map[string]interface{}{
							"type":        "string",
							"description": "Path to the XLSM file",
						},
						"chunk_size": map[string]interface{}{
							"type":        "integer",
							"description": "Number of sheets per chunk (default: 50)",
							"default":     50,
						},
						"stream_mode": map[string]interface{}{
							"type":        "boolean",
							"description": "Enable streaming for large files (default: true for >100MB)",
						},
					},
					"required": []string{"filepath"},
				},
			},
			{
				"name":        "build_navigation_map",
				"description": "Build navigable index with pagination and streaming",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"filepath": map[string]interface{}{
							"type":        "string",
							"description": "Path to the XLSM file",
						},
						"checksum": map[string]interface{}{
							"type":        "string",
							"description": "File checksum for validation",
						},
						"chunk_cursor": map[string]interface{}{
							"type":        "string",
							"description": "Base64 encoded cursor for pagination",
						},
						"window_size": map[string]interface{}{
							"type":        "integer",
							"description": "Maximum sheets per call (default: 1000)",
							"default":     1000,
						},
					},
					"required": []string{"filepath", "checksum"},
				},
			},
			{
				"name":        "query_data",
				"description": "Query multi-sheet data with windowing and streaming",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{
							"type":        "string",
							"description": "Search query",
						},
						"navigation_index": map[string]interface{}{
							"type":        "object",
							"description": "Navigation index from build_navigation_map",
						},
						"window_config": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"max_rows_per_sheet": map[string]interface{}{
									"type":    "integer",
									"default": 1000,
								},
								"max_sheets_per_call": map[string]interface{}{
									"type":    "integer",
									"default": 10,
								},
								"max_results": map[string]interface{}{
									"type":    "integer",
									"default": 100,
								},
							},
						},
					},
					"required": []string{"query", "navigation_index"},
				},
			},
		},
	}
}

func (s *Server) getServerInfo() interface{} {
	return map[string]interface{}{
		"name":    "MCP XLSM Server",
		"version": "2.0.0",
		"capabilities": map[string]interface{}{
			"streaming":     true,
			"chunking":      true,
			"token_aware":   true,
			"compression":   true,
			"caching":       true,
			"indexing":      true,
			"max_file_size": s.config.Server.MaxFileSize,
		},
		"limits": map[string]interface{}{
			"max_concurrent_requests": s.config.Server.MaxConcurrentReqs,
			"request_timeout":         s.config.Server.RequestTimeout.String(),
		},
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
		"version":   "2.0.0",
		"cache":     s.getCacheHealth(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := map[string]interface{}{
		"cache_stats":     s.cache.GetStats(),
		"cache_hit_ratio": s.cache.GetHitRatio(),
		"memory_usage":    s.getCacheMemoryUsage(),
		"timestamp":       time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

func (s *Server) getCacheHealth() map[string]interface{} {
	used, total := s.cache.GetMemoryUsage()
	return map[string]interface{}{
		"memory_used":  used,
		"memory_total": total,
		"hit_ratio":    s.cache.GetHitRatio(),
	}
}

func (s *Server) getCacheMemoryUsage() map[string]interface{} {
	used, total := s.cache.GetMemoryUsage()
	return map[string]interface{}{
		"used_bytes":  used,
		"total_bytes": total,
		"used_mb":     used / (1024 * 1024),
		"total_mb":    total / (1024 * 1024),
		"utilization": float64(used) / float64(total),
	}
}

func (s *Server) sendError(w http.ResponseWriter, id interface{}, code int, message string) {
	response := MCPResponse{
		Error: &MCPError{
			Code:    code,
			Message: message,
		},
		ID: id,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) // MCP errors are still HTTP 200
	json.NewEncoder(w).Encode(response)
}

func (s *Server) startBackgroundServices(ctx context.Context) {
	// Start cache cleanup
	ticker := time.NewTicker(s.config.Cache.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.performMaintenanceTasks()
		}
	}
}

func (s *Server) performMaintenanceTasks() {
	// Cache cleanup is handled internally by SmartCache
	// Add other maintenance tasks here

	s.logger.Debug("Performed maintenance tasks",
		zap.Float64("cache_hit_ratio", s.cache.GetHitRatio()),
	)
}