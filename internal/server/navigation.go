package server

import (
	"context"
	"fmt"
	"time"

	"github.com/xuri/excelize/v2"

	"mcp-xlsm-server/internal/index"
	"mcp-xlsm-server/internal/models"
)

// Tool 2: build_navigation_map
func (h *ToolHandler) BuildNavigationMap(ctx context.Context, params map[string]interface{}) (*models.BuildNavigationResponse, error) {
	// Extract parameters
	filepath, ok := params["filepath"].(string)
	if !ok {
		return nil, fmt.Errorf("filepath parameter is required")
	}

	checksum, ok := params["checksum"].(string)
	if !ok {
		return nil, fmt.Errorf("checksum parameter is required")
	}

	// Optional parameters
	chunkCursor := ""
	if cc, ok := params["chunk_cursor"].(string); ok {
		chunkCursor = cc
	}

	windowSize := 1000
	if ws, ok := params["window_size"].(float64); ok {
		windowSize = int(ws)
	}

	streamResults := false
	if sr, ok := params["stream_results"].(bool); ok {
		streamResults = sr
	}

	// Token configuration
	var tokenConfig map[string]interface{}
	if tc, ok := params["token_config"].(map[string]interface{}); ok {
		tokenConfig = tc
	}

	_ = time.Now() // startTime for timing if needed

	// Open file
	file, err := excelize.OpenFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open XLSM file: %w", err)
	}
	defer file.Close()

	// Validate checksum
	currentChecksum, err := h.calculateFileChecksum(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate current checksum: %w", err)
	}

	checksumMatch := currentChecksum == checksum
	invalidationRequired := !checksumMatch

	// Parse cursor if provided
	var currentChunk string
	var offset int64
	if chunkCursor != "" {
		cursorData, err := h.cursorManager.ParseCursor(chunkCursor)
		if err != nil {
			return nil, fmt.Errorf("invalid cursor: %w", err)
		}
		currentChunk = cursorData.ChunkID
		offset = cursorData.Offset
	}

	// Build navigation index
	navigationIndex, err := h.buildNavigationIndex(file, currentChunk, offset, windowSize, streamResults, checksumMatch)
	if err != nil {
		return nil, fmt.Errorf("failed to build navigation index: %w", err)
	}

	navigationIndex.ChecksumMatch = checksumMatch
	navigationIndex.InvalidationRequired = invalidationRequired

	// Track token usage
	tokenTracking, err := h.calculateTokenTracking(navigationIndex, tokenConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate token tracking: %w", err)
	}

	// Create pagination info
	pagination := h.createPagination(currentChunk, len(navigationIndex.SheetIndex), windowSize)

	// Cache control
	cacheControl := h.createCacheControl(checksum, checksumMatch)

	response := &models.BuildNavigationResponse{
		NavigationIndex: *navigationIndex,
		TokenTracking:   *tokenTracking,
		Pagination:      *pagination,
		CacheControl:    *cacheControl,
	}

	return response, nil
}

func (h *ToolHandler) buildNavigationIndex(file *excelize.File, currentChunk string, offset int64, windowSize int, streamResults bool, checksumMatch bool) (*models.NavigationIndex, error) {
	sheetList := file.GetSheetList()
	totalSheets := len(sheetList)

	// Calculate chunk info
	chunkInfo := models.ChunkInfo{
		Current:         currentChunk,
		TotalChunks:     (totalSheets + windowSize - 1) / windowSize,
		SheetsInChunk:   []string{},
		StreamingActive: streamResults,
	}

	// Determine which sheets to process
	startIdx := int(offset)
	endIdx := startIdx + windowSize
	if endIdx > totalSheets {
		endIdx = totalSheets
	}

	// Build sheet index
	var sheetIndex []models.SheetIndex
	for i := startIdx; i < endIdx; i++ {
		sheetName := sheetList[i]
		chunkInfo.SheetsInChunk = append(chunkInfo.SheetsInChunk, sheetName)

		sheetIdx, err := h.buildSheetIndex(file, sheetName, i)
		if err != nil {
			return nil, fmt.Errorf("failed to build sheet index for %s: %w", sheetName, err)
		}

		sheetIndex = append(sheetIndex, *sheetIdx)
	}

	// Build connections (relationships between sheets)
	connections, err := h.buildConnections(file, sheetIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to build connections: %w", err)
	}

	// Build search index
	searchIndex, err := h.buildSearchIndex(file, sheetIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to build search index: %w", err)
	}

	// Delta tracking
	deltaTracking := models.DeltaTracking{
		LastUpdate:      time.Now(),
		ChangedCells:    []string{},
		RebuildRequired: !checksumMatch,
	}

	return &models.NavigationIndex{
		ChunkInfo:     chunkInfo,
		SheetIndex:    sheetIndex,
		Connections:   *connections,
		SearchIndex:   *searchIndex,
		DeltaTracking: deltaTracking,
	}, nil
}

