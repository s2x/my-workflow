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

The business_description must be at least 3-5 sentences written in plain, non-technical language that a product manager or stakeholder can understand. Explain the problem being solved, who benefits from it, and what value it delivers. Avoid implementation details here.

The technical_description must be at least 3-5 sentences describing implementation approach, which systems/layers are involved, edge cases to handle, and any integration points.

Return ONLY a valid JSON object with this exact structure (no markdown, no explanation, no code blocks):
{
  "title": "short ticket title",
  "business_description": "3-5 sentence plain-language explanation of business value and problem being solved",
  "technical_description": "3-5 sentence technical implementation details covering approach, affected systems, and edge cases",
  "priority": "Low|Medium|High|Critical",
  "complexity": "simple|moderate|complex",
  "affected_components": ["component1", "component2"]
}`

const refineSystemPrompt = `You are a software ticket writer. Refine the existing ticket description based on the refinement notes.

The business_description must be at least 3-5 sentences written in plain, non-technical language that a product manager or stakeholder can understand. Explain the problem being solved, who benefits from it, and what value it delivers. Avoid implementation details here.

The technical_description must be at least 3-5 sentences describing implementation approach, which systems/layers are involved, edge cases to handle, and any integration points.

Return ONLY a valid JSON object with this exact structure (no markdown, no explanation, no code blocks):
{
  "title": "short ticket title",
  "business_description": "3-5 sentence plain-language explanation of business value and problem being solved",
  "technical_description": "3-5 sentence technical implementation details covering approach, affected systems, and edge cases",
  "priority": "Low|Medium|High|Critical",
  "complexity": "simple|moderate|complex",
  "affected_components": ["component1", "component2"]
}`

func (s *service) buildCommand(ctx context.Context, prompt string) *exec.Cmd {
	if s.opencodeBin != "" {
		return exec.CommandContext(ctx, s.opencodeBin, "run", "--agent", "descriptor", prompt)
	}
	if s.qwenBin != "" {
		return exec.CommandContext(ctx, s.qwenBin, prompt)
	}
	return exec.CommandContext(ctx, "opencode", "run", "--agent", "descriptor", prompt)
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
	timeoutCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	cmd := s.buildCommand(timeoutCtx, prompt)

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if timeoutCtx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("timeout: AI took longer than 120s")
		}
		return "", fmt.Errorf("AI command failed: %w", err)
	}

	return strings.TrimSpace(out.String()), nil
}
