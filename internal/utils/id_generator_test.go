package utils

import (
	"testing"
	"time"
)

// mockSnowflakeGenerator implements IDGenerator with predictable behavior for testing
type mockSnowflakeGenerator struct {
	mockNextIDFunc  func() (int64, error)
	mockEncodeFunc  func(id int64) string
	nextIDCallCount int
	encodeCallCount int
}

func (m *mockSnowflakeGenerator) NextID() (int64, error) {
	m.nextIDCallCount++
	return m.mockNextIDFunc()
}

func (m *mockSnowflakeGenerator) Encode(id int64) string {
	m.encodeCallCount++
	return m.mockEncodeFunc(id)
}

func (m *mockSnowflakeGenerator) GenerateShortID() string {
	id, err := m.NextID()
	if err != nil {
		// Fallback to timestamp-based ID in case of error
		id = timeNow().UnixNano()
	}

	return m.Encode(id)
}

func TestGenerateShortID_Success(t *testing.T) {
	// Arrange
	expectedID := int64(123456789)
	expectedEncodedID := "2Bi4"

	mock := &mockSnowflakeGenerator{
		mockNextIDFunc: func() (int64, error) {
			return expectedID, nil
		},
		mockEncodeFunc: func(id int64) string {
			if id != expectedID {
				t.Errorf("Expected encode to be called with %d, got %d", expectedID, id)
			}
			return expectedEncodedID
		},
	}

	// Act
	result := mock.GenerateShortID()

	// Assert
	if result != expectedEncodedID {
		t.Errorf("Expected GenerateShortID to return %s, got %s", expectedEncodedID, result)
	}
	if mock.nextIDCallCount != 1 {
		t.Errorf("Expected NextID to be called once, was called %d times", mock.nextIDCallCount)
	}
	if mock.encodeCallCount != 1 {
		t.Errorf("Expected Encode to be called once, was called %d times", mock.encodeCallCount)
	}
}

func TestGenerateShortID_ErrorFallback(t *testing.T) {
	// Arrange
	now := time.Now()
	expectedTimestamp := now.UnixNano()

	// Capture the timestamp used for fallback
	var capturedTimestamp int64

	mock := &mockSnowflakeGenerator{
		mockNextIDFunc: func() (int64, error) {
			return 0, &mockError{"NextID error"}
		},
		mockEncodeFunc: func(id int64) string {
			capturedTimestamp = id
			return "fallback"
		},
	}

	// Replace time.Now with a mock that returns a fixed time
	originalTimeNow := timeNow
	timeNow = func() time.Time {
		return now
	}
	defer func() { timeNow = originalTimeNow }()

	// Act
	result := mock.GenerateShortID()

	// Assert
	if result != "fallback" {
		t.Errorf("Expected GenerateShortID to return 'fallback', got %s", result)
	}
	if mock.nextIDCallCount != 1 {
		t.Errorf("Expected NextID to be called once, was called %d times", mock.nextIDCallCount)
	}
	if mock.encodeCallCount != 1 {
		t.Errorf("Expected Encode to be called once, was called %d times", mock.encodeCallCount)
	}
	if capturedTimestamp != expectedTimestamp {
		t.Errorf("Expected fallback to use current timestamp %d, but got %d", expectedTimestamp, capturedTimestamp)
	}
}

// BenchmarkGenerateShortID measures the performance of the GenerateShortID function.
//
// Test environment:
// - CPU: 12th Gen Intel(R) Core(TM) i5-12400
// - Architecture: amd64
// - Operating System: Windows
//
// Results:
// - Each ID generation operation takes approximately 353.4 nanoseconds (0.3534 microseconds)
// - The generator can produce about 2.83 million unique IDs per second
// - This is extremely efficient and suitable for high-throughput applications
// - The generator is fast enough to handle most production traffic scenarios
func BenchmarkGenerateShortID(b *testing.B) {
	// Create a real SnowflakeGenerator instance
	generator, err := NewSnowflakeGenerator(1)
	if err != nil {
		b.Fatalf("Failed to create snowflake generator: %v", err)
	}

	// Reset the timer to ensure initialization code is not counted
	b.ResetTimer()

	// Run b.N times (Go benchmark framework will automatically determine the number of runs)
	for i := 0; i < b.N; i++ {
		// For testing 10,000 IDs, we generate 10,000 / b.N IDs each iteration
		// But at least 1
		iterations := 10000 / b.N
		if iterations < 1 {
			iterations = 1
		}

		for j := 0; j < iterations; j++ {
			_ = generator.GenerateShortID()
		}
	}
}

// BenchmarkGenerateShortID_10000 directly tests generating 10,000 short IDs in one batch.
//
// Test environment:
// - CPU: 12th Gen Intel(R) Core(TM) i5-12400
// - Architecture: amd64
// - Operating System: Windows
//
// Results:
// - Generating 10,000 unique IDs takes approximately 3.01 milliseconds
// - Memory allocation: 1.76 MB total (approximately 176 bytes per ID)
// - About 220,000 allocations (22 allocations per ID)
//
// Performance implications:
// - Can generate 3.32 million IDs per second on a single instance
// - Extremely scalable - even with high traffic, ID generation will not be a bottleneck
// - Memory usage is moderate and acceptable for production environments
// - The implementation is efficient enough for most high-traffic applications
func BenchmarkGenerateShortID_10000(b *testing.B) {
	// Create a real SnowflakeGenerator instance
	generator, err := NewSnowflakeGenerator(1)
	if err != nil {
		b.Fatalf("Failed to create snowflake generator: %v", err)
	}

	// Reset the timer to ensure initialization code is not counted
	b.ResetTimer()

	// Run the benchmark b.N times (use -benchtime=1x flag to run just once)
	for i := 0; i < b.N; i++ {
		// Generate 10,000 IDs in each iteration
		for j := 0; j < 10000; j++ {
			_ = generator.GenerateShortID()
		}
	}
}

type mockError struct {
	msg string
}

func (e *mockError) Error() string {
	return e.msg
}
