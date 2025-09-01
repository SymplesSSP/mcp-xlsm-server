package server

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"mcp-xlsm-server/internal/cursor"
	"mcp-xlsm-server/internal/models"
	"mcp-xlsm-server/internal/token"
)

type ToolHandler struct {
	cursorManager *cursor.Manager
	tokenCounter  *token.Counter
}

func NewToolHandler() (*ToolHandler, error) {
	tokenCounter, err := token.NewCounter()
	if err != nil {
		return nil, fmt.Errorf("failed to create token counter: %w", err)
	}

	return &ToolHandler{
		cursorManager: cursor.NewManager(),
		tokenCounter:  tokenCounter,
	}, nil
}

// Tool 1: analyze_file
func (h *ToolHandler) AnalyzeFile(ctx context.Context, params map[string]interface{}) (*models.AnalyzeFileResponse, error) {
	// Extract parameters
	filepath, ok := params["filepath"].(string)
	if !ok {
		return nil, fmt.Errorf("filepath parameter is required")
	}

	chunkSize := 50 // default
	if cs, ok := params["chunk_size"].(float64); ok {
		chunkSize = int(cs)
	}

	streamMode := true // default for > 100MB
	if sm, ok := params["stream_mode"].(bool); ok {
		streamMode = sm
	}

	startTime := time.Now()

	// Open and validate file
	file, err := excelize.OpenFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open XLSM file: %w", err)
	}
	defer file.Close()

	// Calculate file metadata
	metadata, err := h.calculateFileMetadata(filepath, file)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate metadata: %w", err)
	}

	// Detect model and configure token management
	modelDetected := h.detectModel(ctx)
	tokenMgmt := h.createTokenManagement(modelDetected, chunkSize)

	// Check if streaming is needed
	if metadata.FileSize > 100*1024*1024 { // 100MB
		streamMode = true
	}

	// Create chunks
	chunks, err := h.createChunks(file, metadata.SheetsCount, chunkSize, streamMode, metadata.Checksum)
	if err != nil {
		return nil, fmt.Errorf("failed to create chunks: %w", err)
	}

	// Detect patterns
	patterns, err := h.detectPatterns(file)
	if err != nil {
		return nil, fmt.Errorf("failed to detect patterns: %w", err)
	}

	// Create index summary
	indexSummary, err := h.createIndexSummary(file)
	if err != nil {
		return nil, fmt.Errorf("failed to create index summary: %w", err)
	}

	// Generate next cursor if more chunks exist
	var nextCursor string
	hasMore := len(chunks) > 1
	if hasMore {
		nextCursor = h.cursorManager.CreateChunkCursor(
			chunks[1].ChunkID,
			int64(chunks[1].SheetsRange[0]),
			metadata.Checksum,
			nil,
		)
	}

	analysisTime := time.Since(startTime)

	response := &models.AnalyzeFileResponse{
		Metadata:         *metadata,
		Chunks:           chunks,
		PatternsDetected: *patterns,
		TokenManagement:  *tokenMgmt,
		IndexSummary:     *indexSummary,
		NextCursor:       nextCursor,
		HasMore:          hasMore,
		Performance: models.PerformanceMetrics{
			AnalysisTimeMs: analysisTime.Milliseconds(),
			MemoryUsedMB:   h.estimateMemoryUsage(metadata.SheetsCount),
		},
	}

	return response, nil
}

func (h *ToolHandler) calculateFileMetadata(filepath string, file *excelize.File) (*models.FileMetadata, error) {
	// Get file info
	fileInfo, err := os.Stat(filepath)
	if err != nil {
		return nil, err
	}

	// Calculate checksum
	fileData, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(fileData)
	checksum := fmt.Sprintf("%x", hash)

	// Count sheets
	sheetList := file.GetSheetList()
	sheetsCount := len(sheetList)

	// Calculate complexity score
	complexityScore := h.calculateComplexityScore(file, sheetsCount)

	// Estimate memory usage
	memoryEstimate := h.estimateMemoryUsage(sheetsCount)

	return &models.FileMetadata{
		Checksum:         checksum,
		FileSize:         fileInfo.Size(),
		SheetsCount:      sheetsCount,
		Timestamp:        fileInfo.ModTime(),
		ComplexityScore:  complexityScore,
		MemoryEstimate:   memoryEstimate,
	}, nil
}

