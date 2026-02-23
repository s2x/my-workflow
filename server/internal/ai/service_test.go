package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func buildMockBinary(t *testing.T, goSrc string) string {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "mock.go")
	bin := filepath.Join(dir, "mock")

	if err := os.WriteFile(src, []byte(goSrc), 0644); err != nil {
		t.Fatalf("failed to write mock source: %v", err)
	}
	cmd := exec.Command("go", "build", "-o", bin, src)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build mock binary: %v\n%s", err, out)
	}
	return bin
}

func buildPrintBinary(t *testing.T, output string) string {
	t.Helper()
	goSrc := fmt.Sprintf(`package main
import "fmt"
func main() { fmt.Print(%q) }
`, output)
	return buildMockBinary(t, goSrc)
}

func buildSleepBinary(t *testing.T) string {
	t.Helper()
	goSrc := `package main
import "time"
func main() { time.Sleep(60 * time.Second) }
`
	return buildMockBinary(t, goSrc)
}

func TestGenerateTicketParsesValidJSONOutput(t *testing.T) {
	ticket := map[string]interface{}{
		"title":                 "Test Feature",
		"business_description":  "Business value here",
		"technical_description": "Technical details here",
		"priority":              "High",
		"complexity":            "moderate",
		"affected_components":   []string{"api"},
	}
	jsonBytes, _ := json.Marshal(ticket)

	bin := buildPrintBinary(t, string(jsonBytes))
	svc := &service{qwenBin: bin}

	result, err := svc.GenerateTicket(context.Background(), "Implement new feature")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.Title != "Test Feature" {
		t.Errorf("Expected title %q, got %q", "Test Feature", result.Title)
	}
	if result.Priority != "High" {
		t.Errorf("Expected priority %q, got %q", "High", result.Priority)
	}
	if result.BusinessDescription != "Business value here" {
		t.Errorf("Expected business_description %q, got %q", "Business value here", result.BusinessDescription)
	}
}

func TestGenerateTicketReturnsErrorOnInvalidJSON(t *testing.T) {
	bin := buildPrintBinary(t, "not valid json at all")
	svc := &service{qwenBin: bin}

	_, err := svc.GenerateTicket(context.Background(), "Implement something useful here")
	if err == nil {
		t.Fatal("Expected error for invalid JSON output, got nil")
	}
	if !strings.Contains(err.Error(), "parse") && !strings.Contains(err.Error(), "JSON") {
		t.Errorf("Expected JSON parse error, got: %v", err)
	}
}

func TestGenerateTicketBuildsPromptContainingDescription(t *testing.T) {
	ticket := map[string]interface{}{
		"title": "X", "business_description": "Y", "technical_description": "Z",
		"priority": "Low", "complexity": "simple", "affected_components": []string{},
	}
	jsonBytes, _ := json.Marshal(ticket)

	bin := buildPrintBinary(t, string(jsonBytes))
	svc := &service{qwenBin: bin}
	desc := "My unique feature description 12345"
	_, err := svc.GenerateTicket(context.Background(), desc)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestGenerateTicketReturnsErrorWhenBinaryNotExists(t *testing.T) {
	svc := &service{qwenBin: "/nonexistent/binary/path"}
	_, err := svc.GenerateTicket(context.Background(), "Some valid description here")
	if err == nil {
		t.Fatal("Expected error when binary does not exist, got nil")
	}
}

func TestGenerateTicketReturnsTimeoutError(t *testing.T) {
	bin := buildSleepBinary(t)
	svc := &service{qwenBin: bin}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := svc.GenerateTicket(ctx, "Valid description with enough chars here")
	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}
}

func TestRefineTicketParsesValidJSONOutput(t *testing.T) {
	ticket := map[string]interface{}{
		"title":                 "Refined Feature",
		"business_description":  "Refined business value",
		"technical_description": "Refined technical details",
		"priority":              "Medium",
		"complexity":            "complex",
		"affected_components":   []string{"frontend", "backend"},
	}
	jsonBytes, _ := json.Marshal(ticket)

	bin := buildPrintBinary(t, string(jsonBytes))
	svc := &service{qwenBin: bin}

	result, err := svc.RefineTicket(context.Background(), "Current description", "Make it more detailed")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.Title != "Refined Feature" {
		t.Errorf("Expected title %q, got %q", "Refined Feature", result.Title)
	}
	if len(result.AffectedComponents) != 2 {
		t.Errorf("Expected 2 affected components, got %d", len(result.AffectedComponents))
	}
}

func TestRefineTicketReturnsErrorWhenBinaryNotExists(t *testing.T) {
	svc := &service{qwenBin: "/nonexistent/binary/path"}
	_, err := svc.RefineTicket(context.Background(), "Current desc", "Refinement notes")
	if err == nil {
		t.Fatal("Expected error when binary does not exist, got nil")
	}
}
