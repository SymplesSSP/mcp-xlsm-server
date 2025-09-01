package cursor

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"mcp-xlsm-server/internal/models"
)

type Manager struct {
	version int
}

func NewManager() *Manager {
	return &Manager{
		version: models.CURSOR_VERSION,
	}
}

func (m *Manager) GenerateCursor(data models.CursorData) string {
	data.Version = m.version
	data.Timestamp = time.Now().Unix()
	
	jsonData, err := json.Marshal(data)
	if err != nil {
		// In production, handle this error properly
		return ""
	}
	
	return base64.URLEncoding.EncodeToString(jsonData)
}

func (m *Manager) ParseCursor(cursor string) (*models.CursorData, error) {
	if cursor == "" {
		return nil, fmt.Errorf("empty cursor")
	}
	
	decoded, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor format: %w", err)
	}
	
	var data models.CursorData
	if err := json.Unmarshal(decoded, &data); err != nil {
		return nil, fmt.Errorf("cursor parsing failed: %w", err)
	}
	
	// Validation
	if data.Version != m.version {
		return nil, fmt.Errorf("cursor version mismatch: expected %d, got %d", 
			m.version, data.Version)
	}
	
	// Check if cursor is not too old (e.g., 24 hours)
	maxAge := int64(24 * 60 * 60) // 24 hours in seconds
	if time.Now().Unix()-data.Timestamp > maxAge {
		return nil, fmt.Errorf("cursor expired")
	}
	
	return &data, nil
}

func (m *Manager) CreateChunkCursor(chunkID string, offset int64, checksum string, window *models.Window) string {
	data := models.CursorData{
		ChunkID:    chunkID,
		Offset:     offset,
		Checksum:   checksum,
		WindowInfo: window,
	}
	
	return m.GenerateCursor(data)
}

func (m *Manager) CreateNavigationCursor(chunkID string, sheetIndex int, checksum string) string {
	data := models.CursorData{
		ChunkID:  chunkID,
		Offset:   int64(sheetIndex),
		Checksum: checksum,
	}
	
	return m.GenerateCursor(data)
}

func (m *Manager) CreateQueryCursor(query string, offset int64, checksum string, window *models.Window) string {
	data := models.CursorData{
		ChunkID:    query, // Using ChunkID field to store query
		Offset:     offset,
		Checksum:   checksum,
		WindowInfo: window,
	}
	
	return m.GenerateCursor(data)
}

func (m *Manager) ValidateChecksum(cursor string, expectedChecksum string) error {
	data, err := m.ParseCursor(cursor)
	if err != nil {
		return err
	}
	
	if data.Checksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: cursor checksum %s != expected %s", 
			data.Checksum, expectedChecksum)
	}
	
	return nil
}

func (m *Manager) ExtractChunkID(cursor string) (string, error) {
	data, err := m.ParseCursor(cursor)
	if err != nil {
		return "", err
	}
	
	return data.ChunkID, nil
}

func (m *Manager) ExtractOffset(cursor string) (int64, error) {
	data, err := m.ParseCursor(cursor)
	if err != nil {
		return 0, err
	}
	
	return data.Offset, nil
}

func (m *Manager) ExtractWindow(cursor string) (*models.Window, error) {
	data, err := m.ParseCursor(cursor)
	if err != nil {
		return nil, err
	}
	
	return data.WindowInfo, nil
}