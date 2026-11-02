package testenv

import (
	"os"
	"os/exec"
	"runtime"
	"testing"
)

// 在每例测试清空 PATH 之前定位系统命令。
var linkCommand, _ = exec.LookPath("cmd.exe")

// FolderLink 创建真实目录链接；Windows 使用不需要提权的 junction。
func FolderLink(t testing.TB, target, path string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		if out, err := exec.Command(linkCommand, "/d", "/c", "mklink", "/J", path, target).CombinedOutput(); err != nil {
			t.Fatalf("mklink /J: %v %s", err, out)
		}
		return
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
}
