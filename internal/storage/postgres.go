package storage

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

// PostgresStorage implements URLStorage with PostgreSQL
type PostgresStorage struct {
	db *sql.DB
}

// NewPostgresStorage creates a new PostgresStorage instance
func NewPostgresStorage(connStr string) (*PostgresStorage, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	// Create urls table if it doesn't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS urls (
			short_id VARCHAR(255) PRIMARY KEY,
			original_url TEXT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			last_accessed TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to create urls table: %w", err)
	}

	// Create unique index on original_url for reverse lookups
	_, err = db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_urls_original_url ON urls (original_url)
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to create unique index on original_url: %w", err)
	}

	// Create index on created_at
	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_urls_created_at ON urls (created_at)
	`)
	if err != nil {
		log.Printf("Warning: failed to create index on created_at: %v", err)
	}

	// Create index on last_accessed
	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_urls_last_accessed ON urls (last_accessed)
	`)
	if err != nil {
		log.Printf("Warning: failed to create index on last_accessed: %v", err)
	}

	return &PostgresStorage{db: db}, nil
}

// FindShortIDByURL checks if a URL already has a short ID
func (s *PostgresStorage) FindShortIDByURL(originalURL string) (string, error) {
	var shortID string
	err := s.db.QueryRow("SELECT short_id FROM urls WHERE original_url = $1", originalURL).Scan(&shortID)

	if err != nil {
		if err == sql.ErrNoRows {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("failed to query for existing URL: %w", err)
	}

	return shortID, nil
}

// Store implements URLStorage.Store
func (s *PostgresStorage) Store(originalURL string) (string, error) {
	if originalURL == "" {
		return "", ErrInvalidURL
	}

	// First check if this URL already exists
	shortID, err := s.FindShortIDByURL(originalURL)
	if err == nil {
		// URL already exists, return the existing short ID
		return shortID, nil
	} else if err != ErrNotFound {
		// An error other than "not found" occurred
		return "", err
	}

	// URL doesn't exist yet, but we can't generate a new ID here
	return "", fmt.Errorf("postgres storage requires specifying short ID, use StoreWithID instead")
}

// StoreWithID stores a URL with a specific ID
func (s *PostgresStorage) StoreWithID(shortID string, originalURL string) error {
	if originalURL == "" {
		return ErrInvalidURL
	}

	// Insert or update the URL using ON CONFLICT
	_, err := s.db.Exec(
		`INSERT INTO urls (short_id, original_url) 
		VALUES ($1, $2) 
		ON CONFLICT (short_id) DO UPDATE 
		SET original_url = $2, last_accessed = NOW()`,
		shortID, originalURL,
	)

	if err != nil {
		// Check for unique constraint violation on original_url
		// This would happen if the original_url already exists with a different short_id
		if isPgUniqueViolation(err) {
			// Get the existing short_id for this URL
			existingShortID, findErr := s.FindShortIDByURL(originalURL)
			if findErr != nil {
				return fmt.Errorf("failed to handle unique violation: %w", findErr)
			}

			// We have a conflict - we're trying to assign this URL to a new short_id
			// but it already has a different short_id

			// We have two options:
			// 1. Return the existing short_id (URL already shortened)
			// 2. Update the URL to use the new short_id (override)

			// In this implementation, we'll use option 2 - override the mapping

			// Start a transaction for consistency
			tx, err := s.db.Begin()
			if err != nil {
				return fmt.Errorf("failed to begin transaction: %w", err)
			}

			// Delete the existing record for this original_url
			_, err = tx.Exec("DELETE FROM urls WHERE short_id = $1", existingShortID)
			if err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to delete existing record: %w", err)
			}

			// Now insert the new record
			_, err = tx.Exec(
				"INSERT INTO urls (short_id, original_url) VALUES ($1, $2)",
				shortID, originalURL,
			)
			if err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to insert new record: %w", err)
			}

			// Commit the transaction
			if err = tx.Commit(); err != nil {
				return fmt.Errorf("failed to commit transaction: %w", err)
			}

			return nil
		}

		return fmt.Errorf("failed to store URL: %w", err)
	}

	return nil
}

// Get implements URLStorage.Get
func (s *PostgresStorage) Get(shortID string) (string, error) {
	var originalURL string

	// Query the URL and update last_accessed
	err := s.db.QueryRow(
		"UPDATE urls SET last_accessed = NOW() WHERE short_id = $1 RETURNING original_url",
		shortID,
	).Scan(&originalURL)

	if err != nil {
		if err == sql.ErrNoRows {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("failed to get URL: %w", err)
	}

	return originalURL, nil
}

// Helper function to check if an error is a PostgreSQL unique constraint violation
func isPgUniqueViolation(err error) bool {
	// This is a simplified check - in a real implementation, you'd want to use
	// github.com/lib/pq's error type assertions to check more accurately
	return err != nil && (err.Error() == "pq: duplicate key value violates unique constraint" ||
		err.Error() == "ERROR: duplicate key value violates unique constraint" ||
		err.Error() == "duplicate key value violates unique constraint" ||
		err.Error() == "pq: duplicate key value")
}

// Close closes the database connection
func (s *PostgresStorage) Close() error {
	return s.db.Close()
}
