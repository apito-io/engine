package sqlite

const filesTableDDL = `
CREATE TABLE IF NOT EXISTS files(
	id VARCHAR(36) NOT NULL PRIMARY KEY,
	project_id VARCHAR(36),
	file_type VARCHAR(32) NOT NULL,
	file_name TEXT NOT NULL,
	file_extension VARCHAR(65),
	content_type VARCHAR(128),
	size BIGINT NOT NULL,
	storage_key TEXT NOT NULL,
	url TEXT,
	created_by VARCHAR(36),
	created_at VARCHAR(128),
	updated_at VARCHAR(128)
);`

const filesTableDDLPostgresPublic = `
CREATE TABLE IF NOT EXISTS public.files(
	id VARCHAR(36) NOT NULL PRIMARY KEY,
	project_id VARCHAR(36),
	file_type VARCHAR(32) NOT NULL,
	file_name TEXT NOT NULL,
	file_extension VARCHAR(65),
	content_type VARCHAR(128),
	size BIGINT NOT NULL,
	storage_key TEXT NOT NULL,
	url TEXT,
	created_by VARCHAR(36),
	created_at VARCHAR(128),
	updated_at VARCHAR(128)
);`
