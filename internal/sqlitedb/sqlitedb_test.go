package sqlitedb

import (
	"context"
	"database/sql"
	"errors"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// childDB names the database the started-again test binary opens; only
// that child runs TestOpenInChild.
const childDB = "MAGPIE_SQLITEDB_CHILD"

// The environment #2327's reporter's magpie was started with: a fragment
// of a DLP tool's dylib path, with no "=", ahead of everything else. The
// first SQLite open reads SQLITE_TMPDIR, and modernc's getenv used to
// print TODOTODO and os.Exit(1) on that entry (on macOS, Windows and the
// BSDs; Linux's musl-based getenv skips it). Now the open works.
func TestSQLiteSurvivesMalformedEnv(t *testing.T) {
	db := filepath.Join(t.TempDir(), "x.db")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestOpenInChild$", "-test.count=1", "-test.v")
	child.Env = append([]string{"hains/MacSec/libesafenet.dylib"}, os.Environ()...)
	child.Env = append(child.Env, childDB+"="+db)
	if runtime.GOOS == "windows" {
		// Windows 在包初始化之前就拒绝非法环境块；只接受这一精确错误。
		out, err := child.CombinedOutput()
		if !errors.Is(err, syscall.Errno(87)) || len(out) != 0 {
			t.Fatalf("非法环境块的启动结果 = %v, %q，要求 ERROR_INVALID_PARAMETER 且未运行子进程", err, out)
		}
		// 再用相同清理器移除坏条目，实际启动子进程并完成 SQLite 查询。
		clean := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestOpenInChild$", "-test.count=1", "-test.v")
		clean.Env = wellFormed(child.Env)
		child = clean
	}
	out, err := child.CombinedOutput()
	if err != nil || strings.Contains(string(out), "TODOTODO") || !strings.Contains(string(out), "opened 1") {
		t.Fatalf("a magpie started with an entry with no \"=\" couldn't open SQLite: %v\n%s", err, out)
	}
}

// TestOpenInChild is the started-again binary's half of
// TestSQLiteSurvivesMalformedEnv: it opens a database and reads from it.
func TestOpenInChild(t *testing.T) {
	path := os.Getenv(childDB)
	if path == "" {
		t.Skip("run by TestSQLiteSurvivesMalformedEnv")
	}
	bad := slices.Contains(os.Environ(), "hains/MacSec/libesafenet.dylib")
	if bad != (runtime.GOOS != "windows") {
		t.Fatalf("子进程的坏条目 = %v，Windows 必须接收清理后的副本，Unix 必须接收原始坏条目", bad)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRow("SELECT 1").Scan(&n); err != nil {
		t.Fatal(err)
	}
	t.Log("opened " + strconv.Itoa(n))
}

func TestWellFormed(t *testing.T) {
	in := []string{"PATH=/bin", "hains/MacSec/libesafenet.dylib", "", "=C:=C:\\x", "EMPTY=", "A=b=c"}
	want := []string{"PATH=/bin", "=C:=C:\\x", "EMPTY=", "A=b=c"}
	if got := wellFormed(in); !slices.Equal(got, want) {
		t.Fatalf("wellFormed(%q) = %q, want %q", in, got, want)
	}
}

// Every SQLite open goes through this package, so libc's environment is
// cleaned before any of them: no other package's own code imports
// modernc's driver or its libc. A test file may, to build its fixtures.
func TestOnlyThisPackageReachesSQLite(t *testing.T) {
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	self := filepath.Join(root, "internal", "sqlitedb")
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && (strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules" || d.Name() == "testdata" || path == self) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, im := range f.Imports {
			p, _ := strconv.Unquote(im.Path.Value)
			if p == "modernc.org/sqlite" || strings.HasPrefix(p, "modernc.org/sqlite/") || p == "modernc.org/libc" {
				rel, _ := filepath.Rel(root, path)
				t.Errorf("%s imports %s; import internal/sqlitedb instead, which cleans libc's environment first (#2327)", rel, p)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
