# Backup

Everything lives in `DATA_DIR`: the SQLite file `pocket-math.db` (WAL mode; every picture is inside it). Take a consistent copy with `pocket-math backup` (writes `DATA_DIR/backups/pocket-math-<time>.db` with `VACUUM INTO`) or Settings → Backup & data → Download database (admin). Back up that copy, not the live file. `DATA_DIR/core.token*` is the Pocket Core registration.

Restore: stop the app, replace `pocket-math.db` (remove the `-wal`/`-shm` files), start.

Every person can also export everything they made — notebooks with their cells, their own calculations, kept answers and practice rounds — as one JSON file (Settings → Backup & data), and bring it back on any box; importing the same file twice adds nothing twice.
