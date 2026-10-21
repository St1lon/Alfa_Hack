package ingestnews

import "github.com/google/uuid"

// Result — исход приёма новости. Duplicate=true значит, что такая новость уже есть.
type Result struct {
	NewsID    uuid.UUID
	Duplicate bool
}
