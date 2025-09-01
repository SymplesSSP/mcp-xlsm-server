package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"mcp-xlsm-server/internal/index"
	"mcp-xlsm-server/internal/models"
)

// Tool 3: query_data
func (h *ToolHandler) QueryData(ctx context.Context, params map[string]interface{}) (*models.QueryDataResponse, error) {
	// Extract parameters
	query, ok := params["query"].(string)
	if !ok {
		return nil, fmt.Errorf("query parameter is required")
	}

	navigationIndexData, ok := params["navigation_index"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("navigation_index parameter is required")
	}

	// Optional parameters
	continuationCursor := ""
	if cc, ok := params["continuation_cursor"].(string); ok {
		continuationCursor = cc
	}

	// Window configuration
	windowConfig := map[string]interface{}{
		"max_rows_per_sheet":    1000,
		"max_sheets_per_call":   10,
		"max_results":           100,
		"stream_large_results":  false,
	}
	if wc, ok := params["window_config"].(map[string]interface{}); ok {
		for k, v := range wc {
			windowConfig[k] = v
		}
	}

	tokenAware := true
	if ta, ok := params["token_aware"].(bool); ok {
		tokenAware = ta
	}

	optimizationHints := map[string]interface{}{
		"prefer_speed":        true,
		"prefer_completeness": false,
	}
	if oh, ok := params["optimization_hints"].(map[string]interface{}); ok {
		for k, v := range oh {
			optimizationHints[k] = v
		}
	}

	startTime := time.Now()

	// Parse navigation index
	navigationIndex, err := h.parseNavigationIndex(navigationIndexData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse navigation index: %w", err)
	}

	// Parse continuation cursor if provided
	var offset int64
	var window *models.Window
	if continuationCursor != "" {
		cursorData, err := h.cursorManager.ParseCursor(continuationCursor)
		if err != nil {
			return nil, fmt.Errorf("invalid continuation cursor: %w", err)
		}
		offset = cursorData.Offset
		window = cursorData.WindowInfo
	}

	// Execute query
	queryExecution, results, err := h.executeQuery(query, navigationIndex, offset, window, windowConfig, optimizationHints)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	// Calculate statistics if needed
	statistics := h.calculateStatistics(results, query)

	// Apply adaptive response based on model and token limits
	adaptiveResponse, err := h.applyAdaptiveResponse(results, tokenAware)
	if err != nil {
		return nil, fmt.Errorf("failed to apply adaptive response: %w", err)
	}

	// Create pagination for results
	pagination := h.createQueryPagination(query, offset, len(results.Data), windowConfig)

	// Detect index updates needed
	indexUpdates := h.detectIndexUpdates(query, results)

	queryTime := time.Since(startTime)

	response := &models.QueryDataResponse{
		QueryExecution:   *queryExecution,
		Results:          *results,
		Statistics:       *statistics,
		AdaptiveResponse: *adaptiveResponse,
		Pagination:       *pagination,
		IndexUpdates:     *indexUpdates,
		Performance: models.QueryPerformance{
			QueryTimeMs:      queryTime.Milliseconds(),
			IndexTimeMs:      0, // Would be measured during index operations
			TokenCountTimeMs: 0, // Would be measured during token counting
		},
	}

	return response, nil
}

func (h *ToolHandler) parseNavigationIndex(data map[string]interface{}) (*models.NavigationIndex, error) {
	// This is a simplified version - in production would use proper JSON unmarshaling
	// For now, create a minimal navigation index
	return &models.NavigationIndex{
		ChecksumMatch:        true,
		InvalidationRequired: false,
		ChunkInfo: models.ChunkInfo{
			Current:         "chunk_0",
			TotalChunks:     1,
			SheetsInChunk:   []string{"Sheet1"},
			StreamingActive: false,
		},
		SheetIndex: []models.SheetIndex{
			{
				SheetID: "sheet_0",
				Name:    "Sheet1",
				Metadata: models.SheetMetadata{
					Rows:            1000,
					Cols:            26,
					DataDensity:     0.5,
					HasFormulas:     true,
					MemoryFootprint: 1024000,
				},
			},
		},
	}, nil
}

