package test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"mcp-xlsm-server/internal/server"
	"mcp-xlsm-server/pkg/config"
)

func TestAnalyzeFile(t *testing.T) {
	// Setup test server
	cfg := config.DefaultConfig()
	srv, err := server.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Create test request
	reqBody := map[string]interface{}{
		"method": "analyze_file",
		"params": map[string]interface{}{
			"filepath":   "test_data/sample.xlsm",
			"chunk_size": 10,
		},
		"id": "test-1",
	}

	reqData, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/", bytes.NewReader(reqData))
	req.Header.Set("Content-Type", "application/json")

	// Record response
	rr := httptest.NewRecorder()
	srv.HandleMCPRequest(rr, req)

	// Check response
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify response structure
	if _, exists := response["result"]; !exists {
		t.Error("Response missing 'result' field")
	}

	if response["id"] != "test-1" {
		t.Error("Response ID mismatch")
	}
}

func TestBuildNavigationMap(t *testing.T) {
	cfg := config.DefaultConfig()
	srv, err := server.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	reqBody := map[string]interface{}{
		"method": "build_navigation_map",
		"params": map[string]interface{}{
			"filepath":    "test_data/sample.xlsm",
			"checksum":    "test_checksum_123",
			"window_size": 1000,
		},
		"id": "test-2",
	}

	reqData, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/", bytes.NewReader(reqData))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	srv.HandleMCPRequest(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify navigation index structure
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatal("Result is not a map")
	}

	if _, exists := result["navigation_index"]; !exists {
		t.Error("Result missing 'navigation_index' field")
	}

	if _, exists := result["token_tracking"]; !exists {
		t.Error("Result missing 'token_tracking' field")
	}
}

func TestQueryData(t *testing.T) {
	cfg := config.DefaultConfig()
	srv, err := server.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Mock navigation index
	navIndex := map[string]interface{}{
		"sheet_index": []map[string]interface{}{
			{
				"sheet_id": "sheet_0",
				"name":     "Sheet1",
				"metadata": map[string]interface{}{
					"rows": 1000,
					"cols": 26,
				},
			},
		},
	}

	reqBody := map[string]interface{}{
		"method": "query_data",
		"params": map[string]interface{}{
			"query":            "test query",
			"navigation_index": navIndex,
			"window_config": map[string]interface{}{
				"max_results": 50,
			},
			"token_aware": true,
		},
		"id": "test-3",
	}

	reqData, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/", bytes.NewReader(reqData))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	srv.HandleMCPRequest(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify query response structure
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatal("Result is not a map")
	}

	if _, exists := result["query_execution"]; !exists {
		t.Error("Result missing 'query_execution' field")
	}

	if _, exists := result["results"]; !exists {
		t.Error("Result missing 'results' field")
	}

	if _, exists := result["adaptive_response"]; !exists {
		t.Error("Result missing 'adaptive_response' field")
	}
}

func TestListTools(t *testing.T) {
	cfg := config.DefaultConfig()
	srv, err := server.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	reqBody := map[string]interface{}{
		"method": "list_tools",
		"params": map[string]interface{}{},
		"id":     "test-4",
	}

	reqData, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/", bytes.NewReader(reqData))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	srv.HandleMCPRequest(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatal("Result is not a map")
	}

	tools, ok := result["tools"].([]interface{})
	if !ok {
		t.Fatal("Tools is not an array")
	}

	if len(tools) != 3 {
		t.Errorf("Expected 3 tools, got %d", len(tools))
	}

	// Verify tool names
	expectedTools := []string{"analyze_file", "build_navigation_map", "query_data"}
	for i, tool := range tools {
		toolMap, ok := tool.(map[string]interface{})
		if !ok {
			t.Fatalf("Tool %d is not a map", i)
		}

		name, ok := toolMap["name"].(string)
		if !ok {
			t.Fatalf("Tool %d missing name", i)
		}

		if name != expectedTools[i] {
			t.Errorf("Tool %d: expected %s, got %s", i, expectedTools[i], name)
		}
	}
}

