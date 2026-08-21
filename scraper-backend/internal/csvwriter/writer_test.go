package csvwriter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCSVGenerationIncludesTimestampAndNestedJSON(t *testing.T) {
	dir := t.TempDir()
	writer := New(dir)
	records := []map[string]any{
		{
			"name":  "Product A",
			"price": "$20",
			"tags":  []any{"sale", "new"},
		},
		{
			"name":  "Product B",
			"price": "$30",
			"url":   "https://example.com/b",
		},
	}

	filePath, err := writer.Write(records)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if !strings.HasSuffix(filePath, ".csv") {
		t.Fatalf("file path %q does not end with .csv", filePath)
	}

	content, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		t.Fatalf("read generated csv: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "timestamp") {
		t.Fatalf("csv missing timestamp header: %s", text)
	}
	if !strings.Contains(text, "sale") || !strings.Contains(text, "new") {
		t.Fatalf("nested JSON was not encoded sensibly: %s", text)
	}
}

func TestCSVGenerationHandlesEmptyRecords(t *testing.T) {
	dir := t.TempDir()
	writer := New(dir)
	filePath, err := writer.Write(nil)
	if err != nil {
		t.Fatalf("Write(nil) error = %v", err)
	}
	content, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if !strings.Contains(string(content), "timestamp") {
		t.Fatalf("expected timestamp header even with no rows: %s", string(content))
	}
}