func (h *ToolHandler) calculateComplexityScore(file *excelize.File, sheetsCount int) float64 {
	score := float64(sheetsCount) * 0.1

	// Sample first few sheets for complexity indicators
	sheetList := file.GetSheetList()
	sampleSize := 5
	if len(sheetList) < sampleSize {
		sampleSize = len(sheetList)
	}

	for i := 0; i < sampleSize; i++ {
		sheetName := sheetList[i]
		
		// Count rows with data
		rows, err := file.GetRows(sheetName)
		if err != nil {
			continue
		}
		
		rowCount := len(rows)
		score += float64(rowCount) * 0.001

		// Check for formulas in sample cells
		for j := 0; j < 10 && j < rowCount; j++ {
			if j < len(rows) {
				for k := 0; k < 10 && k < len(rows[j]); k++ {
					cellRef, _ := excelize.CoordinatesToCellName(k+1, j+1)
					formula, err := file.GetCellFormula(sheetName, cellRef)
					if err == nil && formula != "" {
						score += 0.1
					}
				}
			}
		}
	}

	// Normalize score to 0-10 range
	if score > 10 {
		score = 10
	}

	return score
}

func (h *ToolHandler) estimateMemoryUsage(sheetsCount int) int64 {
	// Rough estimation: 1MB base + 500KB per sheet
	baseMB := int64(1)
	perSheetKB := int64(500)
	
	return (baseMB * 1024 * 1024) + (int64(sheetsCount) * perSheetKB * 1024)
}

func (h *ToolHandler) detectModel(ctx context.Context) string {
	// Try to detect model from context or headers
	// Default to sonnet-4 for now
	return "sonnet-4"
}

func (h *ToolHandler) createTokenManagement(modelDetected string, chunkSize int) *models.TokenManagement {
	limits := h.tokenCounter.GetModelLimits(modelDetected)
	
	// Calculate optimal chunking strategy
	optimalChunkSize := h.tokenCounter.CalculateOptimalChunkSize(modelDetected, 0.8)
	estimatedTokens := optimalChunkSize

	return &models.TokenManagement{
		ModelDetected:  modelDetected,
		CountingMethod: "tiktoken",
		Limits: models.TokenLimits{
			Context:    limits.Context,
			SafeBuffer: limits.SafeBuffer,
			OutputMax:  limits.OutputMax,
		},
		ChunkingStrategy: models.ChunkingStrategy{
			SheetsPerChunk:  chunkSize,
			EstimatedTokens: estimatedTokens,
			ActualTokens:    0, // Will be calculated during actual processing
		},
	}
}

