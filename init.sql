-- Create URLs table
CREATE TABLE IF NOT EXISTS urls (
    short_id VARCHAR(255) PRIMARY KEY,
    original_url TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_accessed TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Add unique index to original_url for reverse lookup
CREATE UNIQUE INDEX IF NOT EXISTS idx_urls_original_url ON urls (original_url);

-- Add index on created_at for date-based queries
CREATE INDEX IF NOT EXISTS idx_urls_created_at ON urls (created_at);

-- Add index on last_accessed for cleanup/analytics
CREATE INDEX IF NOT EXISTS idx_urls_last_accessed ON urls (last_accessed);

-- Grant permissions (adjust as needed)
GRANT ALL PRIVILEGES ON TABLE urls TO postgres; 