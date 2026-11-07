-- How many distinct metric series an app may report over OTLP (0 = the server default of 100).
ALTER TABLE apps ADD COLUMN metric_series_cap INTEGER NOT NULL DEFAULT 0;
