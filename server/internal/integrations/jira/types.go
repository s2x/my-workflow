package jira

import "time"

type SearchResponse struct {
	Total      int     `json:"total"`
	MaxResults int     `json:"maxResults"`
	Issues     []Issue `json:"issues"`
}

type Issue struct {
	Key    string      `json:"key"`
	Fields IssueFields `json:"fields"`
}

type IssueFields struct {
	Summary     string       `json:"summary"`
	Description *ADF         `json:"description"`
	Status      NamedField   `json:"status"`
	Priority    NamedField   `json:"priority"`
	Assignee    *UserField   `json:"assignee"`
	Labels      []string     `json:"labels"`
	IssueType   NamedField   `json:"issuetype"`
	Project     ProjectField `json:"project"`
	Updated     string       `json:"updated"`
}

type NamedField struct {
	Name string `json:"name"`
}

type UserField struct {
	DisplayName  string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
}

type ProjectField struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

type ADF struct {
	Type    string    `json:"type"`
	Content []ADFNode `json:"content"`
}

type ADFNode struct {
	Type    string    `json:"type"`
	Text    string    `json:"text,omitempty"`
	Content []ADFNode `json:"content,omitempty"`
}

func (a *ADF) ToPlainText() string {
	if a == nil {
		return ""
	}
	return extractText(a.Content)
}

func extractText(nodes []ADFNode) string {
	var result string
	for _, node := range nodes {
		if node.Text != "" {
			result += node.Text
		}
		if len(node.Content) > 0 {
			result += extractText(node.Content)
		}
		if node.Type == "paragraph" || node.Type == "heading" {
			result += "\n"
		}
	}
	return result
}

func ParseJiraTime(s string) time.Time {
	t, err := time.Parse("2006-01-02T15:04:05.000-0700", s)
	if err != nil {
		t, _ = time.Parse(time.RFC3339, s)
	}
	return t
}
