package plugin

import (
	"net"
	"strings"
	"testing"
	"time"
)

// 清理 listener 也须关闭已接入、还未完成握手的连接，等待服务 goroutine。
func TestSOCKSFixtureClosesAcceptedConnections(t *testing.T) {
	var conn net.Conn
	t.Cleanup(func() {
		if conn != nil {
			conn.Close()
		}
	})
	t.Run("server", func(t *testing.T) {
		proxy, _ := socksTo(t, "")
		var err error
		conn, err = net.Dial("tcp", strings.TrimPrefix(proxy, "socks5://"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Write([]byte{5, 1, 0}); err != nil {
			t.Fatal(err)
		}
		var accepted [2]byte
		if _, err := conn.Read(accepted[:]); err != nil {
			t.Fatal(err)
		}
	})
	if conn == nil {
		t.Fatal("SOCKS 夹具未接入连接")
	}
	conn.SetReadDeadline(time.Now().Add(time.Second))
	var b [1]byte
	_, err := conn.Read(b[:])
	if err == nil {
		t.Fatal("清理后连接仍能读取数据")
	}
	if e, ok := err.(net.Error); ok && e.Timeout() {
		t.Fatal("清理 listener 后，已接入的 SOCKS 连接仍未关闭")
	}
}