func (h *ToolHandler) executeQuery(query string, navIndex *models.NavigationIndex, offset int64, window *models.Window, windowConfig map[string]interface{}, hints map[string]interface{}) (*models.QueryExecution, *models.QueryResults, error) {
	// Determine query strategy
	strategy := h.determineQueryStrategy(query, navIndex, hints)
	
	// Create index manager for searching
	indexManager := index.NewManager()
	
	var results []models.DataChunk
	usedIndex := false
	indexType := "none"
	var chunksScanned []string
	bloomFilterUsed := false

	// Execute based on strategy
	switch strategy {
	case "index":
		results, err := h.executeIndexQuery(query, indexManager, navIndex, windowConfig)
		if err != nil {
			return nil, nil, err
		}
		usedIndex = true
		indexType = h.detectIndexType(query)
		bloomFilterUsed = true
		
		return &models.QueryExecution{
			UsedIndex:       usedIndex,
			IndexType:       indexType,
			ChunksScanned:   chunksScanned,
			Strategy:        strategy,
			BloomFilterUsed: bloomFilterUsed,
		}, &models.QueryResults{Data: results}, nil

	case "scan":
		results, chunksScanned, err := h.executeScanQuery(query, navIndex, windowConfig)
		if err != nil {
			return nil, nil, err
		}
		
		return &models.QueryExecution{
			UsedIndex:       false,
			IndexType:       "none",
			ChunksScanned:   chunksScanned,
			Strategy:        strategy,
			BloomFilterUsed: false,
		}, &models.QueryResults{Data: results}, nil

	case "hybrid":
		// Combine index and scan approaches
		indexResults, _ := h.executeIndexQuery(query, indexManager, navIndex, windowConfig)
		scanResults, chunksScanned, _ := h.executeScanQuery(query, navIndex, windowConfig)
		
		// Merge results
		results = append(indexResults, scanResults...)
		
		return &models.QueryExecution{
			UsedIndex:       true,
			IndexType:       "hybrid",
			ChunksScanned:   chunksScanned,
			Strategy:        strategy,
			BloomFilterUsed: true,
		}, &models.QueryResults{Data: results}, nil

	default:
		return nil, nil, fmt.Errorf("unknown query strategy: %s", strategy)
	}
}

func (h *ToolHandler) determineQueryStrategy(query string, navIndex *models.NavigationIndex, hints map[string]interface{}) string {
	preferSpeed := true
	if ps, ok := hints["prefer_speed"].(bool); ok {
		preferSpeed = ps
	}

	// Simple heuristics for strategy selection
	if strings.Contains(query, "=") && preferSpeed {
		return "index" // Exact matches benefit from index
	}
	
	if strings.Contains(query, "*") || strings.Contains(query, "?") {
		return "scan" // Wildcard queries need scanning
	}
	
	if len(navIndex.SheetIndex) > 10 {
		return "hybrid" // Large datasets benefit from hybrid approach
	}

	return "scan" // Default to scan for simplicity
}

func (h *ToolHandler) detectIndexType(query string) string {
	if strings.Contains(query, ">=") || strings.Contains(query, "<=") || strings.Contains(query, ">") || strings.Contains(query, "<") {
		return "btree"
	}
	
	if strings.Contains(query, "NEAR") || strings.Contains(query, "WITHIN") {
		return "spatial"
	}
	
	return "inverted"
}

func (h *ToolHandler) executeIndexQuery(query string, indexManager *index.Manager, navIndex *models.NavigationIndex, windowConfig map[string]interface{}) ([]models.DataChunk, error) {
	var results []models.DataChunk

	// Parse query type and execute appropriate index search
	if isNumericRangeQuery(query) {
		min, max, err := parseNumericRange(query)
		if err != nil {
			return nil, err
		}
		
		locations := indexManager.SearchNumericRange(min, max)
		results = h.convertLocationsToDataChunks(locations, windowConfig)
		
	} else if isTextQuery(query) {
		locations := indexManager.SearchText(query)
		results = h.convertLocationsToDataChunks(locations, windowConfig)
		
	} else if isSpatialQuery(query) {
		bounds, err := parseSpatialBounds(query)
		if err != nil {
			return nil, err
		}
		
		locations := indexManager.SearchSpatial(*bounds)
		results = h.convertLocationsToDataChunks(locations, windowConfig)
	}

	// Apply windowing limits
	maxResults := 100
	if mr, ok := windowConfig["max_results"].(int); ok {
		maxResults = mr
	}
	
	if len(results) > maxResults {
		results = results[:maxResults]
	}

	return results, nil
}

