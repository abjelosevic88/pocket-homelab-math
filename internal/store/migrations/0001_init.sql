CREATE TABLE users (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  email         TEXT NOT NULL UNIQUE,
  name          TEXT NOT NULL DEFAULT '',
  password_hash TEXT NOT NULL DEFAULT '',
  role          TEXT NOT NULL DEFAULT 'member',
  created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at    TEXT
);

CREATE TABLE sessions (
  id         TEXT PRIMARY KEY,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  expires_at TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE api_tokens (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name         TEXT NOT NULL DEFAULT '',
  token_hash   TEXT NOT NULL UNIQUE,
  prefix       TEXT NOT NULL DEFAULT '',
  last_used_at TEXT,
  created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

-- Box-wide settings (title, setup flag); everything else is personal.
CREATE TABLE settings (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL DEFAULT ''
);

CREATE TABLE user_settings (
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  key     TEXT NOT NULL,
  value   TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (user_id, key)
);

CREATE TABLE cache (
  key        TEXT PRIMARY KEY,
  value      TEXT NOT NULL DEFAULT '',
  fetched_at TEXT NOT NULL
);



-- ---------- Pocket Math ----------
-- Everything belongs to the person who made it (user_id); nobody else on the
-- box sees it, box admins included. A notebook can be opened by others only
-- through its share link (read-only, while switched on). Times are UTC ISO
-- strings.

-- A notebook is a page of cells: calculations, words, graphs, geometry,
-- data and equations.
CREATE TABLE notebooks (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title       TEXT NOT NULL,
  subject     TEXT NOT NULL DEFAULT '',       -- a folder: "Home", "School", "Work"
  about       TEXT NOT NULL DEFAULT '',
  color       TEXT NOT NULL DEFAULT '',
  angle       TEXT NOT NULL DEFAULT 'deg',    -- deg | rad: plain numbers in sin/cos/tan
  units       INTEGER NOT NULL DEFAULT 1,     -- read units and currencies in calculations
  tags        TEXT NOT NULL DEFAULT '[]',
  pinned      INTEGER NOT NULL DEFAULT 0,
  archived    INTEGER NOT NULL DEFAULT 0,
  shared      INTEGER NOT NULL DEFAULT 0,
  share_key   TEXT NOT NULL DEFAULT '',
  revision    INTEGER NOT NULL DEFAULT 1,
  source      TEXT NOT NULL DEFAULT '',       -- the app that made it ('' = here)
  source_ref  TEXT NOT NULL DEFAULT '',
  opened_at   TEXT,
  created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX notebooks_user ON notebooks(user_id, archived, updated_at);
CREATE UNIQUE INDEX notebooks_share ON notebooks(share_key) WHERE share_key <> '';
CREATE UNIQUE INDEX notebooks_ref ON notebooks(user_id, source, source_ref) WHERE source_ref <> '';

CREATE TABLE cells (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  notebook_id INTEGER NOT NULL REFERENCES notebooks(id) ON DELETE CASCADE,
  position    INTEGER NOT NULL DEFAULT 0,
  kind        TEXT NOT NULL DEFAULT 'calc',   -- calc | text | graph | geometry | data | solve
  title       TEXT NOT NULL DEFAULT '',
  body        TEXT NOT NULL DEFAULT '',       -- the lines of a calculation, the words of a text
  data        TEXT NOT NULL DEFAULT '{}',     -- a graph's curves, a drawing, a table, an equation
  results     TEXT NOT NULL DEFAULT '[]',     -- the answers of a calculation, worked out on save
  summary     TEXT NOT NULL DEFAULT '',       -- words for search and the journal
  collapsed   INTEGER NOT NULL DEFAULT 0,
  source      TEXT NOT NULL DEFAULT '',
  created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX cells_user ON cells(user_id, notebook_id, position);

-- A saved calculation: boxes to fill in, the working, the answers. The
-- built-in ones live in the code (library.go); copying one makes a row here.
CREATE TABLE formulas (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name        TEXT NOT NULL,
  about       TEXT NOT NULL DEFAULT '',
  category    TEXT NOT NULL DEFAULT '',
  icon        TEXT NOT NULL DEFAULT '',
  code        TEXT NOT NULL DEFAULT '',
  inputs      TEXT NOT NULL DEFAULT '[]',
  outputs     TEXT NOT NULL DEFAULT '[]',
  from_key    TEXT NOT NULL DEFAULT '',       -- the built-in it was copied from
  pinned      INTEGER NOT NULL DEFAULT 0,
  source      TEXT NOT NULL DEFAULT '',
  source_ref  TEXT NOT NULL DEFAULT '',
  created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX formulas_user ON formulas(user_id, category);
CREATE UNIQUE INDEX formulas_ref ON formulas(user_id, source, source_ref) WHERE source_ref <> '';

-- A kept answer of a saved calculation ("Paint for the bedroom: 6 L").
CREATE TABLE runs (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  formula_id   INTEGER REFERENCES formulas(id) ON DELETE SET NULL,
  formula_key  TEXT NOT NULL DEFAULT '',      -- a built-in
  formula_name TEXT NOT NULL DEFAULT '',
  title        TEXT NOT NULL DEFAULT '',      -- what it was for
  inputs       TEXT NOT NULL DEFAULT '{}',
  outputs      TEXT NOT NULL DEFAULT '[]',
  main         TEXT NOT NULL DEFAULT '',      -- the big answer, as words
  note         TEXT NOT NULL DEFAULT '',
  source       TEXT NOT NULL DEFAULT '',
  created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX runs_user ON runs(user_id, created_at);

-- A round of practice: sums for a child, times tables, fractions…
CREATE TABLE practice (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  learner     TEXT NOT NULL DEFAULT '',       -- who practised ('' = you)
  skill       TEXT NOT NULL,
  level       INTEGER NOT NULL DEFAULT 1,
  questions   INTEGER NOT NULL DEFAULT 0,
  correct     INTEGER NOT NULL DEFAULT 0,
  seconds     INTEGER NOT NULL DEFAULT 0,
  details     TEXT NOT NULL DEFAULT '[]',     -- each question, the answer given, the time
  day         TEXT NOT NULL DEFAULT '',       -- local date
  created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX practice_user ON practice(user_id, day);