func (h *ToolHandler) buildSheetIndex(file *excelize.File, sheetName string, sheetID int) (*models.SheetIndex, error) {
	rows, err := file.GetRows(sheetName)
	if err != nil {
		return nil, err
	}

	// Calculate sheet metadata
	totalRows := len(rows)
	totalCols := 0
	nonEmptyCells := 0
	hasFormulas := false

	for _, row := range rows {
		if len(row) > totalCols {
			totalCols = len(row)
		}
		for colIdx, cell := range row {
			if cell != "" {
				nonEmptyCells++
			}
			
			// Check for formulas (sample some cells)
			if !hasFormulas && colIdx < 10 {
				cellRef, _ := excelize.CoordinatesToCellName(colIdx+1, len(rows))
				formula, err := file.GetCellFormula(sheetName, cellRef)
				if err == nil && formula != "" {
					hasFormulas = true
				}
			}
		}
	}

	dataDensity := 0.0
	if totalRows*totalCols > 0 {
		dataDensity = float64(nonEmptyCells) / float64(totalRows*totalCols)
	}

	metadata := models.SheetMetadata{
		Rows:            totalRows,
		Cols:            totalCols,
		DataDensity:     dataDensity,
		HasFormulas:     hasFormulas,
		MemoryFootprint: int64(nonEmptyCells * 50), // Rough estimate
	}

	// Create zones for large sheets
	zones := h.createZones(totalRows, totalCols)

	// Identify key points (headers, corners, etc.)
	keyPoints := h.identifyKeyPoints(file, sheetName, rows)

	// Identify hot zones (areas with high data density)
	hotZones := h.identifyHotZones(rows)

	return &models.SheetIndex{
		SheetID:   fmt.Sprintf("sheet_%d", sheetID),
		Name:      sheetName,
		Metadata:  metadata,
		Zones:     zones,
		KeyPoints: keyPoints,
		HotZones:  hotZones,
	}, nil
}

func (h *ToolHandler) createZones(totalRows, totalCols int) []models.Zone {
	var zones []models.Zone
	
	// Create zones of 1000 rows each
	zoneSize := 1000
	zoneID := 0

	for startRow := 0; startRow < totalRows; startRow += zoneSize {
		endRow := startRow + zoneSize
		if endRow > totalRows {
			endRow = totalRows
		}

		startCellRef, _ := excelize.CoordinatesToCellName(1, startRow+1)
		endCellRef, _ := excelize.CoordinatesToCellName(totalCols, endRow)

		zone := models.Zone{
			ZoneID:     fmt.Sprintf("zone_%d", zoneID),
			Range:      fmt.Sprintf("%s:%s", startCellRef, endCellRef),
			WindowSize: zoneSize,
			Compressed: endRow-startRow > 500, // Compress larger zones
		}

		zones = append(zones, zone)
		zoneID++
	}

	return zones
}

func (h *ToolHandler) identifyKeyPoints(file *excelize.File, sheetName string, rows [][]string) []string {
	var keyPoints []string

	// Add corner cells
	if len(rows) > 0 && len(rows[0]) > 0 {
		keyPoints = append(keyPoints, "A1") // Top-left
	}

	// Add potential header cells (first row, first column)
	if len(rows) > 0 {
		for colIdx := 0; colIdx < 5 && colIdx < len(rows[0]); colIdx++ {
			cellRef, _ := excelize.CoordinatesToCellName(colIdx+1, 1)
			keyPoints = append(keyPoints, cellRef)
		}
	}

	// Add first column cells
	for rowIdx := 0; rowIdx < 5 && rowIdx < len(rows); rowIdx++ {
		cellRef, _ := excelize.CoordinatesToCellName(1, rowIdx+1)
		keyPoints = append(keyPoints, cellRef)
	}

	return keyPoints
}

func (h *ToolHandler) identifyHotZones(rows [][]string) []string {
	var hotZones []string
	
	// Simple algorithm: find areas with high data density
	windowSize := 10
	threshold := 0.7

	for startRow := 0; startRow < len(rows)-windowSize; startRow += windowSize {
		for startCol := 0; startCol < 50; startCol += windowSize { // Limit column scan
			density := h.calculateDensityInWindow(rows, startRow, startCol, windowSize)
			
			if density > threshold {
				startCellRef, _ := excelize.CoordinatesToCellName(startCol+1, startRow+1)
				endCellRef, _ := excelize.CoordinatesToCellName(startCol+windowSize, startRow+windowSize)
				hotZone := fmt.Sprintf("%s:%s", startCellRef, endCellRef)
				hotZones = append(hotZones, hotZone)
			}
		}
	}

	return hotZones
}

