package models

import (
	"time"
)

const (
	CURSOR_VERSION = 1
)

// Core data structures
type CursorData struct {
	ChunkID    string    `json:"chunk_id"`
	Offset     int64     `json:"offset"`
	Version    int       `json:"version"`
	Checksum   string    `json:"checksum"`
	Timestamp  int64     `json:"ts"`
	WindowInfo *Window   `json:"window,omitempty"`
}

type Window struct {
	StartRow int `json:"start_row"`
	EndRow   int `json:"end_row"`
	StartCol int `json:"start_col"`
	EndCol   int `json:"end_col"`
}

type Chunk struct {
	ChunkID          string  `json:"chunk_id"`
	SheetsRange      [2]int  `json:"sheets_range"`
	SizeBytes        int64   `json:"size_bytes"`
	Cursor           string  `json:"cursor"`
	StreamingRequired bool   `json:"streaming_required"`
}

type TokenManagement struct {
	ModelDetected    string           `json:"model_detected"`
	CountingMethod   string           `json:"counting_method"`
	Limits           TokenLimits      `json:"limits"`
	ChunkingStrategy ChunkingStrategy `json:"chunking_strategy"`
}

type TokenLimits struct {
	Context   int `json:"context"`
	SafeBuffer int `json:"safe_buffer"`
	OutputMax int `json:"output_max"`
}

type ChunkingStrategy struct {
	SheetsPerChunk   int `json:"sheets_per_chunk"`
	EstimatedTokens  int `json:"estimated_tokens"`
	ActualTokens     int `json:"actual_tokens"`
}

type FileMetadata struct {
	Checksum         string    `json:"checksum"`
	FileSize         int64     `json:"file_size"`
	SheetsCount      int       `json:"sheets_count"`
	Timestamp        time.Time `json:"timestamp"`
	ComplexityScore  float64   `json:"complexity_score"`
	MemoryEstimate   int64     `json:"memory_estimate"`
}

type PatternsDetected struct {
	NamingPatterns    []string `json:"naming_patterns"`
	DataTypes         map[string]interface{} `json:"data_types"`
	StructuralGroups  int      `json:"structural_groups"`
	FormulaComplexity string   `json:"formula_complexity"`
}

type IndexSummary struct {
	ValueTypes    map[string]interface{} `json:"value_types"`
	FormulaPatterns []string            `json:"formula_patterns"`
	SheetGroups     []string            `json:"sheet_groups"`
	CircularRefs    []string            `json:"circular_refs"`
}

type PerformanceMetrics struct {
	AnalysisTimeMs int64 `json:"analysis_time_ms"`
	MemoryUsedMB   int64 `json:"memory_used_mb"`
}

// Tool 1 Response
type AnalyzeFileResponse struct {
	Metadata         FileMetadata       `json:"metadata"`
	Chunks           []Chunk            `json:"chunks"`
	PatternsDetected PatternsDetected   `json:"patterns_detected"`
	TokenManagement  TokenManagement    `json:"token_management"`
	IndexSummary     IndexSummary       `json:"index_summary"`
	NextCursor       string             `json:"next_cursor"`
	HasMore          bool               `json:"has_more"`
	Performance      PerformanceMetrics `json:"performance_metrics"`
}

// Sheet metadata for navigation
type SheetMetadata struct {
	Rows          int     `json:"rows"`
	Cols          int     `json:"cols"`
	DataDensity   float64 `json:"data_density"`
	HasFormulas   bool    `json:"has_formulas"`
	MemoryFootprint int64 `json:"memory_footprint"`
}

type Zone struct {
	ZoneID      string `json:"zone_id"`
	Range       string `json:"range"`
	WindowSize  int    `json:"window_size"`
	Compressed  bool   `json:"compressed"`
}

type SheetIndex struct {
	SheetID   string        `json:"sheet_id"`
	Name      string        `json:"name"`
	Metadata  SheetMetadata `json:"metadata"`
	Zones     []Zone        `json:"zones"`
	KeyPoints []string      `json:"key_points"`
	HotZones  []string      `json:"hot_zones"`
}

type Connection struct {
	FormulaLinks            []string `json:"formula_links"`
	StructuralSimilarities  []string `json:"structural_similarities"`
	CircularDependencies    []string `json:"circular_dependencies"`
}

type SearchIndex struct {
	BTreeIndex   map[string]interface{} `json:"btree_index"`
	InvertedIndex map[string]interface{} `json:"inverted_index"`
	SpatialIndex  map[string]interface{} `json:"spatial_index"`
	BloomFilter   map[string]interface{} `json:"bloom_filter"`
}

type DeltaTracking struct {
	LastUpdate      time.Time `json:"last_update"`
	ChangedCells    []string  `json:"changed_cells"`
	RebuildRequired bool      `json:"rebuild_required"`
}

type NavigationIndex struct {
	ChecksumMatch        bool          `json:"checksum_match"`
	InvalidationRequired bool          `json:"invalidation_required"`
	ChunkInfo           ChunkInfo     `json:"chunk_info"`
	SheetIndex          []SheetIndex  `json:"sheet_index"`
	Connections         Connection    `json:"connections"`
	SearchIndex         SearchIndex   `json:"search_index"`
	DeltaTracking       DeltaTracking `json:"delta_tracking"`
}

