ALTER TABLE core_monitor_configs
  ADD COLUMN confirmation_check_count INTEGER NOT NULL DEFAULT 0;