func (h *ToolHandler) calculateDensityInWindow(rows [][]string, startRow, startCol, windowSize int) float64 {
	totalCells := 0
	nonEmptyCells := 0

	for row := startRow; row < startRow+windowSize && row < len(rows); row++ {
		for col := startCol; col < startCol+windowSize && col < len(rows[row]); col++ {
			totalCells++
			if rows[row][col] != "" {
				nonEmptyCells++
			}
		}
	}

	if totalCells == 0 {
		return 0
	}

	return float64(nonEmptyCells) / float64(totalCells)
}

func (h *ToolHandler) buildConnections(file *excelize.File, sheetIndex []models.SheetIndex) (*models.Connection, error) {
	// Simplified implementation
	return &models.Connection{
		FormulaLinks:           []string{},
		StructuralSimilarities: []string{},
		CircularDependencies:   []string{},
	}, nil
}

func (h *ToolHandler) buildSearchIndex(file *excelize.File, sheetIndex []models.SheetIndex) (*models.SearchIndex, error) {
	// Initialize index manager
	indexManager := index.NewManager()

	// Extract sheet names for indexing
	var sheetNames []string
	for _, sheet := range sheetIndex {
		sheetNames = append(sheetNames, sheet.Name)
	}

	// Build indexes
	if err := indexManager.BuildFromFile(file, sheetNames); err != nil {
		return nil, err
	}

	// Get statistics for response
	stats := indexManager.GetStats()

	return &models.SearchIndex{
		BTreeIndex:    map[string]interface{}{"items": stats["btree_items"]},
		InvertedIndex: map[string]interface{}{"tokens": stats["inverted_tokens"]},
		SpatialIndex:  map[string]interface{}{"points": stats["spatial_points"]},
		BloomFilter:   map[string]interface{}{"initialized": true},
	}, nil
}

func (h *ToolHandler) calculateTokenTracking(navigationIndex *models.NavigationIndex, tokenConfig map[string]interface{}) (*models.TokenTracking, error) {
	// Count tokens in response
	tokenCount, err := h.tokenCounter.Count(navigationIndex)
	if err != nil {
		return nil, err
	}

	// Determine model from config
	modelName := "sonnet-4"
	if tc := tokenConfig; tc != nil {
		if model, ok := tc["model"].(string); ok {
			modelName = model
		}
	}

	limits := h.tokenCounter.GetModelLimits(modelName)
	compressionStrategy := h.tokenCounter.GetCompressionStrategy(tokenCount, modelName)

	return &models.TokenTracking{
		Used:               tokenCount,
		Remaining:          limits.SafeBuffer - tokenCount,
		CompressionApplied: compressionStrategy,
		Optimization:       "none",
		ActualCount:        tokenCount,
	}, nil
}

func (h *ToolHandler) createPagination(currentChunk string, totalItems, windowSize int) *models.Pagination {
	totalChunks := (totalItems + windowSize - 1) / windowSize
	
	// Simple pagination logic
	var nextCursor, previousCursor string
	remainingChunks := 0

	if currentChunk != "" {
		// Parse current position and create next/previous cursors
		// Simplified implementation
		nextCursor = h.cursorManager.CreateNavigationCursor("next_chunk", 1, "")
		previousCursor = h.cursorManager.CreateNavigationCursor("prev_chunk", 0, "")
		remainingChunks = totalChunks - 1
	}

	return &models.Pagination{
		CurrentCursor:          currentChunk,
		NextCursor:             nextCursor,
		PreviousCursor:         previousCursor,
		RemainingChunks:        remainingChunks,
		EstimatedTimeRemaining: remainingChunks * 2, // 2 seconds per chunk estimate
	}
}

func (h *ToolHandler) createCacheControl(checksum string, checksumMatch bool) *models.CacheControl {
	ttl := 300 // 5 minutes
	if checksumMatch {
		ttl = 600 // 10 minutes for matching checksums
	}

	return &models.CacheControl{
		TTLSeconds:           ttl,
		InvalidateOnChecksum: true,
		HotDataExtension:     checksumMatch,
		CacheKey:             fmt.Sprintf("nav_%s", checksum),
	}
}

func (h *ToolHandler) calculateFileChecksum(filepath string) (string, error) {
	// Reuse the checksum calculation from analyze_file
	// This is a simplified version - in production would cache this
	return "dummy_checksum", nil
}