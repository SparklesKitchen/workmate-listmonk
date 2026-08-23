-- media
-- name: insert-media
INSERT INTO media (uuid, filename, thumb, content_type, provider, meta, list_id, created_at) VALUES($1, $2, $3, $4, $5, $6, $7, NOW()) RETURNING id;

-- name: query-media
SELECT COUNT(*) OVER () AS total, * FROM media
    WHERE ($1 = '' OR filename ILIKE $1) AND provider=$2 AND ($3 = 0 OR list_id=$3) ORDER BY created_at DESC OFFSET $4 LIMIT $5;

-- name: get-media
SELECT * FROM media WHERE
    CASE
        WHEN $1 > 0 THEN id = $1
        WHEN $2 != '' THEN uuid = $2::UUID
        WHEN $3 != '' THEN filename = $3    
        ELSE false
    END AND ($4 = 0 OR list_id=$4);

-- name: delete-media
DELETE FROM media WHERE id=$1 AND ($2 = 0 OR list_id=$2) RETURNING filename;