func (h *ToolHandler) executeScanQuery(query string, navIndex *models.NavigationIndex, windowConfig map[string]interface{}) ([]models.DataChunk, []string, error) {
	var results []models.DataChunk
	var chunksScanned []string

	maxSheetsPerCall := 10
	if ms, ok := windowConfig["max_sheets_per_call"].(int); ok {
		maxSheetsPerCall = ms
	}

	maxRowsPerSheet := 1000
	if mr, ok := windowConfig["max_rows_per_sheet"].(int); ok {
		maxRowsPerSheet = mr
	}

	// Extract real financial data from Excel sheets
	for i, sheet := range navIndex.SheetIndex {
		if i >= maxSheetsPerCall {
			break
		}
		
		chunksScanned = append(chunksScanned, sheet.SheetID)
		
		// Check if this is the target sheet (FROUDIS or CHAMDIS)
		if strings.Contains(strings.ToUpper(sheet.Name), strings.ToUpper(query)) {
			// Extract real data from the Excel file
			realData, err := h.extractRealSheetData(sheet.Name, maxRowsPerSheet)
			if err != nil {
				continue
			}
			
			dataChunk := models.DataChunk{
				Location: fmt.Sprintf("%s!A1", sheet.Name),
				Window:   fmt.Sprintf("A1:Z%d", maxRowsPerSheet),
				DataChunk: realData,
				Metadata: models.ChunkMetadata{
					Size:       int64(len(realData) * 100), // Rough estimate
					Truncated:  sheet.Metadata.Rows > maxRowsPerSheet,
					Compressed: false,
				},
				Context: models.Context{
					Headers:  []string{"Rayons", "Ventes_HT", "Marges", "Taux_Marge", "Demarque", "Frais", "Marge_Theorique"},
					Nearby:   map[string]interface{}{"sheet_data": len(realData)},
					Formulas: []string{},
				},
			}
			
			results = append(results, dataChunk)
		}
	}

	return results, chunksScanned, nil
}

func (h *ToolHandler) convertLocationsToDataChunks(locations []index.Location, windowConfig map[string]interface{}) []models.DataChunk {
	var chunks []models.DataChunk

	maxResults := 100
	if mr, ok := windowConfig["max_results"].(int); ok {
		maxResults = mr
	}

	for i, loc := range locations {
		if i >= maxResults {
			break
		}

		chunk := models.DataChunk{
			Location:  fmt.Sprintf("%s!%s", loc.SheetName, loc.CellRef),
			Window:    fmt.Sprintf("%s:%s", loc.CellRef, loc.CellRef), // Single cell window
			DataChunk: "sample_value", // Would be actual cell value
			Metadata: models.ChunkMetadata{
				Size:       64,
				Truncated:  false,
				Compressed: false,
			},
			Context: models.Context{
				Headers:  []string{},
				Nearby:   map[string]interface{}{},
				Formulas: []string{},
			},
		}

		chunks = append(chunks, chunk)
	}

	return chunks
}

func (h *ToolHandler) calculateStatistics(results *models.QueryResults, query string) *models.Statistics {
	// Simple statistics calculation
	return &models.Statistics{
		Aggregations:       []interface{}{},
		Patterns:           []interface{}{},
		Outliers:           []interface{}{},
		FormulaEvaluations: []interface{}{},
	}
}

func (h *ToolHandler) applyAdaptiveResponse(results *models.QueryResults, tokenAware bool) (*models.AdaptiveResponse, error) {
	if !tokenAware {
		return &models.AdaptiveResponse{}, nil
	}

	// Count tokens in results
	tokenCount, err := h.tokenCounter.Count(results)
	if err != nil {
		return nil, err
	}

	// Get model limits (detect or default)
	modelName := "sonnet-4"
	limits := h.tokenCounter.GetModelLimits(modelName)

	return &models.AdaptiveResponse{
		ModelContext: models.ModelContext{
			Detected:     modelName,
			Limit:        limits.Context,
			Used:         tokenCount,
			PreciseCount: tokenCount,
		},
		IfSonnetBeta: models.StrategyConfig{
			MaxResults:  500,
			WindowRows:  5000,
			Compression: "light",
		},
		IfStandard: models.StrategyConfig{
			MaxResults:  100,
			WindowRows:  1000,
			Compression: "medium",
		},
		IfOpus: models.StrategyConfig{
			MaxResults:  100,
			WindowRows:  800,
			Compression: "aggressive",
		},
	}, nil
}

func (h *ToolHandler) createQueryPagination(query string, offset int64, resultCount int, windowConfig map[string]interface{}) *models.Pagination {
	maxResults := 100
	if mr, ok := windowConfig["max_results"].(int); ok {
		maxResults = mr
	}

	hasMore := resultCount >= maxResults
	var nextCursor string
	
	if hasMore {
		nextWindow := &models.Window{
			StartRow: int(offset) + maxResults,
			EndRow:   int(offset) + maxResults*2,
			StartCol: 0,
			EndCol:   100,
		}
		nextCursor = h.cursorManager.CreateQueryCursor(query, offset+int64(maxResults), "", nextWindow)
	}

	return &models.Pagination{
		CurrentCursor:          "",
		NextCursor:             nextCursor,
		PreviousCursor:         "",
		RemainingChunks:        0,
		EstimatedTimeRemaining: 0,
	}
}

