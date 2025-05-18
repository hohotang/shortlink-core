-- name: FindShortIDByURL :one
SELECT short_id FROM urls WHERE original_url = $1 LIMIT 1;

-- name: StoreWithID :exec
INSERT INTO urls (short_id, original_url) 
VALUES ($1, $2) 
ON CONFLICT (original_url) DO NOTHING;

-- name: GetURL :one
UPDATE urls 
SET last_accessed = NOW() 
WHERE short_id = $1 
RETURNING original_url; 