func TestHealthEndpoint(t *testing.T) {
	cfg := config.DefaultConfig()
	srv, err := server.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()

	srv.HandleHealth(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var health map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&health); err != nil {
		t.Fatalf("Failed to decode health response: %v", err)
	}

	if status, ok := health["status"].(string); !ok || status != "healthy" {
		t.Error("Health status is not 'healthy'")
	}

	if _, exists := health["cache"]; !exists {
		t.Error("Health response missing cache info")
	}
}

func TestMetricsEndpoint(t *testing.T) {
	cfg := config.DefaultConfig()
	srv, err := server.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	req := httptest.NewRequest("GET", "/metrics", nil)
	rr := httptest.NewRecorder()

	srv.HandleMetrics(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var metrics map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&metrics); err != nil {
		t.Fatalf("Failed to decode metrics response: %v", err)
	}

	expectedFields := []string{"cache_stats", "cache_hit_ratio", "memory_usage", "timestamp"}
	for _, field := range expectedFields {
		if _, exists := metrics[field]; !exists {
			t.Errorf("Metrics missing field: %s", field)
		}
	}
}

func TestErrorHandling(t *testing.T) {
	cfg := config.DefaultConfig()
	srv, err := server.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Test invalid JSON
	req := httptest.NewRequest("POST", "/", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	srv.HandleMCPRequest(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 (MCP error), got %d", rr.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode error response: %v", err)
	}

	if _, exists := response["error"]; !exists {
		t.Error("Error response missing 'error' field")
	}

	// Test unknown method
	reqBody := map[string]interface{}{
		"method": "unknown_method",
		"params": map[string]interface{}{},
		"id":     "test-error",
	}

	reqData, _ := json.Marshal(reqBody)
	req = httptest.NewRequest("POST", "/", bytes.NewReader(reqData))
	req.Header.Set("Content-Type", "application/json")

	rr = httptest.NewRecorder()
	srv.HandleMCPRequest(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 (MCP error), got %d", rr.Code)
	}

	response = map[string]interface{}{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode error response: %v", err)
	}

	if _, exists := response["error"]; !exists {
		t.Error("Error response missing 'error' field")
	}
}

func TestConcurrentRequests(t *testing.T) {
	cfg := config.DefaultConfig()
	srv, err := server.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Start test server
	testServer := httptest.NewServer(http.HandlerFunc(srv.HandleMCPRequest))
	defer testServer.Close()

	// Make concurrent requests
	concurrency := 5
	results := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(id int) {
			reqBody := map[string]interface{}{
				"method": "list_tools",
				"params": map[string]interface{}{},
				"id":     id,
			}

			reqData, _ := json.Marshal(reqBody)
			resp, err := http.Post(testServer.URL, "application/json", bytes.NewReader(reqData))
			if err != nil {
				results <- err
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				results <- fmt.Errorf("request %d failed with status %d", id, resp.StatusCode)
				return
			}

			results <- nil
		}(i)
	}

	// Wait for all requests to complete
	timeout := time.After(10 * time.Second)
	for i := 0; i < concurrency; i++ {
		select {
		case err := <-results:
			if err != nil {
				t.Errorf("Concurrent request failed: %v", err)
			}
		case <-timeout:
			t.Fatal("Timeout waiting for concurrent requests")
		}
	}
}

// Benchmark tests
func BenchmarkAnalyzeFile(b *testing.B) {
	cfg := config.DefaultConfig()
	srv, err := server.New(cfg)
	if err != nil {
		b.Fatalf("Failed to create server: %v", err)
	}

	reqBody := map[string]interface{}{
		"method": "analyze_file",
		"params": map[string]interface{}{
			"filepath": "test_data/sample.xlsm",
		},
		"id": "bench",
	}

	reqData, _ := json.Marshal(reqBody)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/", bytes.NewReader(reqData))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		srv.HandleMCPRequest(rr, req)

		if rr.Code != http.StatusOK {
			b.Errorf("Request failed with status %d", rr.Code)
		}
	}
}

func BenchmarkTokenCounting(b *testing.B) {
	data := map[string]interface{}{
		"test":  "value",
		"array": []string{"item1", "item2", "item3"},
		"nested": map[string]interface{}{
			"deep": "value",
		},
	}

	// This would need to be updated to use the actual token counter
	// tokenCounter, _ := token.NewCounter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// tokenCounter.Count(data)
		_ = data // Placeholder
	}
}