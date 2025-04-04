package models

// StorageType represents the type of storage backend to use
type StorageType string

const (
	// Memory storage type uses in-memory map for storage
	Memory StorageType = "memory"

	// Redis storage type uses Redis for storage
	Redis StorageType = "redis"

	// Postgres storage type uses PostgreSQL for storage
	Postgres StorageType = "postgres"

	// Combined storage type uses both Redis (for caching) and PostgreSQL (for persistence)
	Combined StorageType = "both"
)

// String returns the string representation of the storage type
func (s StorageType) String() string {
	return string(s)
}
