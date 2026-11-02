package plugin

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// npm 的 bun.cmd 留着真正的 Bun 继承管道；只杀 cmd.exe 不会给 read EOF。
func TestStopWaitsForTheShimAndItsHost(t *testing.T) {
	sandbox(t)
	dir := t.TempDir()
	shim := filepath.Join(dir, "bun.cmd")
	if err := os.WriteFile(shim, []byte("@call \""+os.Getenv("MAGPIE_BUN")+"\" \"%~dp0host.js\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "host.js"), []byte(`setInterval(() => {}, 1000); require("node:fs").writeFileSync("ready", String(process.pid))`), 0o600); err != nil {
		t.Fatal(err)
	}
	was := stopWait
	stopWait = 20 * time.Millisecond
	t.Cleanup(func() { stopWait = was })
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	cmd := bunCommand(ctx, shim, dir)
	in, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatal(err)
	}
	hctx, hcancel := context.WithCancel(context.Background())
	h := &host{
		cmd: cmd, in: in, calls: map[int64]*call{}, dead: make(chan struct{}),
		out: make(chan writeReq, callLimit()), ctrl: map[int64]*ctrl{},
		ctx: hctx, cancel: hcancel,
	}
	readDone, stderrDone := make(chan struct{}), make(chan struct{})
	go func() { h.read(out); close(readDone) }()
	go func() { drainStderr(stderr); close(stderrDone) }()
	var child *os.Process
	var stopped chan struct{}
	t.Cleanup(func() {
		cancel()
		if child != nil {
			_ = child.Kill()
			_ = child.Release()
		}
		<-readDone
		<-stderrDone
		if stopped != nil {
			<-stopped
		}
	})
	for child == nil {
		if b, err := os.ReadFile(filepath.Join(dir, "ready")); err == nil {
			pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
			if err != nil {
				t.Fatal(err)
			}
			child, err = os.FindProcess(pid)
			if err != nil {
				t.Fatal(err)
			}
			break
		}
		select {
		case <-h.dead:
			t.Fatal("the host quit before it was ready")
		case <-ctx.Done():
			t.Fatal("the host never became ready")
		case <-time.After(10 * time.Millisecond):
		}
	}
	t.Logf("shim pid %d, host pid %d", cmd.Process.Pid, child.Pid)
	stopped = make(chan struct{})
	go func() { h.stop(); close(stopped) }()
	select {
	case <-stopped:
		if h.alive() || cmd.ProcessState == nil {
			t.Fatal("stop returned before the host's reader and Wait finished")
		}
	case <-time.After(10 * time.Second):
		t.Error("stop left the shim's host holding its output pipes")
	}
}
