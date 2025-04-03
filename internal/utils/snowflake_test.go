package utils

import (
	"testing"
	"time"
)

func TestNewSnowflakeGenerator(t *testing.T) {
	tests := []struct {
		name      string
		machineID int64
		expectErr bool
	}{
		{
			name:      "Valid Machine ID",
			machineID: 5,
			expectErr: false,
		},
		{
			name:      "Zero Machine ID",
			machineID: 0,
			expectErr: false,
		},
		{
			name:      "Negative Machine ID",
			machineID: -5,
			expectErr: false, // Should be set to default 1
		},
		{
			name:      "Too large Machine ID",
			machineID: 1024,  // Out of range (0-1023)
			expectErr: false, // Should be set to default 1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator, err := NewSnowflakeGenerator(tt.machineID)

			if tt.expectErr {
				if err == nil {
					t.Errorf("NewSnowflakeGenerator(%d) expected error but got nil", tt.machineID)
				}
			} else {
				if err != nil {
					t.Errorf("NewSnowflakeGenerator(%d) returned unexpected error: %v", tt.machineID, err)
				}

				if generator == nil {
					t.Errorf("NewSnowflakeGenerator(%d) returned nil generator", tt.machineID)
					return
				}

				// Test that the generator works
				_, err := generator.NextID()
				if err != nil {
					t.Errorf("generator.NextID() returned unexpected error: %v", err)
				}
			}
		})
	}
}

func TestSnowflakeNextID(t *testing.T) {
	// Set up a generator
	generator, err := NewSnowflakeGenerator(5)
	if err != nil {
		t.Fatalf("Failed to create generator: %v", err)
	}

	// Generate an ID
	id1, err := generator.NextID()
	if err != nil {
		t.Fatalf("NextID() returned unexpected error: %v", err)
	}

	// Generate a second ID and verify it's different (should be larger)
	id2, err := generator.NextID()
	if err != nil {
		t.Fatalf("Second NextID() returned unexpected error: %v", err)
	}

	if id2 <= id1 {
		t.Errorf("Expected second ID %d to be larger than first ID %d", id2, id1)
	}
}

func TestSnowflakeEncode(t *testing.T) {
	generator, _ := NewSnowflakeGenerator(1)

	tests := []struct {
		id       int64
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{10, "a"},
		{61, "Z"},
		{62, "10"},
	}

	for _, tt := range tests {
		encoded := generator.Encode(tt.id)
		if encoded != tt.expected {
			t.Errorf("Encode(%d) = %s, expected %s", tt.id, encoded, tt.expected)
		}
	}

	// For larger numbers, just ensure we get a non-empty string
	encoded := generator.Encode(123456789)
	if encoded == "" {
		t.Errorf("Encode(123456789) returned empty string")
	}
}

func TestSequentialIDs(t *testing.T) {
	// This test verifies that sequential IDs are generated in order
	generator, _ := NewSnowflakeGenerator(1)

	var lastID int64
	idCount := 1000

	for i := 0; i < idCount; i++ {
		id, err := generator.NextID()
		if err != nil {
			t.Fatalf("NextID() returned unexpected error: %v", err)
		}

		if id <= lastID && lastID != 0 {
			t.Fatalf("ID %d is not greater than the previous ID %d", id, lastID)
		}

		lastID = id
	}
}

func TestShortLinkID(t *testing.T) {
	generator, _ := NewSnowflakeGenerator(1)
	linkCount := 100
	ids := make(map[string]bool)
	for i := 0; i < linkCount; i++ {
		id := GenerateShortID(generator)
		if ids[id] {
			t.Errorf("Duplicate short ID generated: %s", id)
		}
		ids[id] = true
	}
}

// Note: Clock backwards test is not needed for bwmarrin/snowflake
// as it handles this internally, but we will test our generation
// works independently of time changes
func TestTimeIndependence(t *testing.T) {
	// Create a generator
	generator, _ := NewSnowflakeGenerator(1)

	// Save the original time function
	originalTimeFunc := timeNow
	defer func() { timeNow = originalTimeFunc }()

	// Set a fixed time
	fixedTime := time.Date(2021, 5, 1, 0, 0, 0, 0, time.UTC)
	timeNow = func() time.Time {
		return fixedTime
	}

	// Generate an ID (should work regardless of fixed time)
	id, err := generator.NextID()
	if err != nil {
		t.Fatalf("NextID() returned unexpected error with fixed time: %v", err)
	}

	// Should be able to encode the ID
	encoded := generator.Encode(id)
	if encoded == "" {
		t.Errorf("Encode() returned empty string")
	}

	// GenerateShortID should work
	shortID := GenerateShortID(generator)
	if shortID == "" {
		t.Errorf("GenerateShortID() returned empty string")
	}
}
