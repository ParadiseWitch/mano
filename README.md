# orgmaid

[![CI](https://github.com/ParadiseWitch/orgmaid/actions/workflows/ci.yml/badge.svg)](https://github.com/ParadiseWitch/orgmaid/actions/workflows/ci.yml)

一个个人日志 TUI 程序。

像程序日志一样记录自己的各项事务和耗时。数据存为 org-mode 格式，纯文本，随时可以用 Emacs 或其他编辑器打开。

**名字来源**：org-mode + maid，一个帮你打理 org-mode 日志的小助手。

<p align="center">
  <img src="docs/screenshot.png" alt="orgmaid 运行截图" width="720">
</p>

在 herdr 中使用：

<p align="center">
  <img src="docs/use_in_herdr.png" alt="orgmaid 在 herdr 中使用" width="720">
</p>

## 安装

### 脚本安装

macOS / Linux：

```sh
curl -fsSL https://raw.githubusercontent.com/ParadiseWitch/orgmaid/main/install.sh | bash
```

Windows PowerShell：

```powershell
irm https://raw.githubusercontent.com/ParadiseWitch/orgmaid/main/install.ps1 | iex
```

### 下载二进制

从 [Releases](https://github.com/ParadiseWitch/orgmaid/releases) 下载对应平台的文件：

| 平台                  | 文件                        |
| --------------------- | --------------------------- |
| macOS (Apple Silicon) | `orgmaid-darwin-arm64`      |
| macOS (Intel)         | `orgmaid-darwin-amd64`      |
| Linux x64             | `orgmaid-linux-amd64`       |
| Linux arm64           | `orgmaid-linux-arm64`       |
| Windows x64           | `orgmaid-windows-amd64.exe` |

macOS：

```sh
chmod +x orgmaid-darwin-arm64
sudo mv orgmaid-darwin-arm64 /usr/local/bin/orgmaid
```

Windows PowerShell：

```powershell
Move-Item orgmaid-windows-amd64.exe $HOME\bin\orgmaid.exe
```

### 从源码构建

需要 Go 1.24 或更新版本。

```sh
git clone https://github.com/ParadiseWitch/orgmaid.git && cd orgmaid
make build                          # 产物在 dist/orgmaid
cp dist/orgmaid /usr/local/bin/     # 或任何在 PATH 里的目录
```

交叉编译 Windows 版本（在 macOS/Linux 上也能构建）：

```sh
make build-windows   # 产物在 dist/orgmaid-windows-amd64.exe
```

### 更新与卸载

装好之后：

```sh
orgmaid version      # 打印版本号
orgmaid update       # 检查最新 Release，下载校验后替换自身
orgmaid uninstall    # 删除 orgmaid 本体（会先确认；日志与配置保留）
```

## 运行

```sh
orgmaid                        # 打开今天的日志
orgmaid -v                     # 打印版本号（-version 同效，子命令 version 亦可）
orgmaid -date 2026-08-01       # 打开指定日期，也可写 20260801
orgmaid -file ~/notes/time.org # 换一个数据文件
```

默认数据文件是 `~/.orgmaid/orgmaid.org`，目录和文件都会在第一次保存时自动创建。

## 数据格式

数据文件使用 org-mode 格式：

```org
* 2026-08-01
** 日志事项1内容
   - START: 09:00
   - END: 10:21
** TODO 写周报                                                      :work:
   - START: 10:30
   - END: 11:21
** DONE 买菜                                                        :life:

* 2026-08-02
** 只写了内容，没有时间
```

- `- START:` 和 `- END:` 都是可选的，缺哪个就不写哪一行
- `TODO` / `DONE` 是可选的状态关键字
- `:tag1:tag2:` 是可选的标签，写在行末

## 开发

```sh
make test    # go test ./...
make vet     # go vet ./...
make fmt     # gofmt -w .
make dist    # 同时构建本机版和 Windows 版
make clean   # 删掉 dist/
```
