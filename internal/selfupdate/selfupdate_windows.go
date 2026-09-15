//go:build windows

package selfupdate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// Windows 不允许删除或覆盖正在运行的 exe，但允许改名。
// 于是把旧文件改名为 .old，让新文件顶上原位；.old 留给下次启动时清理。
func replace(self, new string) error {
	cleanOld(self)
	old := fmt.Sprintf("%s.old-%d", self, time.Now().UnixNano())
	if err := os.Rename(self, old); err != nil {
		return fmt.Errorf("无法改名正在运行的 %s：%w", self, err)
	}
	if err := os.Rename(new, self); err != nil {
		os.Rename(old, self) // 尽力回滚
		return fmt.Errorf("替换失败，已恢复原版本：%w", err)
	}
	os.Remove(old) // 此刻通常仍被占用，删不掉就留着
	return nil
}

func removeSelf(self string) error {
	cleanOld(self)
	old := fmt.Sprintf("%s.old-%d", self, time.Now().UnixNano())
	if err := os.Rename(self, old); err != nil {
		return fmt.Errorf("无法改名正在运行的 %s：%w", self, err)
	}
	// 运行中的镜像删不掉，交给一个两秒后收尾的后台进程。
	ps := "Start-Sleep 2; Remove-Item -LiteralPath '" +
		strings.ReplaceAll(old, "'", "''") + "' -Force"
	cmd := exec.Command("powershell", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", ps)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "mano: 后台清理进程启动失败，请手动删除 %s\n", old)
	}
	return nil
}

// cleanOld 删掉历次更新残留的 .old 文件。
func cleanOld(self string) {
	matches, _ := filepath.Glob(self + ".old-*")
	for _, m := range matches {
		os.Remove(m)
	}
}