func (h *ToolHandler) detectIndexUpdates(query string, results *models.QueryResults) *models.IndexUpdates {
	// Detect if query revealed new patterns that should be indexed
	var newPatterns []string
	var suggestedChunks []string

	// Simple pattern detection
	if len(results.Data) > 50 {
		newPatterns = append(newPatterns, "high_volume_query")
	}

	if strings.Contains(query, "SUM") || strings.Contains(query, "COUNT") {
		newPatterns = append(newPatterns, "aggregation_query")
	}

	return &models.IndexUpdates{
		NewPatterns:     newPatterns,
		SuggestedChunks: suggestedChunks,
		DeltaApplied:    false,
	}
}

// Query parsing helper functions
func isNumericRangeQuery(query string) bool {
	return strings.Contains(query, ">=") || strings.Contains(query, "<=") || 
		   strings.Contains(query, ">") || strings.Contains(query, "<") ||
		   strings.Contains(query, "BETWEEN")
}

func parseNumericRange(query string) (float64, float64, error) {
	// Simplified parsing - in production would be more robust
	if strings.Contains(query, "BETWEEN") {
		// Parse "value BETWEEN 10 AND 20"
		return 10.0, 20.0, nil
	}
	
	if strings.Contains(query, ">=") {
		// Parse "value >= 10"
		return 10.0, 999999.0, nil
	}
	
	// Default range
	return 0.0, 100.0, nil
}

func isTextQuery(query string) bool {
	return !isNumericRangeQuery(query) && !isSpatialQuery(query)
}

func isSpatialQuery(query string) bool {
	return strings.Contains(query, "NEAR") || strings.Contains(query, "WITHIN") ||
		   strings.Contains(query, "RANGE")
}

func parseSpatialBounds(query string) (*index.Rectangle, error) {
	// Simplified spatial bounds parsing
	return &index.Rectangle{
		X:      0,
		Y:      0,
		Width:  100,
		Height: 100,
	}, nil
}

// extractRealSheetData extrait les vraies données financières d'une feuille Excel
func (h *ToolHandler) extractRealSheetData(sheetName string, maxRows int) ([][]interface{}, error) {
	// Ouvrir le fichier Excel (en dur pour l'instant)
	file, err := excelize.OpenFile("/Volumes/SSD/MCP/COMBINE INTERACTIF 02 2025.xlsm")
	if err != nil {
		return nil, fmt.Errorf("failed to open Excel file: %w", err)
	}
	defer file.Close()

	// Obtenir les lignes de la feuille
	rows, err := file.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to get rows from sheet %s: %w", sheetName, err)
	}

	var financialData [][]interface{}
	
	// Limiter le nombre de lignes
	maxRowsToProcess := len(rows)
	if maxRows > 0 && maxRows < len(rows) {
		maxRowsToProcess = maxRows
	}

	// Extraire les données ligne par ligne
	for i := 0; i < maxRowsToProcess; i++ {
		if i >= len(rows) {
			break
		}
		
		row := rows[i]
		var processedRow []interface{}
		
		// Traiter chaque cellule de la ligne
		for j, cell := range row {
			if j > 20 { // Limiter à 20 colonnes pour éviter les données vides
				break
			}
			
			// Convertir les valeurs numériques si possible
			if cell == "" {
				processedRow = append(processedRow, nil)
			} else {
				// Essayer de parser en nombre
				if num, err := parseFinancialValue(cell); err == nil {
					processedRow = append(processedRow, num)
				} else {
					processedRow = append(processedRow, cell)
				}
			}
		}
		
		// Ajouter seulement les lignes non vides
		if len(processedRow) > 0 && hasNonEmptyData(processedRow) {
			financialData = append(financialData, processedRow)
		}
	}

	return financialData, nil
}

// parseFinancialValue tente de parser une valeur financière
func parseFinancialValue(value string) (float64, error) {
	if value == "" {
		return 0, fmt.Errorf("empty value")
	}
	
	// Nettoyer la valeur (supprimer espaces, virgules françaises)
	cleaned := strings.ReplaceAll(value, " ", "")
	cleaned = strings.ReplaceAll(cleaned, ",", ".")
	
	// Essayer de parser
	var result float64
	n, err := fmt.Sscanf(cleaned, "%f", &result)
	if err != nil || n != 1 {
		return 0, fmt.Errorf("not a number: %s", value)
	}
	
	return result, nil
}

// hasNonEmptyData vérifie si une ligne contient des données non vides
func hasNonEmptyData(row []interface{}) bool {
	for _, cell := range row {
		if cell != nil && cell != "" {
			return true
		}
	}
	return false
}