package appdir

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// magpie's folders and the files named from them are joined once, not on
// every read of a file, and a folder changed since is seen at once.
func TestFoldersJoinedOnce(t *testing.T) {
	t.Setenv("APPIMAGE", "")
	r := root(t)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(r, "cfg"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(r, "cache"))
	t.Cleanup(func() { UseExecutable("") })
	UseExecutable("")

	for name, f := range map[string]func(){
		"Config":   func() { Config() },
		"Cache":    func() { Cache() },
		"JoinFile": func() { JoinFile(Cache(), "openai", ".json") },
	} {
		f()
		if n := testing.AllocsPerRun(20, f); n > 0 {
			t.Errorf("%s with the folders unchanged: %.0f allocations", name, n)
		}
	}
	if got, want := JoinFile(Join(Cache(), "models"), "openai", ".json"), filepath.Join(r, "cache", "magpie", "models", "openai.json"); got != want {
		t.Errorf("JoinFile = %q, want %q", got, want)
	}

	t.Setenv("XDG_CACHE_HOME", filepath.Join(r, "other"))
	if got, want := Cache(), filepath.Join(r, "other", "magpie"); got != want {
		t.Errorf("a cache folder changed since: %q, want %q", got, want)
	}
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv(homeVar(), filepath.Join(r, "home"))
	if got, want := Config(), filepath.Join(r, "home", ".config", "magpie"); got != want {
		t.Errorf("under the home folder: %q, want %q", got, want)
	}
}

// 没有 XDG 时仍按当前 HOME 组合路径，不缓存上一次的目录。
func TestFoldersWithoutXDGJoinedOnce(t *testing.T) {
	t.Setenv("APPIMAGE", "")
	r := root(t)
	t.Setenv(homeVar(), r)
	t.Cleanup(func() { UseExecutable("") })
	UseExecutable("")
	for _, unset := range []bool{false, true} {
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("XDG_CACHE_HOME", "")
		if unset {
			for _, name := range []string{"XDG_CONFIG_HOME", "XDG_CACHE_HOME"} {
				if err := os.Unsetenv(name); err != nil {
					t.Fatal(err)
				}
			}
		}
		for name, f := range map[string]func() string{"Config": Config, "Cache": Cache, "Home": mustHome} {
			f()
			if n := testing.AllocsPerRun(20, func() { f() }); n > 0 {
				t.Errorf("%s without XDG (unset=%t): %.0f allocations", name, unset, n)
			}
		}
	}
}

// 缓冲增长、Unicode、空值和删除均读当前值，已经返回的路径不随缓冲改写。
func TestFolderReadsKeepTheCurrentEnvironment(t *testing.T) {
	t.Setenv("APPIMAGE", "")
	r := root(t)
	t.Setenv(homeVar(), r)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")
	t.Cleanup(func() { UseExecutable("") })
	UseExecutable("")
	long := filepath.Join(r, strings.Repeat("long-folder/", 40), "目录😀")
	for _, tc := range []struct {
		name, fallback string
		read           func() string
	}{
		{"XDG_CONFIG_HOME", ".config", Config},
		{"XDG_CACHE_HOME", ".cache", Cache},
	} {
		for _, value := range []string{long, filepath.Join(r, "短😀"), ""} {
			t.Setenv(tc.name, value)
			want := filepath.Join(value, "magpie")
			if value == "" {
				want = filepath.Join(r, tc.fallback, "magpie")
			}
			got := tc.read()
			if got != want {
				t.Fatalf("%s=%q: got %q, want %q", tc.name, value, got, want)
			}
			if n := testing.AllocsPerRun(20, func() { tc.read() }); n > 0 {
				t.Errorf("%s (%d bytes): %.0f allocations", tc.name, len(value), n)
			}
			t.Setenv(tc.name, filepath.Join(r, "changed"))
			tc.read()
			if got != want {
				t.Fatalf("%s: previously returned path changed: %q, want %q", tc.name, got, want)
			}
		}
		if err := os.Unsetenv(tc.name); err != nil {
			t.Fatal(err)
		}
		if got, want := tc.read(), filepath.Join(r, tc.fallback, "magpie"); got != want {
			t.Fatalf("unset %s: got %q, want %q", tc.name, got, want)
		}
	}
	for _, home := range []string{long, filepath.Join(r, "other-home")} {
		t.Setenv(homeVar(), home)
		if got, err := Home(); err != nil || got != home {
			t.Fatalf("Home() = %q, %v, want %q", got, err, home)
		}
	}
}
