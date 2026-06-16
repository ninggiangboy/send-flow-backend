-- +goose Up
ALTER TABLE audience_export_jobs
    ADD COLUMN zip_output BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN processed_count BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN estimated_total_count BIGINT NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE audience_export_jobs
    DROP COLUMN IF EXISTS estimated_total_count,
    DROP COLUMN IF EXISTS processed_count,
    DROP COLUMN IF EXISTS zip_output;
