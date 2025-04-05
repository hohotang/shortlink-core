package storage

import (
	"database/sql"
	"fmt"

	"github.com/hohotang/shortlink-core/internal/config"
	"github.com/hohotang/shortlink-core/internal/logger"
	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

// PostgresStorage implements URLStorage with PostgreSQL
type PostgresStorage struct {
	db *sql.DB
}

// NewPostgresStorage creates a new PostgresStorage instance
func NewPostgresStorage(cfg *config.Config) (*PostgresStorage, error) {
	var connStr string
	pgConfig := cfg.Storage.Postgres
	log := logger.L()

	// Generate connection string from individual parameters
	if pgConfig.Host != "" {
		// Use the new detailed config if available
		connStr = fmt.Sprintf(
			"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			pgConfig.Host, pgConfig.Port, pgConfig.User, pgConfig.Password, pgConfig.DBName, pgConfig.SSLMode,
		)
	} else {
		// Fall back to the legacy postgres_url if detailed config is not set
		connStr = cfg.Storage.PostgresURL
		log.Info("Using legacy postgres_url config. Consider updating to the new postgres configuration format.")
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	// Configure connection pool
	if pgConfig.MaxOpenConns > 0 {
		db.SetMaxOpenConns(pgConfig.MaxOpenConns)
		log.Info("PostgreSQL connection pool: max open connections set",
			zap.Int("maxOpenConns", pgConfig.MaxOpenConns))
	}

	if pgConfig.MaxIdleConns > 0 {
		db.SetMaxIdleConns(pgConfig.MaxIdleConns)
		log.Info("PostgreSQL connection pool: max idle connections set",
			zap.Int("maxIdleConns", pgConfig.MaxIdleConns))
	}

	if pgConfig.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(pgConfig.ConnMaxLifetime)
		log.Info("PostgreSQL connection pool: connection max lifetime set",
			zap.Duration("maxLifetime", pgConfig.ConnMaxLifetime))
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	log.Info("PostgreSQL connection established")

	return &PostgresStorage{db: db}, nil
}

// FindShortIDByURL checks if a URL already has a short ID
func (s *PostgresStorage) FindShortIDByURL(originalURL string) (string, error) {
	log := logger.L()

	if originalURL == "" {
		return "", ErrInvalidURL
	}

	var shortID string
	err := s.db.QueryRow(
		"SELECT short_id FROM urls WHERE original_url = $1 LIMIT 1",
		originalURL,
	).Scan(&shortID)

	if err != nil {
		if err == sql.ErrNoRows {
			log.Debug("No existing short ID found for URL", zap.String("url", originalURL))
			return "", ErrNotFound
		}
		log.Error("Failed to query for existing URL", zap.Error(err))
		return "", fmt.Errorf("failed to query for existing URL: %w", err)
	}

	log.Debug("Found existing short ID for URL",
		zap.String("shortID", shortID),
		zap.String("url", originalURL))
	return shortID, nil
}

// Store implements URLStorage.Store
func (s *PostgresStorage) Store(originalURL string) (string, error) {
	// This method simply calls FindShortIDByURL to check if the URL already exists
	return s.FindShortIDByURL(originalURL)
}

// StoreWithID implements URLStorage.StoreWithID
func (s *PostgresStorage) StoreWithID(shortID string, originalURL string) error {
	log := logger.L()

	if originalURL == "" {
		return ErrInvalidURL
	}

	// First, check if this URL already exists
	existingShortID, err := s.FindShortIDByURL(originalURL)
	if err == nil {
		// URL already exists with a different shortID
		if existingShortID != shortID {
			log.Info("URL already exists with different short ID, replacing",
				zap.String("existingShortID", existingShortID),
				zap.String("newShortID", shortID),
				zap.String("url", originalURL))

			// Replace the existing mapping
			return s.replaceURLMapping(existingShortID, shortID, originalURL)
		}
		// URL already exists with the same shortID, no-op
		log.Debug("URL already exists with same short ID, no changes needed",
			zap.String("shortID", shortID),
			zap.String("url", originalURL))
		return nil
	} else if err != ErrNotFound {
		// An actual error occurred (not just "not found")
		log.Error("Error checking for existing URL", zap.Error(err))
		return err
	}

	// URL doesn't exist, insert it
	return s.withTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(
			"INSERT INTO urls (short_id, original_url) VALUES ($1, $2)",
			shortID, originalURL,
		)
		if err != nil {
			log.Error("Failed to insert URL",
				zap.Error(err),
				zap.String("shortID", shortID),
				zap.String("url", originalURL))
			return fmt.Errorf("failed to insert URL: %w", err)
		}

		log.Debug("Inserted new URL",
			zap.String("shortID", shortID),
			zap.String("url", originalURL))
		return nil
	})
}

// withTransaction handles the boilerplate of transactions
func (s *PostgresStorage) withTransaction(fn func(*sql.Tx) error) error {
	log := logger.L()

	// Start a transaction
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Execute the function
	err = fn(tx)

	// If there was an error, rollback
	if err != nil {
		// Attempt to rollback, but don't override the original error
		if rbErr := tx.Rollback(); rbErr != nil {
			// Log rollback error but return the original error
			log.Error("Error rolling back transaction", zap.Error(rbErr))
		}
		return err
	}

	// Otherwise commit
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// replaceURLMapping handles the transaction to replace an existing URL mapping with a new one
func (s *PostgresStorage) replaceURLMapping(existingShortID, newShortID, originalURL string) error {
	log := logger.L()

	return s.withTransaction(func(tx *sql.Tx) error {
		// Delete the existing record for this original_url
		_, err := tx.Exec("DELETE FROM urls WHERE short_id = $1", existingShortID)
		if err != nil {
			log.Error("Failed to delete existing record",
				zap.Error(err),
				zap.String("shortID", existingShortID))
			return fmt.Errorf("failed to delete existing record: %w", err)
		}

		// Now insert the new record
		_, err = tx.Exec(
			"INSERT INTO urls (short_id, original_url) VALUES ($1, $2)",
			newShortID, originalURL,
		)
		if err != nil {
			log.Error("Failed to insert new record",
				zap.Error(err),
				zap.String("shortID", newShortID))
			return fmt.Errorf("failed to insert new record: %w", err)
		}

		log.Info("Replaced URL mapping",
			zap.String("oldShortID", existingShortID),
			zap.String("newShortID", newShortID),
			zap.String("url", originalURL))

		return nil
	})
}

// Get implements URLStorage.Get
func (s *PostgresStorage) Get(shortID string) (string, error) {
	log := logger.L()
	var originalURL string

	// Query the URL and update last_accessed
	err := s.db.QueryRow(
		"UPDATE urls SET last_accessed = NOW() WHERE short_id = $1 RETURNING original_url",
		shortID,
	).Scan(&originalURL)

	if err != nil {
		if err == sql.ErrNoRows {
			log.Debug("Short ID not found", zap.String("shortID", shortID))
			return "", ErrNotFound
		}
		log.Error("Failed to get URL", zap.Error(err), zap.String("shortID", shortID))
		return "", fmt.Errorf("failed to get URL: %w", err)
	}

	log.Debug("Retrieved URL for short ID",
		zap.String("shortID", shortID),
		zap.String("url", originalURL))

	return originalURL, nil
}

// Close closes the database connection
func (s *PostgresStorage) Close() error {
	log := logger.L()
	log.Info("Closing PostgreSQL connection")
	return s.db.Close()
}
