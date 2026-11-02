package catalog

import (
	"path/filepath"
	"testing"
)

// A provider's list file is named once, not on each read of the list: a
// look at the providers reads it for every model.
func TestLivePathNamedOnce(t *testing.T) {
	if got, want := LivePath("openai"), filepath.Join(filepath.Dir(CachePath()), "models", "openai.json"); got != want {
		t.Fatalf("LivePath = %q, want %q", got, want)
	}
	if n := testing.AllocsPerRun(20, func() { LivePath("openai") }); n > 0 {
		t.Errorf("LivePath with the folders unchanged: %.0f allocations", n)
	}
}

// 同一供应商的目录跟随当前缓存根目录，已返回的文件名保持不变。
func TestLivePathFollowsCacheFolder(t *testing.T) {
	for _, dir := range []string{t.TempDir(), t.TempDir()} {
		t.Setenv("XDG_CACHE_HOME", dir)
		for _, id := range []string{"openai", "deepseek"} {
			got, want := LivePath(id), filepath.Join(dir, "magpie", "models", id+".json")
			if got != want {
				t.Fatalf("LivePath(%q) = %q, want %q", id, got, want)
			}
			if n := testing.AllocsPerRun(20, func() { LivePath(id) }); n > 0 {
				t.Errorf("LivePath(%q) after moving the cache: %.0f allocations", id, n)
			}
		}
	}
}
