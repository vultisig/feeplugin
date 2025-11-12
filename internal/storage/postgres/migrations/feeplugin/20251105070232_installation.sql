-- +goose Up
-- +goose StatementBegin
BEGIN;

CREATE TABLE plugin_keys (
                             id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                             public_key VARCHAR(255) NOT NULL,
                             created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE plugin_keys ADD CONSTRAINT unique_public_key UNIQUE (public_key);
CREATE INDEX idx_plugin_keys_created_at ON plugin_keys(created_at);

END;
-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS plugin_keys;