func (h *ToolHandler) createChunks(file *excelize.File, sheetsCount, chunkSize int, streamMode bool, checksum string) ([]models.Chunk, error) {
	var chunks []models.Chunk
	
	for i := 0; i < sheetsCount; i += chunkSize {
		endIdx := i + chunkSize
		if endIdx > sheetsCount {
			endIdx = sheetsCount
		}

		chunkID := fmt.Sprintf("chunk_%d_%d", i, endIdx-1)
		
		// Estimate chunk size
		sizeBytes := h.estimateChunkSize(file, i, endIdx)
		
		chunk := models.Chunk{
			ChunkID:           chunkID,
			SheetsRange:       [2]int{i, endIdx - 1},
			SizeBytes:         sizeBytes,
			StreamingRequired: streamMode && sizeBytes > 10*1024*1024, // 10MB threshold
			Cursor: h.cursorManager.CreateChunkCursor(
				chunkID,
				int64(i),
				checksum,
				nil,
			),
		}
		
		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

func (h *ToolHandler) estimateChunkSize(file *excelize.File, startIdx, endIdx int) int64 {
	sheetList := file.GetSheetList()
	totalSize := int64(0)

	for i := startIdx; i < endIdx && i < len(sheetList); i++ {
		rows, err := file.GetRows(sheetList[i])
		if err != nil {
			continue
		}
		
		// Rough estimation: 50 bytes per cell on average
		cellCount := 0
		for _, row := range rows {
			cellCount += len(row)
		}
		
		totalSize += int64(cellCount * 50)
	}

	return totalSize
}

func (h *ToolHandler) detectPatterns(file *excelize.File) (*models.PatternsDetected, error) {
	sheetList := file.GetSheetList()
	
	// Detect naming patterns
	namingPatterns := h.analyzeNamingPatterns(sheetList)
	
	// Analyze data types (sample first few sheets)
	dataTypes := make(map[string]interface{})
	dataTypes["text"] = 0
	dataTypes["numeric"] = 0
	dataTypes["formula"] = 0
	dataTypes["date"] = 0

	// Detect structural groups
	structuralGroups := h.detectStructuralGroups(sheetList)
	
	// Analyze formula complexity
	formulaComplexity := h.analyzeFormulaComplexity(file, sheetList)

	return &models.PatternsDetected{
		NamingPatterns:    namingPatterns,
		DataTypes:         dataTypes,
		StructuralGroups:  structuralGroups,
		FormulaComplexity: formulaComplexity,
	}, nil
}

func (h *ToolHandler) analyzeNamingPatterns(sheetList []string) []string {
	patterns := make(map[string]bool)
	
	for _, name := range sheetList {
		// Look for common patterns
		if strings.Contains(name, "_") {
			patterns["underscore_separated"] = true
		}
		if strings.Contains(name, " ") {
			patterns["space_separated"] = true
		}
		if len(name) > 0 && name[0] >= '0' && name[0] <= '9' {
			patterns["starts_with_number"] = true
		}
		// Check for date patterns
		if strings.Contains(name, "2023") || strings.Contains(name, "2024") || strings.Contains(name, "2025") {
			patterns["contains_year"] = true
		}
	}
	
	var result []string
	for pattern := range patterns {
		result = append(result, pattern)
	}
	
	return result
}

func (h *ToolHandler) detectStructuralGroups(sheetList []string) int {
	// Simple grouping based on name prefixes
	groups := make(map[string]bool)
	
	for _, name := range sheetList {
		// Extract prefix (before first _ or space)
		prefix := name
		if idx := strings.Index(name, "_"); idx > 0 {
			prefix = name[:idx]
		} else if idx := strings.Index(name, " "); idx > 0 {
			prefix = name[:idx]
		}
		
		groups[prefix] = true
	}
	
	return len(groups)
}

func (h *ToolHandler) analyzeFormulaComplexity(file *excelize.File, sheetList []string) string {
	formulaCount := 0
	complexFormulaCount := 0
	
	// Sample first 3 sheets
	sampleSize := 3
	if len(sheetList) < sampleSize {
		sampleSize = len(sheetList)
	}
	
	for i := 0; i < sampleSize; i++ {
		sheetName := sheetList[i]
		rows, err := file.GetRows(sheetName)
		if err != nil {
			continue
		}
		
		// Sample first 10x10 cells
		for j := 0; j < 10 && j < len(rows); j++ {
			for k := 0; k < 10 && k < len(rows[j]); k++ {
				cellRef, _ := excelize.CoordinatesToCellName(k+1, j+1)
				formula, err := file.GetCellFormula(sheetName, cellRef)
				if err == nil && formula != "" {
					formulaCount++
					
					// Check for complex formulas
					if strings.Contains(formula, "IF") || 
					   strings.Contains(formula, "VLOOKUP") || 
					   strings.Contains(formula, "INDEX") ||
					   strings.Contains(formula, "MATCH") {
						complexFormulaCount++
					}
				}
			}
		}
	}
	
	if formulaCount == 0 {
		return "none"
	} else if complexFormulaCount == 0 {
		return "simple"
	} else if float64(complexFormulaCount)/float64(formulaCount) > 0.3 {
		return "complex"
	} else {
		return "mixed"
	}
}

func (h *ToolHandler) createIndexSummary(file *excelize.File) (*models.IndexSummary, error) {
	// This is a simplified version - in production, this would be more comprehensive
	return &models.IndexSummary{
		ValueTypes: map[string]interface{}{
			"numeric": 0,
			"text":    0,
			"formula": 0,
			"empty":   0,
		},
		FormulaPatterns: []string{},
		SheetGroups:     []string{},
		CircularRefs:    []string{},
	}, nil
}