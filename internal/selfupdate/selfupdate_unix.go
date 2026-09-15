//go:build !windows

package selfupdate

import "os"

// replace 直接用新文件盖掉旧路径；Unix 下运行中的可执行文件可以被 rename 覆盖。
func replace(self, new string) error {
	return os.Rename(new, self)
}

// removeSelf 删掉自己；Unix 允许 unlink 正在运行的二进制。
func removeSelf(self string) error {
	return os.Remove(self)
}
