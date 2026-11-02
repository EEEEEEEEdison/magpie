//go:build !windows

package appdir

import "os"

func folderEnv(name string) string { return os.Getenv(name) }
