package streaming

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/xuri/excelize/v2"

	"mcp-xlsm-server/internal/models"
)

type ChunkReader struct {
	file    *excelize.File
	chunk   models.Chunk
	buffer  *bytes.Buffer
	encoder *json.Encoder
}

func NewChunkReader(file *excelize.File, chunk models.Chunk) *ChunkReader {
	buffer := &bytes.Buffer{}
	return &ChunkReader{
		file:    file,
		chunk:   chunk,
		buffer:  buffer,
		encoder: json.NewEncoder(buffer),
	}
}

func (s *ChunkReader) StreamChunk(writer io.Writer) error {
	encoder := json.NewEncoder(writer)
	sheetList := s.file.GetSheetList()
	
	// Stream metadata first
	metadata := map[string]interface{}{
		"chunk_id":    s.chunk.ChunkID,
		"sheets_range": s.chunk.SheetsRange,
		"streaming":   true,
	}
	
	if err := encoder.Encode(map[string]interface{}{
		"type": "metadata",
		"data": metadata,
	}); err != nil {
		return fmt.Errorf("failed to encode metadata: %w", err)
	}
	
	// Flush if possible
	if flusher, ok := writer.(http.Flusher); ok {
		flusher.Flush()
	}
	
	// Stream each sheet in the chunk
	for sheetIdx := s.chunk.SheetsRange[0]; sheetIdx <= s.chunk.SheetsRange[1] && sheetIdx < len(sheetList); sheetIdx++ {
		sheetName := sheetList[sheetIdx]
		
		if err := s.streamSheet(encoder, sheetName, sheetIdx); err != nil {
			return fmt.Errorf("failed to stream sheet %s: %w", sheetName, err)
		}
		
		// Flush after each sheet
		if flusher, ok := writer.(http.Flusher); ok {
			flusher.Flush()
		}
	}
	
	// Send completion marker
	if err := encoder.Encode(map[string]interface{}{
		"type": "complete",
		"chunk_id": s.chunk.ChunkID,
	}); err != nil {
		return fmt.Errorf("failed to encode completion: %w", err)
	}
	
	return nil
}

func (s *ChunkReader) streamSheet(encoder *json.Encoder, sheetName string, sheetIdx int) error {
	rows, err := s.file.GetRows(sheetName)
	if err != nil {
		return err
	}
	
	// Send sheet header
	sheetInfo := map[string]interface{}{
		"sheet_index": sheetIdx,
		"sheet_name":  sheetName,
		"total_rows":  len(rows),
	}
	
	if err := encoder.Encode(map[string]interface{}{
		"type": "sheet_start",
		"data": sheetInfo,
	}); err != nil {
		return err
	}
	
	// Stream rows in batches
	batchSize := 100
	batch := make([][]string, 0, batchSize)
	
	for i, row := range rows {
		batch = append(batch, row)
		
		// Send batch when full or at end
		if len(batch) >= batchSize || i == len(rows)-1 {
			rowData := map[string]interface{}{
				"sheet_index": sheetIdx,
				"sheet_name":  sheetName,
				"start_row":   i - len(batch) + 1,
				"rows":        batch,
			}
			
			if err := encoder.Encode(map[string]interface{}{
				"type": "rows",
				"data": rowData,
			}); err != nil {
				return err
			}
			
			// Reset batch
			batch = batch[:0]
		}
	}
	
	// Send sheet completion
	if err := encoder.Encode(map[string]interface{}{
		"type": "sheet_complete",
		"sheet_index": sheetIdx,
		"sheet_name":  sheetName,
	}); err != nil {
		return err
	}
	
	return nil
}

func (s *ChunkReader) GetBuffer() *bytes.Buffer {
	return s.buffer
}

// StreamingResponse manages streaming response for large data
type StreamingResponse struct {
	writer  io.Writer
	encoder *json.Encoder
}

func NewStreamingResponse(writer io.Writer) *StreamingResponse {
	return &StreamingResponse{
		writer:  writer,
		encoder: json.NewEncoder(writer),
	}
}

func (sr *StreamingResponse) WriteMetadata(metadata interface{}) error {
	return sr.encoder.Encode(map[string]interface{}{
		"type": "metadata",
		"data": metadata,
	})
}

func (sr *StreamingResponse) WriteData(dataType string, data interface{}) error {
	return sr.encoder.Encode(map[string]interface{}{
		"type": dataType,
		"data": data,
	})
}

func (sr *StreamingResponse) WriteError(err error) error {
	return sr.encoder.Encode(map[string]interface{}{
		"type": "error",
		"error": err.Error(),
	})
}

func (sr *StreamingResponse) WriteComplete() error {
	return sr.encoder.Encode(map[string]interface{}{
		"type": "complete",
	})
}

func (sr *StreamingResponse) Flush() {
	if flusher, ok := sr.writer.(http.Flusher); ok {
		flusher.Flush()
	}
}

// WindowedReader for reading data in windows
type WindowedReader struct {
	file     *excelize.File
	window   models.Window
	sheetName string
}

func NewWindowedReader(file *excelize.File, sheetName string, window models.Window) *WindowedReader {
	return &WindowedReader{
		file:      file,
		window:    window,
		sheetName: sheetName,
	}
}

func (wr *WindowedReader) ReadWindow() ([][]string, error) {
	rows, err := wr.file.GetRows(wr.sheetName)
	if err != nil {
		return nil, err
	}
	
	// Extract the specified window
	var windowData [][]string
	
	for rowIdx := wr.window.StartRow; rowIdx <= wr.window.EndRow && rowIdx < len(rows); rowIdx++ {
		if rowIdx < len(rows) {
			row := rows[rowIdx]
			
			// Extract columns within window
			var windowRow []string
			for colIdx := wr.window.StartCol; colIdx <= wr.window.EndCol && colIdx < len(row); colIdx++ {
				if colIdx < len(row) {
					windowRow = append(windowRow, row[colIdx])
				} else {
					windowRow = append(windowRow, "")
				}
			}
			
			windowData = append(windowData, windowRow)
		}
	}
	
	return windowData, nil
}

func (wr *WindowedReader) StreamWindow(writer io.Writer) error {
	windowData, err := wr.ReadWindow()
	if err != nil {
		return err
	}
	
	encoder := json.NewEncoder(writer)
	
	// Send window metadata
	metadata := map[string]interface{}{
		"sheet_name": wr.sheetName,
		"window":     wr.window,
		"rows":       len(windowData),
	}
	
	if err := encoder.Encode(map[string]interface{}{
		"type": "window_start",
		"data": metadata,
	}); err != nil {
		return err
	}
	
	// Stream data in smaller batches
	batchSize := 50
	for i := 0; i < len(windowData); i += batchSize {
		end := i + batchSize
		if end > len(windowData) {
			end = len(windowData)
		}
		
		batch := windowData[i:end]
		
		if err := encoder.Encode(map[string]interface{}{
			"type": "window_data",
			"data": map[string]interface{}{
				"start_row": i + wr.window.StartRow,
				"rows":      batch,
			},
		}); err != nil {
			return err
		}
		
		// Flush periodically
		if flusher, ok := writer.(http.Flusher); ok {
			flusher.Flush()
		}
	}
	
	// Send completion
	return encoder.Encode(map[string]interface{}{
		"type": "window_complete",
	})
}