type ChunkInfo struct {
	Current         string   `json:"current"`
	TotalChunks     int      `json:"total_chunks"`
	SheetsInChunk   []string `json:"sheets_in_chunk"`
	StreamingActive bool     `json:"streaming_active"`
}

type TokenTracking struct {
	Used              int    `json:"used"`
	Remaining         int    `json:"remaining"`
	CompressionApplied string `json:"compression_applied"`
	Optimization      string `json:"optimization"`
	ActualCount       int    `json:"actual_count"`
}

type Pagination struct {
	CurrentCursor          string `json:"current_cursor"`
	NextCursor             string `json:"next_cursor"`
	PreviousCursor         string `json:"previous_cursor"`
	RemainingChunks        int    `json:"remaining_chunks"`
	EstimatedTimeRemaining int    `json:"estimated_time_remaining"`
}

type CacheControl struct {
	TTLSeconds          int    `json:"ttl_seconds"`
	InvalidateOnChecksum bool   `json:"invalidate_on_checksum"`
	HotDataExtension     bool   `json:"hot_data_extension"`
	CacheKey             string `json:"cache_key"`
}

// Tool 2 Response
type BuildNavigationResponse struct {
	NavigationIndex NavigationIndex `json:"navigation_index"`
	TokenTracking   TokenTracking   `json:"token_tracking"`
	Pagination      Pagination      `json:"pagination"`
	CacheControl    CacheControl    `json:"cache_control"`
}

// Query execution details
type QueryExecution struct {
	UsedIndex       bool     `json:"used_index"`
	IndexType       string   `json:"index_type"`
	ChunksScanned   []string `json:"chunks_scanned"`
	Strategy        string   `json:"strategy"`
	BloomFilterUsed bool     `json:"bloom_filter_used"`
}

type DataChunk struct {
	Location   string      `json:"location"`
	Window     string      `json:"window"`
	DataChunk  interface{} `json:"data_chunk"`
	Metadata   ChunkMetadata `json:"metadata"`
	Context    Context     `json:"context"`
}

type ChunkMetadata struct {
	Size       int64 `json:"size"`
	Truncated  bool  `json:"truncated"`
	Compressed bool  `json:"compressed"`
}

type Context struct {
	Headers  []string               `json:"headers"`
	Nearby   map[string]interface{} `json:"nearby"`
	Formulas []string               `json:"formulas"`
}

type QueryResults struct {
	Data []DataChunk `json:"data"`
}

type Statistics struct {
	Aggregations        []interface{} `json:"aggregations"`
	Patterns            []interface{} `json:"patterns"`
	Outliers            []interface{} `json:"outliers"`
	FormulaEvaluations  []interface{} `json:"formula_evaluations"`
}

type ModelContext struct {
	Detected     string `json:"detected"`
	Limit        int    `json:"limit"`
	Used         int    `json:"used"`
	PreciseCount int    `json:"precise_count"`
}

type StrategyConfig struct {
	MaxResults  int    `json:"max_results"`
	WindowRows  int    `json:"window_rows"`
	Compression string `json:"compression"`
}

type AdaptiveResponse struct {
	ModelContext    ModelContext   `json:"model_context"`
	IfSonnetBeta    StrategyConfig `json:"if_sonnet_beta"`
	IfStandard      StrategyConfig `json:"if_standard"`
	IfOpus          StrategyConfig `json:"if_opus"`
}

type IndexUpdates struct {
	NewPatterns      []string `json:"new_patterns"`
	SuggestedChunks  []string `json:"suggested_chunks"`
	DeltaApplied     bool     `json:"delta_applied"`
}

type QueryPerformance struct {
	QueryTimeMs      int64 `json:"query_time_ms"`
	IndexTimeMs      int64 `json:"index_time_ms"`
	TokenCountTimeMs int64 `json:"token_count_time_ms"`
}

// Tool 3 Response
type QueryDataResponse struct {
	QueryExecution   QueryExecution   `json:"query_execution"`
	Results          QueryResults     `json:"results"`
	Statistics       Statistics       `json:"statistics"`
	AdaptiveResponse AdaptiveResponse `json:"adaptive_response"`
	Pagination       Pagination       `json:"pagination"`
	IndexUpdates     IndexUpdates     `json:"index_updates"`
	Performance      QueryPerformance `json:"performance"`
}

// Delta tracking for incremental updates
type DeltaType string

const (
	CellUpdate    DeltaType = "cell_update"
	SheetAdd      DeltaType = "sheet_add"
	FormulaChange DeltaType = "formula_change"
	BulkChange    DeltaType = "bulk_change"
)

type Delta struct {
	Type         DeltaType   `json:"type"`
	SheetID      string      `json:"sheet_id"`
	Location     string      `json:"location"`
	OldValue     interface{} `json:"old_value"`
	NewValue     interface{} `json:"new_value"`
	AffectedCells int        `json:"affected_cells"`
}

// Hot cache entry for performance tracking
type HotEntry struct {
	AccessCount int           `json:"access_count"`
	LastAccess  time.Time     `json:"last_access"`
	TTL         time.Duration `json:"ttl"`
	Size        int64         `json:"size"`
}