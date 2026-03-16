package domain

import "time"

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
