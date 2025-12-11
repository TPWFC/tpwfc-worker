// Package models defines data structures for the crawler and normalizer.
package models

import "time"

// Article represents a news article or document.
type Article struct {
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Author    string    `json:"author"`
	Source    string    `json:"source"`
	URL       string    `json:"url"`
}
