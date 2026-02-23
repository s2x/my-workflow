package workflow

import (
	"fmt"
	"strings"

	"github.com/piotr-halas/decodo-workflow/internal/models"
)

type PromptBuilder struct {
	baseBranch string
}

func NewPromptBuilder(baseBranch string) *PromptBuilder {
	return &PromptBuilder{baseBranch: baseBranch}
}

func (pb *PromptBuilder) BuildDescribePrompt(ticket *models.Ticket) string {
	var b strings.Builder
	b.WriteString("Przeanalizuj poniższy ticket i stwórz specyfikację techniczną.\n\n")
	b.WriteString(fmt.Sprintf("## Ticket: %s\n", ticket.JiraKey))
	b.WriteString(fmt.Sprintf("**Summary**: %s\n", ticket.Summary))
	b.WriteString(fmt.Sprintf("**Type**: %s\n", ticket.TicketType))
	b.WriteString(fmt.Sprintf("**Priority**: %s\n", ticket.Priority))

	if ticket.Labels != "" {
		b.WriteString(fmt.Sprintf("**Labels**: %s\n", ticket.Labels))
	}

	b.WriteString(fmt.Sprintf("\n**Description**:\n%s\n", ticket.Description))

	if ticket.AcceptanceCriteria != "" {
		b.WriteString(fmt.Sprintf("\n**Acceptance Criteria**:\n%s\n", ticket.AcceptanceCriteria))
	}

	b.WriteString(`
## Instrukcje
1. Przeanalizuj strukturę repozytorium
2. Zidentyfikuj pliki do zmiany/utworzenia
3. Oceń czy zadanie powinno być rozbite na mniejsze podzadania

## Format odpowiedzi
Odpowiedz WYŁĄCZNIE w formacie JSON:
{
  "summary": "krótki opis co trzeba zrobić",
  "complexity": "simple|medium|complex",
  "subtasks": [
    {
      "title": "tytuł podzadania",
      "description": "szczegółowy opis co zrobić",
      "files": ["ścieżki/do/plików"],
      "type": "code|test"
    }
  ],
  "code_changes": [
    {
      "file": "ścieżka/do/pliku",
      "action": "create|modify|delete",
      "description": "co zmienić"
    }
  ],
  "test_spec": [
    {
      "file": "ścieżka/do/testu",
      "cases": ["opis przypadku testowego"]
    }
  ]
}
`)
	return b.String()
}

func (pb *PromptBuilder) BuildCodePrompt(ticket *models.Ticket, spec string, subtaskDesc string) string {
	branchName := fmt.Sprintf("feature/%s", strings.ToLower(ticket.JiraKey))
	var b strings.Builder
	b.WriteString("Zaimplementuj poniższe zmiany w kodzie.\n\n")
	b.WriteString(fmt.Sprintf("## Ticket: %s - %s\n\n", ticket.JiraKey, ticket.Summary))
	b.WriteString(fmt.Sprintf("## Branch: %s\n\n", branchName))

	if subtaskDesc != "" {
		b.WriteString(fmt.Sprintf("## Podzadanie\n%s\n\n", subtaskDesc))
	}

	b.WriteString(fmt.Sprintf("## Specyfikacja techniczna\n%s\n\n", spec))
	b.WriteString("## Instrukcje\n")
	b.WriteString(fmt.Sprintf("1. Sprawdź na jakim branchu jesteś: `git branch --show-current`\n"))
	b.WriteString(fmt.Sprintf("2. Jeżeli nie jesteś na branchu `%s`, przełącz się: `git checkout %s`\n", branchName, branchName))
	b.WriteString("3. Zaimplementuj wymagane zmiany\n")
	b.WriteString("4. Napisz unit testy dla swoich zmian\n")
	b.WriteString("5. Uruchom testy przed zakończeniem\n")
	b.WriteString("6. Commituj zmiany: `git add . && git commit -m \"<type>(<scope>): <description>\"`\n")
	b.WriteString("\nNIE twórz nowego brancha ani nie pushuj - branch jest zarządzany przez system.\n")
	return b.String()
}

func (pb *PromptBuilder) BuildTestPrompt(ticket *models.Ticket, spec string, branchName string) string {
	var b strings.Builder
	b.WriteString("Przetestuj zmiany na podanym branchu.\n\n")
	b.WriteString(fmt.Sprintf("## Ticket: %s - %s\n", ticket.JiraKey, ticket.Summary))
	b.WriteString(fmt.Sprintf("## Branch: %s\n\n", branchName))
	b.WriteString(fmt.Sprintf("## Specyfikacja testów\n%s\n\n", spec))
	b.WriteString("## Instrukcje\n")
	b.WriteString(fmt.Sprintf("1. Sprawdź na jakim branchu jesteś: `git branch --show-current`\n"))
	b.WriteString(fmt.Sprintf("2. Jeżeli nie jesteś na branchu `%s`, przełącz się: `git checkout %s`\n", branchName, branchName))
	b.WriteString("3. Uruchom istniejące testy\n")
	b.WriteString("4. Sprawdź pokrycie kodu testami\n")
	b.WriteString("5. Napisz brakujące testy (edge cases, error handling)\n")
	b.WriteString("6. Commituj nowe testy: `git add . && git commit -m \"test: <description>\"`\n\n")
	b.WriteString("Odpowiedz w JSON:\n")
	b.WriteString(`{
  "result": "PASS|FAIL",
  "tests_run": 0,
  "tests_passed": 0,
  "tests_failed": 0,
  "coverage": "XX%",
  "issues": ["opis problemu"],
  "new_tests_added": 0
}
`)
	return b.String()
}

