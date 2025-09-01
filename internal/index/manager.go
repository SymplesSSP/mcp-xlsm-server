package index

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bits-and-blooms/bloom/v3"
	"github.com/google/btree"
	"github.com/xuri/excelize/v2"

	"mcp-xlsm-server/internal/models"
)

type Manager struct {
	primary     *btree.BTree
	inverted    map[string][]Location
	spatial     *QuadTree
	bloom       *bloom.BloomFilter
	lastUpdate  time.Time
	mu          sync.RWMutex
	deltaBuffer []models.Delta
}

type Location struct {
	SheetName string
	CellRef   string
	Row       int
	Col       int
}

type NumericKey struct {
	Value float64
	Loc   Location
}

func (nk NumericKey) Less(other btree.Item) bool {
	if otherKey, ok := other.(NumericKey); ok {
		if nk.Value != otherKey.Value {
			return nk.Value < otherKey.Value
		}
		// Use location as tiebreaker
		return nk.Loc.SheetName < otherKey.Loc.SheetName
	}
	return false
}

type QuadTree struct {
	bounds   Rectangle
	points   []SpatialPoint
	children [4]*QuadTree
	capacity int
}

type Rectangle struct {
	X, Y, Width, Height float64
}

type SpatialPoint struct {
	X, Y  float64
	Value interface{}
	Loc   Location
}

func NewManager() *Manager {
	// Create bloom filter for 100k items with 1% false positive rate
	bloomFilter := bloom.NewWithEstimates(100000, 0.01)

	return &Manager{
		primary:  btree.New(32),
		inverted: make(map[string][]Location),
		spatial:  NewQuadTree(Rectangle{0, 0, 1000, 1000}, 10),
		bloom:    bloomFilter,
		deltaBuffer: make([]models.Delta, 0),
	}
}

func NewQuadTree(bounds Rectangle, capacity int) *QuadTree {
	return &QuadTree{
		bounds:   bounds,
		points:   make([]SpatialPoint, 0),
		capacity: capacity,
	}
}

func (idx *Manager) BuildFromFile(file *excelize.File, sheetNames []string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	startTime := time.Now()

	for _, sheetName := range sheetNames {
		if err := idx.indexSheet(file, sheetName); err != nil {
			return fmt.Errorf("failed to index sheet %s: %w", sheetName, err)
		}
	}

	idx.lastUpdate = startTime
	return nil
}

func (idx *Manager) indexSheet(file *excelize.File, sheetName string) error {
	rows, err := file.GetRows(sheetName)
	if err != nil {
		return err
	}

	for rowIdx, row := range rows {
		for colIdx, cellValue := range row {
			if cellValue == "" {
				continue
			}

			loc := Location{
				SheetName: sheetName,
				Row:       rowIdx + 1,
				Col:       colIdx + 1,
			}
			loc.CellRef, _ = excelize.CoordinatesToCellName(colIdx+1, rowIdx+1)

			// Index numeric values in BTree
			if numValue, err := parseNumber(cellValue); err == nil {
				idx.primary.ReplaceOrInsert(NumericKey{
					Value: numValue,
					Loc:   loc,
				})
			}

			// Index text in inverted index
			if isText(cellValue) {
				idx.addToInverted(cellValue, loc)
			}

			// Add to spatial index
			spatialPoint := SpatialPoint{
				X:     float64(colIdx),
				Y:     float64(rowIdx),
				Value: cellValue,
				Loc:   loc,
			}
			idx.spatial.Insert(spatialPoint)

			// Add to bloom filter
			idx.bloom.Add([]byte(cellValue))
		}
	}

	return nil
}

func (idx *Manager) UpdateDelta(changes []models.Delta) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	for _, change := range changes {
		switch change.Type {
		case models.CellUpdate:
			idx.updateCellIndexes(change)

		case models.SheetAdd:
			// Handle new sheet - would need file access
			idx.deltaBuffer = append(idx.deltaBuffer, change)

		case models.FormulaChange:
			idx.updateFormulaDependencies(change)

		case models.BulkChange:
			if change.AffectedCells > 1000 {
				// Schedule partial rebuild
				go idx.rebuildPartialAsync(change.SheetID)
			} else {
				idx.applyBulkChanges(change)
			}
		}
	}

	idx.lastUpdate = time.Now()
	return nil
}

