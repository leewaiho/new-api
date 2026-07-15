-- Create tool compatibility event aggregation table.
-- MySQL/MariaDB version. Safe to run repeatedly for a fresh or already migrated database.

CREATE TABLE IF NOT EXISTS `tool_compatibility_events` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `event_key` char(64) NOT NULL,
  `channel_id` bigint NOT NULL,
  `route` varchar(255) NOT NULL,
  `requested_model` varchar(255) NOT NULL DEFAULT '',
  `upstream_model` varchar(255) NOT NULL DEFAULT '',
  `tool_type` varchar(128) NOT NULL DEFAULT '',
  `tool_name` varchar(255) NOT NULL DEFAULT '',
  `event_type` varchar(64) NOT NULL,
  `current_policy` varchar(32) NOT NULL DEFAULT '',
  `suggested_policy` varchar(32) NOT NULL DEFAULT '',
  `error_fingerprint` char(64) NOT NULL,
  `sanitized_error` text NOT NULL,
  `occurrence_count` bigint NOT NULL DEFAULT 1,
  `first_seen_at` bigint NOT NULL,
  `last_seen_at` bigint NOT NULL,
  `resolution_status` varchar(32) NOT NULL DEFAULT 'open',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_tool_compatibility_events_event_key` (`event_key`),
  KEY `idx_tool_compatibility_events_channel_id` (`channel_id`),
  KEY `idx_tool_compatibility_events_requested_model` (`requested_model`),
  KEY `idx_tool_compatibility_events_event_type` (`event_type`),
  KEY `idx_tool_compatibility_events_last_seen_at` (`last_seen_at`),
  KEY `idx_tool_compatibility_events_resolution_status` (`resolution_status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
