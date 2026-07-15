-- Create tool compatibility event aggregation table.
-- PostgreSQL version. Safe to run repeatedly.

CREATE TABLE IF NOT EXISTS tool_compatibility_events (
  id BIGSERIAL PRIMARY KEY,
  event_key char(64) NOT NULL,
  channel_id bigint NOT NULL,
  route varchar(255) NOT NULL,
  requested_model varchar(255) NOT NULL DEFAULT '',
  upstream_model varchar(255) NOT NULL DEFAULT '',
  tool_type varchar(128) NOT NULL DEFAULT '',
  tool_name varchar(255) NOT NULL DEFAULT '',
  event_type varchar(64) NOT NULL,
  current_policy varchar(32) NOT NULL DEFAULT '',
  suggested_policy varchar(32) NOT NULL DEFAULT '',
  error_fingerprint char(64) NOT NULL,
  sanitized_error text NOT NULL DEFAULT '',
  occurrence_count bigint NOT NULL DEFAULT 1,
  first_seen_at bigint NOT NULL,
  last_seen_at bigint NOT NULL,
  resolution_status varchar(32) NOT NULL DEFAULT 'open'
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_tool_compatibility_events_event_key
  ON tool_compatibility_events (event_key);
CREATE INDEX IF NOT EXISTS idx_tool_compatibility_events_channel_id
  ON tool_compatibility_events (channel_id);
CREATE INDEX IF NOT EXISTS idx_tool_compatibility_events_requested_model
  ON tool_compatibility_events (requested_model);
CREATE INDEX IF NOT EXISTS idx_tool_compatibility_events_event_type
  ON tool_compatibility_events (event_type);
CREATE INDEX IF NOT EXISTS idx_tool_compatibility_events_last_seen_at
  ON tool_compatibility_events (last_seen_at);
CREATE INDEX IF NOT EXISTS idx_tool_compatibility_events_resolution_status
  ON tool_compatibility_events (resolution_status);