func (pb *PromptBuilder) BuildReviewPrompt(ticket *models.Ticket, branchName string) string {
	var b strings.Builder
	b.WriteString("Przeprowadź code review zmian na podanym branchu.\n\n")
	b.WriteString(fmt.Sprintf("## Ticket: %s - %s\n", ticket.JiraKey, ticket.Summary))
	b.WriteString(fmt.Sprintf("## Branch: %s\n\n", branchName))
	b.WriteString(fmt.Sprintf("## Instrukcje\n"))
	b.WriteString(fmt.Sprintf("1. Sprawdź na jakim branchu jesteś: `git branch --show-current`\n"))
	b.WriteString(fmt.Sprintf("2. Jeżeli nie jesteś na branchu `%s`, przełącz się: `git checkout %s`\n", branchName, branchName))
	b.WriteString(fmt.Sprintf("3. Sprawdź diff: git diff %s...%s\n", pb.baseBranch, branchName))
	b.WriteString("4. Oceń jakość kodu, czytelność, konwencje\n")
	b.WriteString("5. Sprawdź bezpieczeństwo (brak wycieków danych, SQL injection, etc.)\n")
	b.WriteString("6. Sprawdź performance (n+1 queries, memory leaks, etc.)\n")
	b.WriteString("7. Sprawdź testy (pokrycie, sensowność)\n\n")
	b.WriteString("Odpowiedz w JSON:\n")
	b.WriteString(`{
  "result": "APPROVED|CHANGES_REQUESTED",
  "score": 0,
  "comments": [
    {
      "file": "ścieżka",
      "line": 0,
      "severity": "critical|warning|suggestion",
      "message": "opis"
    }
  ],
  "summary": "podsumowanie review"
}
`)
	return b.String()
}

func (pb *PromptBuilder) BuildRejectFixPrompt(ticket *models.Ticket, spec string, branchName string, rejectComment string) string {
	var b strings.Builder
	b.WriteString("Napraw problemy zgłoszone przez użytkownika podczas odrzucenia wdrożenia.\n\n")
	b.WriteString(fmt.Sprintf("## Ticket: %s - %s\n", ticket.JiraKey, ticket.Summary))
	b.WriteString(fmt.Sprintf("## Branch: %s\n\n", branchName))
	b.WriteString(fmt.Sprintf("## Powód odrzucenia przez użytkownika\n%s\n\n", rejectComment))
	b.WriteString(fmt.Sprintf("## Oryginalna specyfikacja\n%s\n\n", spec))
	b.WriteString("## Instrukcje\n")
	b.WriteString(fmt.Sprintf("1. Sprawdź na jakim branchu jesteś: `git branch --show-current`\n"))
	b.WriteString(fmt.Sprintf("2. Jeżeli nie jesteś na branchu `%s`, przełącz się: `git checkout %s`\n", branchName, branchName))
	b.WriteString("3. Napraw problemy opisane przez użytkownika w powodzie odrzucenia\n")
	b.WriteString("4. Uruchom testy\n")
	b.WriteString("5. Commituj poprawki: `git add . && git commit -m \"fix: <description>\"`\n")
	return b.String()
}

func (pb *PromptBuilder) BuildFixPrompt(ticket *models.Ticket, spec string, branchName string, issues string) string {
	var b strings.Builder
	b.WriteString("Napraw problemy znalezione w poprzedniej iteracji.\n\n")
	b.WriteString(fmt.Sprintf("## Ticket: %s - %s\n", ticket.JiraKey, ticket.Summary))
	b.WriteString(fmt.Sprintf("## Branch: %s\n\n", branchName))
	b.WriteString(fmt.Sprintf("## Problemy do naprawienia\n%s\n\n", issues))
	b.WriteString(fmt.Sprintf("## Oryginalna specyfikacja\n%s\n\n", spec))
	b.WriteString("## Instrukcje\n")
	b.WriteString(fmt.Sprintf("1. Sprawdź na jakim branchu jesteś: `git branch --show-current`\n"))
	b.WriteString(fmt.Sprintf("2. Jeżeli nie jesteś na branchu `%s`, przełącz się: `git checkout %s`\n", branchName, branchName))
	b.WriteString("3. Napraw wymienione problemy\n")
	b.WriteString("4. Uruchom testy\n")
	b.WriteString("5. Commituj poprawki: `git add . && git commit -m \"fix: <description>\"`\n")
	return b.String()
}
