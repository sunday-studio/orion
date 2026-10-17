CREATE TABLE IF NOT EXISTS alert_channel_recipients (
  id TEXT PRIMARY KEY,
  channel_id TEXT NOT NULL,
  email TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',
  confirmed_at DATETIME,
  confirmation_sent_at DATETIME,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_alert_channel_recipients_unique ON alert_channel_recipients(channel_id, email);
CREATE TABLE IF NOT EXISTS alert_delivery_recipients (
  delivery_id TEXT NOT NULL,
  recipient_id TEXT NOT NULL,
  sent_at DATETIME NOT NULL,
  PRIMARY KEY (delivery_id, recipient_id)
);
