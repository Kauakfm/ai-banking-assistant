package domain

import "time"

type AssistantRequest struct {
	CustomerID string `json:"customer_id"`
	Prompt     string `json:"prompt"`
}

type AssistantResponse struct {
	CustomerID string         `json:"customer_id"`
	Prompt     string         `json:"prompt"`
	Response   string         `json:"response"`
	Cached     bool           `json:"cached"`
	DurationMs int64          `json:"duration_ms"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type AgentRequest struct {
	CustomerID string `json:"customer_id"`
	Query      string `json:"query"`
}

type AgentResponse struct {
	CustomerID string         `json:"customer_id"`
	Query      string         `json:"query"`
	Response   string         `json:"response"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type Profile struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Segment   string    `json:"segment"`
	AccountID string    `json:"account_id"`
	CreatedAt time.Time `json:"created_at"`
}

type Transaction struct {
	ID          string    `json:"id"`
	Date        time.Time `json:"date"`
	Description string    `json:"description"`
	Amount      float64   `json:"amount"`
	Category    string    `json:"category"`
	Type        string    `json:"type"`
}
