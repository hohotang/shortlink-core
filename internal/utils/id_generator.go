package utils

import (
	"time"

	"github.com/bwmarrin/snowflake"
)

// For easier testing
var timeNow = time.Now

const (
	// Base62Charset is the character set for base62 encoding (0-9, a-z, A-Z)
	Base62Charset = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

// IDGenerator is the interface for generating unique IDs
type IDGenerator interface {
	NextID() (int64, error)
	Encode(id int64) string
}

// SnowflakeGenerator wraps bwmarrin/snowflake Node for ID generation
type SnowflakeGenerator struct {
	node *snowflake.Node
}

// NewSnowflakeGenerator creates a new SnowflakeGenerator
func NewSnowflakeGenerator(machineID int64) (*SnowflakeGenerator, error) {
	// Ensure machineID is in valid range (0-1023)
	if machineID < 0 || machineID > 1023 {
		machineID = 1 // Default to 1 if out of range
	}

	// Create a new snowflake node
	node, err := snowflake.NewNode(machineID)
	if err != nil {
		return nil, err
	}

	return &SnowflakeGenerator{
		node: node,
	}, nil
}

// NextID generates the next unique ID
func (s *SnowflakeGenerator) NextID() (int64, error) {
	// Generate a new snowflake ID
	id := s.node.Generate()

	// Return the int64 representation
	return id.Int64(), nil
}

// Encode converts a numeric ID to a base62 string
func (s *SnowflakeGenerator) Encode(id int64) string {
	// Handle 0 case
	if id == 0 {
		return string(Base62Charset[0])
	}

	// Convert to base62
	var result []byte
	base := int64(len(Base62Charset))

	for id > 0 {
		remainder := id % base
		id = id / base
		result = append([]byte{Base62Charset[remainder]}, result...)
	}

	return string(result)
}

// GenerateShortID generates a short ID using snowflake and base62
func GenerateShortID(generator IDGenerator) string {
	id, err := generator.NextID()
	if err != nil {
		// Fallback to timestamp-based ID in case of error
		id = timeNow().UnixNano()
	}

	return generator.Encode(id)
}