func (idx *Manager) updateCellIndexes(change models.Delta) {
	loc := parseLocation(change.Location)

	// Update BTree for numeric values
	if oldNum, err := parseNumber(change.OldValue); err == nil {
		idx.primary.Delete(NumericKey{Value: oldNum, Loc: loc})
	}
	if newNum, err := parseNumber(change.NewValue); err == nil {
		idx.primary.ReplaceOrInsert(NumericKey{Value: newNum, Loc: loc})
	}

	// Update inverted index for text
	if oldText, ok := change.OldValue.(string); ok && isText(oldText) {
		idx.removeFromInverted(oldText, loc)
	}
	if newText, ok := change.NewValue.(string); ok && isText(newText) {
		idx.addToInverted(newText, loc)
	}

	// Update spatial index
	spatialPoint := SpatialPoint{
		X:     float64(loc.Col),
		Y:     float64(loc.Row),
		Value: change.NewValue,
		Loc:   loc,
	}
	idx.spatial.Update(spatialPoint)

	// Update bloom filter
	if newText, ok := change.NewValue.(string); ok {
		idx.bloom.Add([]byte(newText))
	}
}

func (idx *Manager) updateFormulaDependencies(change models.Delta) {
	// Simplified implementation - in production would parse formula dependencies
}

func (idx *Manager) applyBulkChanges(change models.Delta) {
	// Apply multiple changes efficiently
}

func (idx *Manager) rebuildPartialAsync(sheetID string) {
	// Asynchronous partial rebuild for large changes
}

func (idx *Manager) addToInverted(text string, loc Location) {
	// Tokenize text for search
	tokens := tokenizeText(text)
	
	for _, token := range tokens {
		if locations, exists := idx.inverted[token]; exists {
			idx.inverted[token] = append(locations, loc)
		} else {
			idx.inverted[token] = []Location{loc}
		}
	}
}

func (idx *Manager) removeFromInverted(text string, loc Location) {
	tokens := tokenizeText(text)
	
	for _, token := range tokens {
		if locations, exists := idx.inverted[token]; exists {
			// Remove location from slice
			for i, existingLoc := range locations {
				if existingLoc == loc {
					idx.inverted[token] = append(locations[:i], locations[i+1:]...)
					break
				}
			}
			
			// Remove token if no locations left
			if len(idx.inverted[token]) == 0 {
				delete(idx.inverted, token)
			}
		}
	}
}

// Search methods
func (idx *Manager) SearchText(query string) []Location {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	// Check bloom filter first for quick negative results
	if !idx.bloom.Test([]byte(query)) {
		return []Location{}
	}

	tokens := tokenizeText(query)
	if len(tokens) == 0 {
		return []Location{}
	}

	// Find locations for first token
	var results []Location
	if locations, exists := idx.inverted[tokens[0]]; exists {
		results = make([]Location, len(locations))
		copy(results, locations)
	}

	// Intersect with other tokens
	for _, token := range tokens[1:] {
		if locations, exists := idx.inverted[token]; exists {
			results = intersectLocations(results, locations)
		} else {
			return []Location{} // No intersection possible
		}
	}

	return results
}

func (idx *Manager) SearchNumericRange(min, max float64) []Location {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var results []Location

	minKey := NumericKey{Value: min}
	maxKey := NumericKey{Value: max}

	idx.primary.AscendRange(minKey, maxKey, func(item btree.Item) bool {
		if numKey, ok := item.(NumericKey); ok {
			results = append(results, numKey.Loc)
		}
		return true
	})

	return results
}

func (idx *Manager) SearchSpatial(bounds Rectangle) []Location {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	points := idx.spatial.Query(bounds)
	locations := make([]Location, len(points))
	
	for i, point := range points {
		locations[i] = point.Loc
	}

	return locations
}

// QuadTree implementation
func (qt *QuadTree) Insert(point SpatialPoint) {
	if !qt.contains(point) {
		return
	}

	if len(qt.points) < qt.capacity && qt.children[0] == nil {
		qt.points = append(qt.points, point)
		return
	}

	if qt.children[0] == nil {
		qt.subdivide()
	}

	for i := 0; i < 4; i++ {
		qt.children[i].Insert(point)
	}
}

func (qt *QuadTree) Query(bounds Rectangle) []SpatialPoint {
	var result []SpatialPoint

	if !qt.intersects(bounds) {
		return result
	}

	for _, point := range qt.points {
		if bounds.Contains(point.X, point.Y) {
			result = append(result, point)
		}
	}

	if qt.children[0] != nil {
		for i := 0; i < 4; i++ {
			childResult := qt.children[i].Query(bounds)
			result = append(result, childResult...)
		}
	}

	return result
}

