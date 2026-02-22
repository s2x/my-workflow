package jira

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/piotr-halas/decodo-workflow/internal/models"
)

type Client struct {
	baseURL    string
	email      string
	token      string
	httpClient *http.Client
	logger     *slog.Logger
}

func NewClient(baseURL, email, token string, logger *slog.Logger) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		email:      email,
		token:      token,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		logger:     logger,
	}
}

func (c *Client) FetchMyTickets() ([]models.Ticket, error) {
	jql := "assignee = currentUser() AND status != Done ORDER BY updated DESC"
	url := fmt.Sprintf("%s/rest/api/3/search?jql=%s&maxResults=50", c.baseURL, jql)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.SetBasicAuth(c.email, c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jira API returned %d: %s", resp.StatusCode, string(body))
	}

	var searchResp SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	c.logger.Info("fetched tickets from Jira", "count", len(searchResp.Issues))

	var tickets []models.Ticket
	for _, issue := range searchResp.Issues {
		raw, _ := json.Marshal(issue)

		assignee := ""
		if issue.Fields.Assignee != nil {
			assignee = issue.Fields.Assignee.DisplayName
		}

		tickets = append(tickets, models.Ticket{
			JiraKey:            issue.Key,
			Summary:            issue.Fields.Summary,
			Description:        issue.Fields.Description.ToPlainText(),
			Status:             issue.Fields.Status.Name,
			Priority:           issue.Fields.Priority.Name,
			Assignee:           assignee,
			Labels:             strings.Join(issue.Fields.Labels, ","),
			AcceptanceCriteria: extractAC(issue.Fields.Description),
			TicketType:         issue.Fields.IssueType.Name,
			ProjectKey:         issue.Fields.Project.Key,
			RawJSON:            string(raw),
			JiraUpdatedAt:      ParseJiraTime(issue.Fields.Updated),
		})
	}

	return tickets, nil
}

func extractAC(desc *ADF) string {
	if desc == nil {
		return ""
	}
	text := desc.ToPlainText()
	lower := strings.ToLower(text)
	idx := strings.Index(lower, "acceptance criteria")
	if idx == -1 {
		return ""
	}
	return strings.TrimSpace(text[idx:])
}
