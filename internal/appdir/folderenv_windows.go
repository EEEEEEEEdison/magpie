package appdir

import (
	"slices"
	"sync"
	"syscall"
)

var folderVars struct {
	sync.Mutex
	by map[string]*folderValue
}

type folderValue struct {
	key   *uint16
	buf   []uint16
	seen  []uint16
	value string
}

// folderEnv 每次读当前进程环境，只在值改变时重新解码；Windows 的
// os.Getenv 每次都分配键、UTF-16 缓冲和返回字符串。
func folderEnv(name string) string {
	folderVars.Lock()
	defer folderVars.Unlock()
	f := folderVars.by[name]
	if f == nil {
		key, err := syscall.UTF16PtrFromString(name)
		if err != nil {
			return ""
		}
		f = &folderValue{key: key, buf: make([]uint16, 256)}
		if folderVars.by == nil {
			folderVars.by = map[string]*folderValue{}
		}
		folderVars.by[name] = f
	}
	for {
		n, err := syscall.GetEnvironmentVariable(f.key, &f.buf[0], uint32(len(f.buf)))
		if err != nil || n == 0 {
			f.seen, f.value = f.seen[:0], ""
			return ""
		}
		if n >= uint32(len(f.buf)) {
			f.buf = make([]uint16, n+1)
			continue
		}
		if !slices.Equal(f.seen, f.buf[:n]) {
			f.value = syscall.UTF16ToString(f.buf[:n])
			f.seen = append(f.seen[:0], f.buf[:n]...)
		}
		return f.value
	}
}
