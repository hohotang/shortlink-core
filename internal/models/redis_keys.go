package models

// Redis key constants for URL shortener
const (
	// ReverseURLsKey is the hash that maps original URLs to short IDs
	ReverseURLsKey = "reverse_urls"

	// ShortIDKeyPrefix is the prefix for keys that store short ID data
	ShortIDKeyPrefix = "url:"
)
