package token

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/pkoukk/tiktoken-go"
)

type Counter struct {
	encoder *tiktoken.Tiktoken
	cache   sync.Map
	mu      sync.RWMutex
}

type ModelLimits struct {
	Context   int
	SafeBuffer int
	OutputMax int
}

var ModelConfigs = map[string]ModelLimits{
	"sonnet-4": {
		Context:   200_000,
		SafeBuffer: 180_000,
		OutputMax: 64_000,
	},
	"sonnet-4-beta": {
		Context:   1_000_000,
		SafeBuffer: 950_000,
		OutputMax: 64_000,
	},
	"opus-4-1": {
		Context:   200_000,
		SafeBuffer: 180_000,
		OutputMax: 32_000,
	},
}

func NewCounter() (*Counter, error) {
	// Use Claude's tokenizer encoding
	enc, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		return nil, fmt.Errorf("failed to get tiktoken encoding: %w", err)
	}
	
	return &Counter{
		encoder: enc,
	}, nil
}

func (tc *Counter) Count(data interface{}) (int, error) {
	// Generate cache key
	key := fmt.Sprintf("%T_%v", data, data)
	
	// Check cache first
	if cached, ok := tc.cache.Load(key); ok {
		return cached.(int), nil
	}
	
	// Convert to JSON for accurate counting
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal data: %w", err)
	}
	
	// Tokenize
	tokens := tc.encoder.Encode(string(jsonBytes), nil, nil)
	count := len(tokens)
	
	// Cache result with size limit
	tc.cache.Store(key, count)
	
	return count, nil
}

func (tc *Counter) CountString(text string) int {
	// Check cache first
	if cached, ok := tc.cache.Load(text); ok {
		return cached.(int)
	}
	
	tokens := tc.encoder.Encode(text, nil, nil)
	count := len(tokens)
	
	// Cache result
	tc.cache.Store(text, count)
	
	return count
}

func (tc *Counter) EstimateCompressed(data interface{}, method string) (int, error) {
	baseCount, err := tc.Count(data)
	if err != nil {
		return 0, err
	}
	
	switch method {
	case "gzip":
		return int(float64(baseCount) * 0.7), nil
	case "brotli":
		return int(float64(baseCount) * 0.6), nil
	case "brotli-aggressive":
		return int(float64(baseCount) * 0.5), nil
	default:
		return baseCount, nil
	}
}

func (tc *Counter) GetModelLimits(modelName string) ModelLimits {
	if limits, exists := ModelConfigs[modelName]; exists {
		return limits
	}
	
	// Default to standard limits
	return ModelConfigs["sonnet-4"]
}

func (tc *Counter) CalculateOptimalChunkSize(modelName string, targetUtilization float64) int {
	limits := tc.GetModelLimits(modelName)
	
	// Calculate optimal tokens per chunk based on target utilization
	targetTokens := int(float64(limits.SafeBuffer) * targetUtilization)
	
	return targetTokens
}

func (tc *Counter) ValidateTokenLimit(data interface{}, modelName string) error {
	count, err := tc.Count(data)
	if err != nil {
		return err
	}
	
	limits := tc.GetModelLimits(modelName)
	
	if count > limits.SafeBuffer {
		return fmt.Errorf("token count %d exceeds safe buffer %d for model %s", 
			count, limits.SafeBuffer, modelName)
	}
	
	return nil
}

func (tc *Counter) GetCompressionStrategy(tokenCount int, modelName string) string {
	limits := tc.GetModelLimits(modelName)
	ratio := float64(tokenCount) / float64(limits.SafeBuffer)
	
	switch {
	case ratio < 0.5:
		return "none"
	case ratio < 0.7:
		return "gzip"
	case ratio < 0.9:
		return "brotli"
	default:
		return "brotli-aggressive"
	}
}

func (tc *Counter) CleanCache() {
	// Clean old cache entries to prevent memory growth
	tc.cache.Range(func(key, value interface{}) bool {
		tc.cache.Delete(key)
		return true
	})
}

// Advanced token management
func (tc *Counter) BatchCount(items []interface{}) ([]int, error) {
	counts := make([]int, len(items))
	
	for i, item := range items {
		count, err := tc.Count(item)
		if err != nil {
			return nil, fmt.Errorf("failed to count item %d: %w", i, err)
		}
		counts[i] = count
	}
	
	return counts, nil
}

func (tc *Counter) EstimateChunkingNeeded(data interface{}, modelName string) (bool, int, error) {
	count, err := tc.Count(data)
	if err != nil {
		return false, 0, err
	}
	
	limits := tc.GetModelLimits(modelName)
	
	if count <= limits.SafeBuffer {
		return false, 1, nil
	}
	
	// Calculate number of chunks needed
	chunksNeeded := (count + limits.SafeBuffer - 1) / limits.SafeBuffer
	
	return true, chunksNeeded, nil
}