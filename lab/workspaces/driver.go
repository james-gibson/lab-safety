package workspaces

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

// ErrNotExist is returned by Get for a missing path, on every host.
var ErrNotExist = fs.ErrNotExist

// Driver is a host a workspace store is materialised on. The whole hypothesis
// (register §0) is that hosts are swappable: the same conventions must hold on
// each, and the same suite must pass *unchanged* on both (register §5).
//
// Store paths are slash-separated and relative to the store root. Directories
// carry a trailing "/", so a host without real directories (sqlite) can still
// report an empty tree.
type Driver interface {
	// Mkdir records a directory (and its parents).
	Mkdir(ctx context.Context, p string) error
	// Put writes body at p, creating parents as needed.
	Put(ctx context.Context, p string, body []byte) error
	// Get reads p. A missing path returns ErrNotExist.
	Get(ctx context.Context, p string) ([]byte, error)
	// Delete removes p. A missing path is not an error.
	Delete(ctx context.Context, p string) error
	// Exists reports whether p exists (as a file or a directory).
	Exists(ctx context.Context, p string) (bool, error)
	// List returns every path under prefix (a directory, or "" for the root),
	// store-relative and sorted. Directories carry a trailing "/".
	List(ctx context.Context, prefix string) ([]string, error)
	// Name identifies the host ("tmpdir", "sqlite").
	Name() string
	// Close releases the host.
	Close() error
}

// ---------- the fast host: a real directory ----------

// TmpDir is a store backed by a real directory on a real filesystem — the
// experiment, not staging (register §0).
type TmpDir struct{ root string }

// NewTmpDir opens (creating if needed) a directory-backed store.
func NewTmpDir(root string) (*TmpDir, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	return &TmpDir{root: root}, nil
}

func (d *TmpDir) Name() string { return "tmpdir" }
func (d *TmpDir) Close() error { return nil }

func (d *TmpDir) abs(p string) string {
	return filepath.Join(d.root, filepath.FromSlash(strings.TrimSuffix(p, "/")))
}

func (d *TmpDir) Mkdir(_ context.Context, p string) error { return os.MkdirAll(d.abs(p), 0o755) }

func (d *TmpDir) Put(_ context.Context, p string, body []byte) error {
	a := d.abs(p)
	if err := os.MkdirAll(filepath.Dir(a), 0o755); err != nil {
		return err
	}
	return os.WriteFile(a, body, 0o644)
}

func (d *TmpDir) Get(_ context.Context, p string) ([]byte, error) { return os.ReadFile(d.abs(p)) }

func (d *TmpDir) Delete(_ context.Context, p string) error {
	err := os.Remove(d.abs(p))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

func (d *TmpDir) Exists(_ context.Context, p string) (bool, error) {
	_, err := os.Stat(d.abs(p))
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, fs.ErrNotExist):
		return false, nil
	default:
		return false, err
	}
}

func (d *TmpDir) List(_ context.Context, prefix string) ([]string, error) {
	base := d.abs(prefix)
	var out []string
	err := filepath.WalkDir(base, func(p string, e fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if p == base {
			return nil
		}
		rel, relErr := filepath.Rel(d.root, p)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if e.IsDir() {
			rel += "/"
		}
		out = append(out, rel)
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

// ---------- the mirror host: sqlite ----------

// SQLite is a store backed by a single sqlite file (.wsdb, one per session).
// It is pure-Go (modernc.org/sqlite), so the mirror needs no cgo and no server.
type SQLite struct {
	db   *sql.DB
	path string
}

// NewSQLite opens (creating if needed) a sqlite-backed store at path.
func NewSQLite(path string) (*SQLite, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// One writer at a time: the spike's whole point is a single writer.
	db.SetMaxOpenConns(1)
	const schema = `CREATE TABLE IF NOT EXISTS files (
		path TEXT PRIMARY KEY,
		dir  INTEGER NOT NULL DEFAULT 0,
		body BLOB
	)`
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &SQLite{db: db, path: path}, nil
}

// Path returns the backing .wsdb file path.
func (d *SQLite) Path() string { return d.path }
func (d *SQLite) Name() string { return "sqlite" }
func (d *SQLite) Close() error { return d.db.Close() }

func dirKey(p string) string { return strings.TrimSuffix(p, "/") + "/" }

// ancestors returns the directory keys that contain key, outermost first.
func ancestors(key string) []string {
	key = strings.TrimSuffix(key, "/")
	if key == "" {
		return nil
	}
	parts := strings.Split(key, "/")
	var out []string
	acc := ""
	for _, part := range parts[:len(parts)-1] {
		acc += part + "/"
		out = append(out, acc)
	}
	return out
}

// mkdirs records a directory *and its ancestors*: a real filesystem has the
// ancestors implicitly, so recording them keeps the sqlite tree identical to
// the tmpdir tree.
func (d *SQLite) mkdirs(ctx context.Context, key string) error {
	key = dirKey(key)
	if key == "/" || key == "./" {
		return nil
	}
	for _, p := range append(ancestors(key), key) {
		if _, err := d.db.ExecContext(ctx,
			`INSERT INTO files(path, dir, body) VALUES(?, 1, NULL)
			 ON CONFLICT(path) DO UPDATE SET dir=1`, p); err != nil {
			return err
		}
	}
	return nil
}

func (d *SQLite) Mkdir(ctx context.Context, p string) error { return d.mkdirs(ctx, p) }

func (d *SQLite) Put(ctx context.Context, p string, body []byte) error {
	key := strings.TrimSuffix(p, "/")
	if err := d.mkdirs(ctx, path.Dir(key)); err != nil {
		return err
	}
	_, err := d.db.ExecContext(ctx,
		`INSERT INTO files(path, dir, body) VALUES(?, 0, ?)
		 ON CONFLICT(path) DO UPDATE SET dir=0, body=excluded.body`, key, body)
	return err
}

func (d *SQLite) Get(ctx context.Context, p string) ([]byte, error) {
	var body []byte
	err := d.db.QueryRowContext(ctx,
		`SELECT body FROM files WHERE path=? AND dir=0`, strings.TrimSuffix(p, "/")).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotExist
	}
	return body, err
}

func (d *SQLite) Delete(ctx context.Context, p string) error {
	key := strings.TrimSuffix(p, "/")
	_, err := d.db.ExecContext(ctx,
		`DELETE FROM files WHERE path=? OR path=?`, key, key+"/")
	return err
}

func (d *SQLite) Exists(ctx context.Context, p string) (bool, error) {
	key := strings.TrimSuffix(p, "/")
	var n int
	err := d.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM files WHERE path=? OR path=?`, key, key+"/").Scan(&n)
	return n > 0, err
}

func (d *SQLite) List(ctx context.Context, prefix string) ([]string, error) {
	// Everything strictly under prefix — excluding the prefix directory itself,
	// so the result matches a directory walk on a real filesystem.
	rows, err := d.db.QueryContext(ctx,
		`SELECT path FROM files WHERE substr(path,1,?)=? AND path<>? AND path<>? ORDER BY path`,
		len(prefix), prefix, prefix, dirKey(prefix))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
