package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/piotr-halas/decodo-workflow/internal/models"
)

type AITicketService interface {
	GenerateTicket(ctx context.Context, description string) (*models.GeneratedTicket, error)
	RefineTicket(ctx context.Context, currentDescription string, refinementNotes string) (*models.GeneratedTicket, error)
}

type service struct {
	opencodeBin string
	qwenBin     string
}

func NewAITicketService(opencodeBin, qwenBin string) AITicketService {
	return &service{
		opencodeBin: opencodeBin,
		qwenBin:     qwenBin,
	}
}

const generateSystemPrompt = `You are a software ticket writer. Based on the user description, generate a structured ticket in JSON format.
Return ONLY a valid JSON object with this exact structure (no markdown, no explanation):
{
  "title": "short ticket title",
  "business_description": "business context and value",
  "technical_description": "technical implementation details",
  "priority": "Low|Medium|High|Critical",
  "complexity": "simple|moderate|complex",
  "affected_components": ["component1", "component2"]
}`

const refineSystemPrompt = `You are a software ticket writer. Refine the existing ticket description based on the refinement notes.
Return ONLY a valid JSON object with this exact structure (no markdown, no explanation):
{
  "title": "short ticket title",
  "business_description": "business context and value",
  "technical_description": "technical implementation details",
  "priority": "Low|Medium|High|Critical",
  "complexity": "simple|moderate|complex",
  "affected_components": ["component1", "component2"]
}`

func (s *service) run(prompt string) (string, error) {
	bin := s.qwenBin
	if bin == "" {
		bin = s.opencodeBin
	}
	if bin == "" {
		bin = "qwen"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, "run", "--agent", "descriptor", prompt)

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("timeout: AI took longer than 30s")
		}
		return "", fmt.Errorf("AI command failed: %w", err)
	}

	return strings.TrimSpace(out.String()), nil
}

func extractJSON(output string) string {
	start := strings.Index(output, "{")
	end := strings.LastIndex(output, "}")
	if start == -1 || end == -1 || end <= start {
		return output
	}
	return output[start : end+1]
}

func (s *service) GenerateTicket(ctx context.Context, description string) (*models.GeneratedTicket, error) {
	prompt := generateSystemPrompt + "\n\nUser description:\n" + description

	output, err := s.runWithContext(ctx, prompt)
	if err != nil {
		return nil, err
	}

	jsonStr := extractJSON(output)
	var ticket models.GeneratedTicket
	if err := json.Unmarshal([]byte(jsonStr), &ticket); err != nil {
		return nil, fmt.Errorf("failed to parse AI response as JSON: %w", err)
	}
	return &ticket, nil
}

func (s *service) RefineTicket(ctx context.Context, currentDescription string, refinementNotes string) (*models.GeneratedTicket, error) {
	prompt := refineSystemPrompt + "\n\nCurrent description:\n" + currentDescription + "\n\nRefinement notes:\n" + refinementNotes

	output, err := s.runWithContext(ctx, prompt)
	if err != nil {
		return nil, err
	}

	jsonStr := extractJSON(output)
	var ticket models.GeneratedTicket
	if err := json.Unmarshal([]byte(jsonStr), &ticket); err != nil {
		return nil, fmt.Errorf("failed to parse AI response as JSON: %w", err)
	}
	return &ticket, nil
}

func (s *service) runWithContext(ctx context.Context, prompt string) (string, error) {
	bin := s.qwenBin
	if bin == "" {
		bin = s.opencodeBin
	}
	if bin == "" {
		bin = "qwen"
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, bin, "run", "--agent", "descriptor", prompt)

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if timeoutCtx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("timeout: AI took longer than 30s")
		}
		return "", fmt.Errorf("AI command failed: %w", err)
	}

	return strings.TrimSpace(out.String()), nil
}
