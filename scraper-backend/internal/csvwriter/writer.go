package csvwriter

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// CSVWriter serializes []map[string]any records into a CSV file with a timestamp column.
type CSVWriter struct {
	OutputDir string
}

func New(outputDir string) *CSVWriter {
	return &CSVWriter{OutputDir: outputDir}
}

func (w *CSVWriter) Write(records []map[string]any) (string, error) {
	if w == nil {
		return "", fmt.Errorf("csv writer is nil")
	}
	if err := os.MkdirAll(w.OutputDir, 0o755); err != nil {
		return "", fmt.Errorf("create output directory: %w", err)
	}

	timestamp := time.Now().Format(time.RFC3339)
	fileName := fmt.Sprintf("scrape_%s.csv", sanitizeTimestampForFile(timestamp))
	filePath := filepath.Join(w.OutputDir, fileName)

	headers, rows := buildCSVData(records, timestamp)
	file, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("create csv file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	if err := writer.Write(headers); err != nil {
		return "", fmt.Errorf("write csv header: %w", err)
	}
	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			return "", fmt.Errorf("write csv row: %w", err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", fmt.Errorf("flush csv file: %w", err)
	}

	return filepath.ToSlash(filePath), nil
}

func buildCSVData(records []map[string]any, timestampValue string) ([]string, [][]string) {
	headersSet := map[string]struct{}{"timestamp": {}}
	for _, record := range records {
		for key := range record {
			headersSet[key] = struct{}{}
		}
	}

	headers := make([]string, 0, len(headersSet))
	for key := range headersSet {
		headers = append(headers, key)
	}
	sort.Strings(headers)
	if headers[0] != "timestamp" {
		headers = reorderHeaders(headers)
	}

	rows := make([][]string, 0, len(records))
	for _, record := range records {
		row := make([]string, len(headers))
		for idx, key := range headers {
			if key == "timestamp" {
				row[idx] = timestampValue
				continue
			}
			row[idx] = normalizeCSVValue(record[key])
		}
		rows = append(rows, row)
	}

	if len(headers) == 0 {
		headers = []string{"timestamp"}
		rows = [][]string{{timestampValue}}
	}
	return headers, rows
}

func reorderHeaders(headers []string) []string {
	ordered := make([]string, 0, len(headers))
	ordered = append(ordered, "timestamp")
	for _, header := range headers {
		if header == "timestamp" {
			continue
		}
		ordered = append(ordered, header)
	}
	return ordered
}

func normalizeCSVValue(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case []byte:
		return string(v)
	case map[string]any:
		b, err := json.Marshal(v)
		if err == nil {
			return string(b)
		}
		return fmt.Sprintf("%v", v)
	case []any:
		b, err := json.Marshal(v)
		if err == nil {
			return string(b)
		}
		return fmt.Sprintf("%v", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func sanitizeTimestampForFile(ts string) string {
	clean := strings.ReplaceAll(ts, ":", "-")
	clean = strings.ReplaceAll(clean, "+", "")
	clean = strings.ReplaceAll(clean, "Z", "Z")
	return clean
}
