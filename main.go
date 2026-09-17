package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"orgmaid/internal/config"
	"orgmaid/internal/selfupdate"
	"orgmaid/internal/store"
	"orgmaid/internal/ui"
)

// version 由 release.yml 用 -ldflags -X main.version=... 注入。
var version = "devel"

func main() {
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		subcommand()
		return
	}

	file := flag.String("file", "", "日志文件路径（默认 ~/.orgmaid/orgmaid.org）")
	date := flag.String("date", "", "打开指定日期，写作 20260801 或 2026-08-01（默认今天）")
	showVersion := flag.Bool("v", false, "打印版本号并退出")
	flag.BoolVar(showVersion, "version", false, "同 -v")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	path := *file
	if path == "" {
		resolved, err := store.DefaultPath()
		if err != nil {
			fail("找不到用户主目录：%v", err)
		}
		path = resolved
	}

	day := store.Today()
	if *date != "" {
		parsed, ok := store.ParseDate(*date)
		if !ok {
			fail("不是有效日期：%s", *date)
		}
		day = parsed
	}

	journal, err := store.Open(path)
	if err != nil {
		fail("读取 %s 失败：%v", path, err)
	}

	colors, notice := palette()

	program := tea.NewProgram(ui.New(journal, day, colors, notice), tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		fail("%v", err)
	}
}

// palette is the look orgmaid starts with: the config file is written out on the
// first run and read back from then on. Trouble with the file is reported on
// the status line rather than being fatal, because the shipped defaults can
// always carry a session.
func palette() (config.Colors, string) {
	path, err := config.DefaultPath()
	if err != nil {
		return config.Default(), "找不到用户主目录，配色用默认值"
	}
	if err := config.Ensure(path); err != nil {
		return config.Default(), fmt.Sprintf("无法写入 %s：%v", path, err)
	}
	colors, err := config.Load(path)
	if err != nil {
		return colors, fmt.Sprintf("配置 %s：%v", path, err)
	}
	return colors, ""
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "orgmaid: "+format+"\n", args...)
	os.Exit(1)
}

// subcommand 处理 version / update / uninstall，跑完即退出，不进界面。
func subcommand() {
	name, args := os.Args[1], os.Args[2:]
	switch name {
	case "version":
		fmt.Println(version)
	case "update":
		rejectArgs(name, args)
		self, err := selfupdate.SelfPath()
		if err != nil {
			fail("找不到 orgmaid 自己的位置：%v", err)
		}
		if err := selfupdate.Update(version, self, os.Stdout); err != nil {
			fail("%v", err)
		}
	case "uninstall":
		yes := false
		for _, a := range args {
			if a == "-y" || a == "--yes" {
				yes = true
			} else {
				rejectArgs(name, args)
			}
		}
		self, err := selfupdate.SelfPath()
		if err != nil {
			fail("找不到 orgmaid 自己的位置：%v", err)
		}
		if err := selfupdate.Uninstall(self, yes, os.Stdin, os.Stdout); err != nil {
			fail("%v", err)
		}
	default:
		fail("未知子命令 %q，可用：version、update、uninstall", name)
	}
}

func rejectArgs(name string, args []string) {
	if len(args) > 0 {
		fail("orgmaid %s 不接受参数：%v", name, args)
	}
}