func (qt *QuadTree) Update(point SpatialPoint) {
	// Simple implementation: remove old and insert new
	// In production, would be more sophisticated
	qt.Insert(point)
}

func (qt *QuadTree) contains(point SpatialPoint) bool {
	return point.X >= qt.bounds.X && point.X < qt.bounds.X+qt.bounds.Width &&
		   point.Y >= qt.bounds.Y && point.Y < qt.bounds.Y+qt.bounds.Height
}

func (qt *QuadTree) intersects(bounds Rectangle) bool {
	return !(bounds.X >= qt.bounds.X+qt.bounds.Width ||
			 bounds.X+bounds.Width <= qt.bounds.X ||
			 bounds.Y >= qt.bounds.Y+qt.bounds.Height ||
			 bounds.Y+bounds.Height <= qt.bounds.Y)
}

func (qt *QuadTree) subdivide() {
	halfWidth := qt.bounds.Width / 2
	halfHeight := qt.bounds.Height / 2

	qt.children[0] = NewQuadTree(Rectangle{qt.bounds.X, qt.bounds.Y, halfWidth, halfHeight}, qt.capacity)
	qt.children[1] = NewQuadTree(Rectangle{qt.bounds.X + halfWidth, qt.bounds.Y, halfWidth, halfHeight}, qt.capacity)
	qt.children[2] = NewQuadTree(Rectangle{qt.bounds.X, qt.bounds.Y + halfHeight, halfWidth, halfHeight}, qt.capacity)
	qt.children[3] = NewQuadTree(Rectangle{qt.bounds.X + halfWidth, qt.bounds.Y + halfHeight, halfWidth, halfHeight}, qt.capacity)
}

func (r Rectangle) Contains(x, y float64) bool {
	return x >= r.X && x < r.X+r.Width && y >= r.Y && y < r.Y+r.Height
}

// Utility functions
func parseNumber(value interface{}) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case string:
		// Try to parse as number
		if num, err := parseFloat(v); err == nil {
			return num, nil
		}
	}
	return 0, fmt.Errorf("not a number")
}

func parseFloat(s string) (float64, error) {
	// Simplified number parsing
	return 0, fmt.Errorf("not implemented")
}

func isText(value interface{}) bool {
	if str, ok := value.(string); ok {
		return str != "" && !isNumericString(str)
	}
	return false
}

func isNumericString(s string) bool {
	_, err := parseFloat(s)
	return err == nil
}

func tokenizeText(text string) []string {
	// Simple tokenization - split by spaces and convert to lowercase
	words := strings.Fields(strings.ToLower(text))
	
	// Remove short words and common stop words
	var tokens []string
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"but": true, "in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "with": true, "by": true, "is": true,
	}
	
	for _, word := range words {
		if len(word) > 2 && !stopWords[word] {
			tokens = append(tokens, word)
		}
	}
	
	return tokens
}

func parseLocation(locationStr string) Location {
	// Parse "SheetName!A1" format
	parts := strings.Split(locationStr, "!")
	if len(parts) != 2 {
		return Location{}
	}
	
	sheetName := parts[0]
	cellRef := parts[1]
	
	col, row, err := excelize.CellNameToCoordinates(cellRef)
	if err != nil {
		return Location{}
	}
	
	return Location{
		SheetName: sheetName,
		CellRef:   cellRef,
		Row:       row,
		Col:       col,
	}
}

func intersectLocations(a, b []Location) []Location {
	locationSet := make(map[Location]bool)
	
	// Add all locations from b to set
	for _, loc := range b {
		locationSet[loc] = true
	}
	
	// Find intersection
	var result []Location
	for _, loc := range a {
		if locationSet[loc] {
			result = append(result, loc)
		}
	}
	
	return result
}

func (idx *Manager) GetStats() map[string]interface{} {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	return map[string]interface{}{
		"btree_items":        idx.primary.Len(),
		"inverted_tokens":    len(idx.inverted),
		"spatial_points":     idx.spatial.countPoints(),
		"last_update":        idx.lastUpdate,
		"delta_buffer_size":  len(idx.deltaBuffer),
	}
}

func (qt *QuadTree) countPoints() int {
	count := len(qt.points)
	
	if qt.children[0] != nil {
		for i := 0; i < 4; i++ {
			count += qt.children[i].countPoints()
		}
	}
	
	return count
}