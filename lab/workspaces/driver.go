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

var (
	// ErrNotExist is returned by Get for a missing path, on every host.
	ErrNotExist = fs.ErrNotExist
	// ErrAboveRoot is returned for a path that leaves the session root. The
	// session IS the root (filesystem.feature), so there is nothing above "/".
	ErrAboveRoot = errors.New("path above the session root")
)

// norm makes a store path absolute in the session's namespace. A session IS the
// root filesystem, so "data/x" and "/data/x" name one path, and ".." does not
// resolve — isolation is structural, not a permission check.
func norm(p string) (string, error) {
	if p == "" {
		return "/", nil
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	dir := strings.HasSuffix(p, "/")
	var out []string
	for _, seg := range strings.Split(p, "/") {
		switch seg {
		case "", ".":
			continue
		case "..":
			return "", ErrAboveRoot
		default:
			out = append(out, seg)
		}
	}
	r := "/" + strings.Join(out, "/")
	if r != "/" && dir {
		r += "/"
	}
	return r, nil
}

// fileKey normalises p to a file path ("/a/b").
func fileKey(p string) (string, error) {
	n, err := norm(p)
	if err != nil {
		return "", err
	}
	if n = strings.TrimRight(n, "/"); n == "" {
		n = "/"
	}
	return n, nil
}

// dirKey normalises p to a directory path ("/a/b/"), the root staying "/".
func dirKey(p string) (string, error) {
	n, err := fileKey(p)
	if err != nil {
		return "", err
	}
	if n == "/" {
		return "/", nil
	}
	return n + "/", nil
}

// Driver is a host a session's filesystem is materialised on. The whole
// hypothesis (register §0) is that hosts are swappable: the same suite must pass
// *unchanged* on each (filesystem.feature → Rule: the two hosts present the same
// filesystem).
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
	// List returns every path under prefix (a directory, or "/" for the root),
	// absolute in the session's namespace and sorted. Directories keep "/".
	List(ctx context.Context, prefix string) ([]string, error)
	// Name identifies the host ("tmpdir", "sqlite").
	Name() string
	// Close releases the host.
	Close() error
}

// ---------- the fast host: a real directory ----------

// TmpDir is a session materialised on a real directory on a real filesystem.
type TmpDir struct{ root string }

// NewTmpDir opens (creating if needed) a directory-backed session root.
func NewTmpDir(root string) (*TmpDir, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	return &TmpDir{root: root}, nil
}

func (d *TmpDir) Name() string { return "tmpdir" }
func (d *TmpDir) Close() error { return nil }

func (d *TmpDir) abs(p string) (string, error) {
	k, err := fileKey(p)
	if err != nil {
		return "", err
	}
	if k == "/" {
		return d.root, nil
	}
	return filepath.Join(d.root, filepath.FromSlash(k)), nil
}

func (d *TmpDir) Mkdir(_ context.Context, p string) error {
	a, err := d.abs(p)
	if err != nil {
		return err
	}
	return os.MkdirAll(a, 0o755)
}

func (d *TmpDir) Put(_ context.Context, p string, body []byte) error {
	a, err := d.abs(p)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(a), 0o755); err != nil {
		return err
	}
	return os.WriteFile(a, body, 0o644)
}

func (d *TmpDir) Get(_ context.Context, p string) ([]byte, error) {
	a, err := d.abs(p)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(a)
}

func (d *TmpDir) Delete(_ context.Context, p string) error {
	a, err := d.abs(p)
	if err != nil {
		return err
	}
	if err := os.Remove(a); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func (d *TmpDir) Exists(_ context.Context, p string) (bool, error) {
	a, err := d.abs(p)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(a)
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
	base, err := d.abs(prefix)
	if err != nil {
		return nil, err
	}
	var out []string
	walkErr := filepath.WalkDir(base, func(p string, e fs.DirEntry, err error) error {
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
		rel = "/" + filepath.ToSlash(rel)
		if e.IsDir() {
			rel += "/"
		}
		out = append(out, rel)
		return nil
	})
	if errors.Is(walkErr, fs.ErrNotExist) {
		return nil, nil
	}
	if walkErr != nil {
		return nil, walkErr
	}
	sort.Strings(out)
	return out, nil
}

// ---------- the mirror host: sqlite ----------

// SQLite is a session materialised on a single sqlite file (.wsdb). It is
// pure-Go (modernc.org/sqlite), so the mirror needs no cgo and no server.
type SQLite struct {
	db   *sql.DB
	path string
}

// NewSQLite opens (creating if needed) a sqlite-backed session root at path.
func NewSQLite(path string) (*SQLite, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
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

// ancestors returns the directory keys that contain key, outermost first.
func ancestors(key string) []string {
	key = strings.TrimSuffix(key, "/")
	if key == "" || key == "/" {
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
// ancestors implicitly, so recording them keeps the sqlite tree identical to the
// tmpdir tree.
func (d *SQLite) mkdirs(ctx context.Context, p string) error {
	key, err := dirKey(p)
	if err != nil {
		return err
	}
	if key == "/" {
		return nil
	}
	for _, k := range append(ancestors(key), key) {
		if _, err := d.db.ExecContext(ctx,
			`INSERT INTO files(path, dir, body) VALUES(?, 1, NULL)
			 ON CONFLICT(path) DO UPDATE SET dir=1`, k); err != nil {
			return err
		}
	}
	return nil
}

func (d *SQLite) Mkdir(ctx context.Context, p string) error { return d.mkdirs(ctx, p) }

func (d *SQLite) Put(ctx context.Context, p string, body []byte) error {
	k, err := fileKey(p)
	if err != nil {
		return err
	}
	if err := d.mkdirs(ctx, path.Dir(k)); err != nil {
		return err
	}
	_, err = d.db.ExecContext(ctx,
		`INSERT INTO files(path, dir, body) VALUES(?, 0, ?)
		 ON CONFLICT(path) DO UPDATE SET dir=0, body=excluded.body`, k, body)
	return err
}

func (d *SQLite) Get(ctx context.Context, p string) ([]byte, error) {
	k, err := fileKey(p)
	if err != nil {
		return nil, err
	}
	var body []byte
	err = d.db.QueryRowContext(ctx, `SELECT body FROM files WHERE path=? AND dir=0`, k).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotExist
	}
	return body, err
}

func (d *SQLite) Delete(ctx context.Context, p string) error {
	k, err := fileKey(p)
	if err != nil {
		return err
	}
	dk, _ := dirKey(p)
	_, err = d.db.ExecContext(ctx, `DELETE FROM files WHERE path=? OR path=?`, k, dk)
	return err
}

func (d *SQLite) Exists(ctx context.Context, p string) (bool, error) {
	k, err := fileKey(p)
	if err != nil {
		return false, err
	}
	dk, _ := dirKey(p)
	var n int
	err = d.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM files WHERE path=? OR path=?`, k, dk).Scan(&n)
	return n > 0, err
}

func (d *SQLite) List(ctx context.Context, prefix string) ([]string, error) {
	n, err := norm(prefix)
	if err != nil {
		return nil, err
	}
	k, _ := fileKey(prefix)
	dk, _ := dirKey(prefix)
	rows, err := d.db.QueryContext(ctx,
		`SELECT path FROM files
		  WHERE substr(path,1,?)=? AND path<>? AND path<>?
		  ORDER BY path`, len(n), n, k, dk)
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
