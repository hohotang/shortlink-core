// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.29.0

package db

import (
	"database/sql"
)

type Url struct {
	ShortID      string       `json:"short_id"`
	OriginalUrl  string       `json:"original_url"`
	CreatedAt    sql.NullTime `json:"created_at"`
	LastAccessed sql.NullTime `json:"last_accessed"`
}
