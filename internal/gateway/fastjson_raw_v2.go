//go:build goexperiment.jsonv2

package gateway

// The v2-backed encoder escapes raw line separators independently of HTML.
const rawEscapesJS = true

// jsonRuneError 是当前编码器对一个非法 UTF-8 字节的 JSON 写法。
const jsonRuneError = "�"
