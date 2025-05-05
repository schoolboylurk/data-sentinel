-- RBAC groups for Permit.io “group” sync
CREATE TABLE IF NOT EXISTS groups (
  id   INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT    UNIQUE NOT NULL
);

-- Who the kids are and how old they are
CREATE TABLE IF NOT EXISTS kids (
  username   TEXT PRIMARY KEY,
  age        INTEGER NOT NULL
);

-- What each kid is allowed or explicitly restricted from asking
CREATE TABLE IF NOT EXISTS content_policies (
  kid_username TEXT PRIMARY KEY,
  allowed      TEXT,  -- comma-separated list of allowed topics
  restricted   TEXT   -- comma-separated list of disallowed topics
);

-- Track each prompt request and whether a parent approved it
CREATE TABLE IF NOT EXISTS prompt_requests (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  kid_username  TEXT    NOT NULL,
  prompt        TEXT    NOT NULL,
  approved      BOOLEAN NOT NULL DEFAULT FALSE,
  created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Chat sessions (one per child)
CREATE TABLE IF NOT EXISTS chat_sessions (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  kid_username  TEXT    NOT NULL,
  created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Chat messages (all user/AI exchanges)
CREATE TABLE IF NOT EXISTS chat_messages (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  session_id    INTEGER NOT NULL,
  sender        TEXT    NOT NULL,      -- "kid" or "ai"
  content       TEXT    NOT NULL,
  timestamp     DATETIME DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY(session_id) REFERENCES chat_sessions(id)
);

-- Violation attempts log (when child prompt violates policy)
CREATE TABLE IF NOT EXISTS violation_attempts (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  kid_username  TEXT    NOT NULL,
  prompt        TEXT    NOT NULL,
  violation     TEXT    NOT NULL,      -- e.g. "restricted topic"
  timestamp     DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS audit_events (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  event_type TEXT    NOT NULL,
  username   TEXT    NOT NULL,
  timestamp  DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Group membership (many-to-many between groups and users)
CREATE TABLE IF NOT EXISTS group_members (
  group_id INTEGER NOT NULL,
  username TEXT    NOT NULL,
  PRIMARY KEY (group_id, username),
  FOREIGN KEY (group_id) REFERENCES groups(id)
);

-- (Optional) speed up the admin “requests” listing
CREATE INDEX IF NOT EXISTS idx_prompt_requests_created_at
  ON prompt_requests(created_at);

-- (Optional) speed up violation lookups
CREATE INDEX IF NOT EXISTS idx_violation_attempts_timestamp
  ON violation_attempts(timestamp);