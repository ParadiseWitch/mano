package selfupdate

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParseChecksums(t *testing.T) {
	data := "aabb  mano-linux-amd64\nccdd  mano-windows-amd64.exe\n"
	got, err := parseChecksums(data, "mano-windows-amd64.exe")
	if err != nil || got != "ccdd" {
		t.Fatalf("得到 %q %v，期望 ccdd", got, err)
	}
	if _, err := parseChecksums(data, "mano-darwin-arm64"); err == nil {
		t.Fatal("缺资产时应报错")
	}
}

func TestVerifySha256(t *testing.T) {
	f := filepath.Join(t.TempDir(), "bin")
	content := []byte("hello mano")
	if err := os.WriteFile(f, content, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	if err := verifySha256(f, hex.EncodeToString(sum[:])); err != nil {
		t.Fatalf("正确哈希应通过：%v", err)
	}
	if err := verifySha256(f, strings.Repeat("00", 32)); err == nil {
		t.Fatal("错误哈希应失败")
	}
}

// fakeRelease 起一个模仿 GitHub Release 下载路径的服务器。
func fakeRelease(t *testing.T, tag, payload string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/releases/tag/"+tag, http.StatusFound)
	})
	// GitHub 上 tag 页本身是 200，跟随重定向后要拿到 200 才算成功。
	mux.HandleFunc("/releases/tag/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	asset := assetName()
	sum := sha256.Sum256([]byte(payload))
	mux.HandleFunc("/releases/download/"+tag+"/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(sum[:]), asset)
	})
	mux.HandleFunc("/releases/download/"+tag+"/"+asset, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(payload))
	})
	return httptest.NewServer(mux)
}

func withReleaseBase(t *testing.T, base string) {
	t.Helper()
	old := releaseBase
	releaseBase = base
	t.Cleanup(func() { releaseBase = old })
}

func TestLatestTag(t *testing.T) {
	srv := fakeRelease(t, "v3.2.1", "x")
	defer srv.Close()
	withReleaseBase(t, srv.URL+"/releases")
	tag, err := LatestTag()
	if err != nil || tag != "v3.2.1" {
		t.Fatalf("得到 %q %v，期望 v3.2.1", tag, err)
	}
}

func TestUpdateReplacesSelf(t *testing.T) {
	srv := fakeRelease(t, "v2.0.0", "new-binary")
	defer srv.Close()
	withReleaseBase(t, srv.URL+"/releases")

	self := filepath.Join(t.TempDir(), "mano")
	if err := os.WriteFile(self, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Update("v1.0.0", self, &out); err != nil {
		t.Fatalf("更新失败：%v", err)
	}
	got, err := os.ReadFile(self)
	if err != nil || string(got) != "new-binary" {
		t.Fatalf("替换后内容 %q %v", got, err)
	}
	if !strings.Contains(out.String(), "v2.0.0") {
		t.Fatalf("输出应提到新版本：%s", out.String())
	}
	// Unix 临时文件用 rename 顶掉，不该留下残渣；Windows 的 .old 留给下次清理。
	if runtime.GOOS != "windows" {
		matches, _ := filepath.Glob(self + "*")
		if len(matches) != 1 {
			t.Fatalf("留下多余文件：%v", matches)
		}
	}
}

func TestUpdateAlreadyLatest(t *testing.T) {
	srv := fakeRelease(t, "v2.0.0", "x")
	defer srv.Close()
	withReleaseBase(t, srv.URL+"/releases")

	var out bytes.Buffer
	if err := Update("v2.0.0", filepath.Join(t.TempDir(), "mano"), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "已是最新") {
		t.Fatalf("输出：%s", out.String())
	}
}

func TestUpdateRejectsBadChecksum(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/latest"):
			http.Redirect(w, r, "/releases/tag/v9.0.0", http.StatusFound)
		case strings.HasSuffix(r.URL.Path, "checksums.txt"):
			fmt.Fprintf(w, "%s  %s\n", strings.Repeat("11", 32), assetName())
		default:
			w.Write([]byte("tampered"))
		}
	}))
	defer srv.Close()
	withReleaseBase(t, srv.URL+"/releases")

	self := filepath.Join(t.TempDir(), "mano")
	if err := os.WriteFile(self, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Update("v1.0.0", self, &bytes.Buffer{}); err == nil {
		t.Fatal("校验失败时 Update 应报错")
	}
	got, _ := os.ReadFile(self)
	if string(got) != "old" {
		t.Fatal("校验失败时不应动原文件")
	}
}

func TestUninstall(t *testing.T) {
	self := filepath.Join(t.TempDir(), "mano")
	if err := os.WriteFile(self, []byte("bin"), 0o755); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := Uninstall(self, false, strings.NewReader("n\n"), &out); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(self); err != nil {
		t.Fatal("回答 n 时不应删除")
	}

	out.Reset()
	if err := Uninstall(self, true, strings.NewReader(""), &out); err != nil {
		t.Fatalf("卸载失败：%v", err)
	}
	if _, err := os.Stat(self); !os.IsNotExist(err) {
		t.Fatalf("原路径应消失：%v", err)
	}
	if !strings.Contains(out.String(), "日志保留") {
		t.Fatalf("应提示日志位置：%s", out.String())
	}
}
