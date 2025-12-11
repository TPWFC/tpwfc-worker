package models

// Person represents a person related to the documentary.
type Person struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Role        string `json:"role"`
	Description string `json:"description"`
	Image       string `json:"image"`
	// TODO: Add more fields as needed
}
