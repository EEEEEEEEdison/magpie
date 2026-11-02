//go:build !goexperiment.jsonv2

package gateway

// The legacy encoder escapes raw line separators only with HTML.
const rawEscapesJS = false

// jsonRuneError 是当前编码器对一个非法 UTF-8 字节的 JSON 写法。
const jsonRuneError = `\ufffd`
