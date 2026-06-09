-- +goose Up
-- Backfill workspace_id for dead_letter_records that were created before the
-- column was added (migration 000023).  The old DeadLetterRepository.Save used
-- json.Marshal(record.Payload) where record.Payload was the raw Kafka message
-- bytes ([]byte).  Go's json.Marshal base64-encodes []byte, so those payloads
-- are stored as JSON strings rather than JSON objects.
--
-- Step 1: decode base64-encoded (string-typed) payloads back to JSON objects
-- and extract workspace_id at the same time.
DO $$
DECLARE
    rec RECORD;
    decoded_text TEXT;
    decoded_json JSONB;
BEGIN
    FOR rec IN
        SELECT id, payload FROM dead_letter_records
        WHERE jsonb_typeof(payload) = 'string'
        FOR UPDATE
    LOOP
        BEGIN
            decoded_text := convert_from(decode(rec.payload #>> '{}', 'base64'), 'UTF8');
            decoded_json := decoded_text::jsonb;
            UPDATE dead_letter_records
            SET payload = decoded_json,
                workspace_id = decoded_json->>'workspace_id'
            WHERE id = rec.id;
        EXCEPTION WHEN others THEN
            -- Payload isn't valid base64 or the decoded bytes are not valid
            -- JSON; leave the row untouched, workspace_id stays NULL.
            NULL;
        END;
    END LOOP;
END;
$$;

-- Step 2: extract workspace_id from any remaining object-typed payloads that
-- were stored correctly (e.g. by tests or future code paths).
UPDATE dead_letter_records
SET workspace_id = payload->>'workspace_id'
WHERE workspace_id IS NULL
  AND jsonb_typeof(payload) = 'object'
  AND payload->>'workspace_id' IS NOT NULL;

-- +goose Down
-- No-op: backfill is a one-way data correction.
