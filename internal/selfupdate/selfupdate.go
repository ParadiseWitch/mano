// Package selfupdate 实现 `mano update` 与 `mano uninstall`：
// 从 GitHub Release 检查、下载、校验并就地替换 mano 自己的二进制，
// 或删除自己。日志与配置永远保留。
package selfupdate

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"mano/internal/config"
	"mano/internal/store"
)

// releaseBase 在测试里指向 httptest 服务器。
var releaseBase = "https://github.com/ParadiseWitch/mano/releases"

var httpc = &http.Client{Timeout: 2 * time.Minute}

// assetName 是本平台在 Release 里的资产名，与 release.yml 的产物一致。
func assetName() string {
	name := "mano-" + runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

// SelfPath 返回当前可执行文件的绝对路径。
func SelfPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

// LatestTag 跟随 releases/latest 的重定向，取出最新 release 的 tag。
func LatestTag() (string, error) {
	resp, err := httpc.Get(releaseBase + "/latest")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("查询最新版本失败：HTTP %d", resp.StatusCode)
	}
	const marker = "/releases/tag/"
	i := strings.LastIndex(resp.Request.URL.Path, marker)
	if i < 0 {
		return "", fmt.Errorf("无法从 %s 解析版本号", resp.Request.URL)
	}
	return resp.Request.URL.Path[i+len(marker):], nil
}

// parseChecksums 从 sha256sum 输出里取 asset 的哈希。
func parseChecksums(data, asset string) (string, error) {
	for _, line := range strings.Split(data, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == asset {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("release 中没有 %s 这个平台的产物", asset)
}

func verifySha256(path, want string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("sha256 校验失败：期望 %s，实际 %s", want, got)
	}
	return nil
}

// download 把 url 写入 dst。
func download(url string, dst io.Writer) error {
	resp, err := httpc.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载 %s 失败：HTTP %d", url, resp.StatusCode)
	}
	_, err = io.Copy(dst, resp.Body)
	return err
}

// Update 检查最新版本，若有更新则下载、校验并替换 self。
func Update(version, self string, out io.Writer) error {
	tag, err := LatestTag()
	if err != nil {
		return err
	}
	if tag == version {
		fmt.Fprintf(out, "mano %s 已是最新版本\n", tag)
		return nil
	}
	fmt.Fprintf(out, "当前 %s，最新 %s，开始更新\n", version, tag)

	want, err := func() (string, error) {
		var sums strings.Builder
		if err := download(releaseBase+"/download/"+tag+"/checksums.txt", &sums); err != nil {
			return "", err
		}
		return parseChecksums(sums.String(), assetName())
	}()
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(self), ".mano-update-*")
	if err != nil {
		return fmt.Errorf("无法在 %s 写入临时文件（权限不足？可 sudo mano update）：%w", filepath.Dir(self), err)
	}
	defer os.Remove(tmp.Name())
	url := fmt.Sprintf("%s/download/%s/%s", releaseBase, tag, assetName())
	if err := download(url, tmp); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := verifySha256(tmp.Name(), want); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return err
	}
	if err := replace(self, tmp.Name()); err != nil {
		return err
	}
	fmt.Fprintf(out, "已更新到 %s\n", tag)
	return nil
}

// Uninstall 删除自身。日志与配置只提示位置，绝不代删。
func Uninstall(self string, yes bool, in io.Reader, out io.Writer) error {
	if !yes {
		fmt.Fprintf(out, "将删除 %s，日志与配置保留。输入 y 确认：", self)
		answer, err := bufio.NewReader(in).ReadString('\n')
		if err != nil && answer == "" {
			return err
		}
		if s := strings.ToLower(strings.TrimSpace(answer)); s != "y" && s != "yes" {
			fmt.Fprintln(out, "已取消")
			return nil
		}
	}
	if err := removeSelf(self); err != nil {
		return err
	}
	fmt.Fprintf(out, "已删除 %s\n", self)
	if p, err := store.DefaultPath(); err == nil {
		fmt.Fprintf(out, "日志保留在 %s\n", p)
	}
	if p, err := config.DefaultPath(); err == nil {
		fmt.Fprintf(out, "配置保留在 %s\n", p)
	}
	fmt.Fprintln(out, "不再需要时手动删除上述文件即可")
	return nil
}
