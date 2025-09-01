package compression

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"

	"github.com/andybalholm/brotli"

	"mcp-xlsm-server/internal/token"
)

type Manager struct {
	tokenCounter *token.Counter
}

func NewManager(tokenCounter *token.Counter) *Manager {
	return &Manager{
		tokenCounter: tokenCounter,
	}
}

func (cm *Manager) OptimizeResponse(data interface{}, limit int) ([]byte, string, error) {
	tokens, err := cm.tokenCounter.Count(data)
	if err != nil {
		return nil, "", fmt.Errorf("failed to count tokens: %w", err)
	}

	ratio := float64(tokens) / float64(limit)

	var result []byte
	var method string

	switch {
	case ratio < 0.5:
		// No compression needed
		result, err = json.Marshal(data)
		if err != nil {
			return nil, "", fmt.Errorf("failed to marshal data: %w", err)
		}
		method = "none"

	case ratio < 0.8:
		// Light compression with GZIP
		result, err = cm.compressGzip(data)
		if err != nil {
			return nil, "", fmt.Errorf("failed to compress with gzip: %w", err)
		}
		method = "gzip"

	case ratio < 1.0:
		// Medium compression with Brotli level 4
		result, err = cm.compressBrotli(data, 4)
		if err != nil {
			return nil, "", fmt.Errorf("failed to compress with brotli-4: %w", err)
		}
		method = "brotli-4"

	default:
		// Aggressive compression + truncation
		truncated := cm.truncateData(data, int(float64(limit)*0.7))
		result, err = cm.compressBrotli(truncated, 11)
		if err != nil {
			return nil, "", fmt.Errorf("failed to compress with brotli-11: %w", err)
		}
		method = "brotli-11-truncated"
	}

	return result, method, nil
}

func (cm *Manager) compressGzip(data interface{}) ([]byte, error) {
	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	
	encoder := json.NewEncoder(gzWriter)
	if err := encoder.Encode(data); err != nil {
		gzWriter.Close()
		return nil, err
	}
	
	if err := gzWriter.Close(); err != nil {
		return nil, err
	}
	
	return buf.Bytes(), nil
}

func (cm *Manager) compressBrotli(data interface{}, level int) ([]byte, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	
	var buf bytes.Buffer
	writer := brotli.NewWriterLevel(&buf, level)
	
	if _, err := writer.Write(jsonData); err != nil {
		writer.Close()
		return nil, err
	}
	
	if err := writer.Close(); err != nil {
		return nil, err
	}
	
	return buf.Bytes(), nil
}

func (cm *Manager) truncateData(data interface{}, targetTokens int) interface{} {
	// Implement intelligent truncation based on data type
	switch v := data.(type) {
	case map[string]interface{}:
		return cm.truncateMap(v, targetTokens)
	case []interface{}:
		return cm.truncateSlice(v, targetTokens)
	case string:
		return cm.truncateString(v, targetTokens)
	default:
		return data
	}
}

func (cm *Manager) truncateMap(data map[string]interface{}, targetTokens int) map[string]interface{} {
	result := make(map[string]interface{})
	currentTokens := 0
	
	// Priority order for keeping fields
	priorityFields := []string{"metadata", "summary", "index", "pagination"}
	
	// Add priority fields first
	for _, field := range priorityFields {
		if value, exists := data[field]; exists {
			fieldTokens, err := cm.tokenCounter.Count(value)
			if err == nil && currentTokens+fieldTokens <= targetTokens {
				result[field] = value
				currentTokens += fieldTokens
			}
		}
	}
	
	// Add remaining fields until token limit
	for key, value := range data {
		if _, exists := result[key]; exists {
			continue // Already added
		}
		
		fieldTokens, err := cm.tokenCounter.Count(value)
		if err != nil {
			continue
		}
		
		if currentTokens+fieldTokens <= targetTokens {
			result[key] = value
			currentTokens += fieldTokens
		} else {
			// Try to truncate the value
			truncatedValue := cm.truncateData(value, targetTokens-currentTokens)
			truncatedTokens, err := cm.tokenCounter.Count(truncatedValue)
			if err == nil && truncatedTokens > 0 {
				result[key] = truncatedValue
				currentTokens += truncatedTokens
			}
			break
		}
	}
	
	// Add truncation marker
	result["_truncated"] = true
	result["_original_size"] = len(data)
	result["_included_fields"] = len(result) - 2
	
	return result
}

func (cm *Manager) truncateSlice(data []interface{}, targetTokens int) []interface{} {
	var result []interface{}
	currentTokens := 0
	
	for i, item := range data {
		itemTokens, err := cm.tokenCounter.Count(item)
		if err != nil {
			continue
		}
		
		if currentTokens+itemTokens <= targetTokens {
			result = append(result, item)
			currentTokens += itemTokens
		} else {
			// Try to truncate the item
			truncatedItem := cm.truncateData(item, targetTokens-currentTokens)
			truncatedTokens, err := cm.tokenCounter.Count(truncatedItem)
			if err == nil && truncatedTokens > 0 {
				result = append(result, truncatedItem)
			}
			
			// Add summary of remaining items
			if i < len(data)-1 {
				summary := map[string]interface{}{
					"_truncated_items": len(data) - i - 1,
					"_total_items":     len(data),
				}
				result = append(result, summary)
			}
			break
		}
	}
	
	return result
}

func (cm *Manager) truncateString(data string, targetTokens int) string {
	tokens := cm.tokenCounter.CountString(data)
	
	if tokens <= targetTokens {
		return data
	}
	
	// Rough estimation: average 4 characters per token
	targetLength := targetTokens * 4
	if targetLength >= len(data) {
		return data
	}
	
	// Try to cut at word boundaries
	truncated := data[:targetLength]
	if lastSpace := bytes.LastIndexByte([]byte(truncated), ' '); lastSpace > targetLength/2 {
		truncated = truncated[:lastSpace]
	}
	
	return truncated + "... [truncated]"
}

// Decompression methods
func (cm *Manager) Decompress(data []byte, method string) ([]byte, error) {
	switch method {
	case "none":
		return data, nil
	case "gzip":
		return cm.decompressGzip(data)
	case "brotli-4", "brotli-11", "brotli-11-truncated":
		return cm.decompressBrotli(data)
	default:
		return nil, fmt.Errorf("unknown compression method: %s", method)
	}
}

func (cm *Manager) decompressGzip(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(reader); err != nil {
		return nil, err
	}
	
	return buf.Bytes(), nil
}

func (cm *Manager) decompressBrotli(data []byte) ([]byte, error) {
	reader := brotli.NewReader(bytes.NewReader(data))
	
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(reader); err != nil {
		return nil, err
	}
	
	return buf.Bytes(), nil
}

// Estimation methods
func (cm *Manager) EstimateCompressionRatio(data interface{}, method string) (float64, error) {
	_, err := cm.tokenCounter.Count(data)
	if err != nil {
		return 0, err
	}
	
	switch method {
	case "gzip":
		return 0.7, nil
	case "brotli-4":
		return 0.6, nil
	case "brotli-11":
		return 0.5, nil
	default:
		return 1.0, nil
	}
}

func (cm *Manager) SuggestCompressionMethod(data interface{}, tokenLimit int) (string, error) {
	tokens, err := cm.tokenCounter.Count(data)
	if err != nil {
		return "", err
	}
	
	ratio := float64(tokens) / float64(tokenLimit)
	
	switch {
	case ratio < 0.5:
		return "none", nil
	case ratio < 0.8:
		return "gzip", nil
	case ratio < 1.0:
		return "brotli-4", nil
	default:
		return "brotli-11-truncated", nil
	}
}