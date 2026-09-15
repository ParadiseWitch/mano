package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"mano/internal/config"
	"mano/internal/store"
	"mano/internal/ui"
)

func main() {
	file := flag.String("file", "", "日志文件路径（默认 ~/.mano/mano.md）")
	date := flag.String("date", "", "打开指定日期，写作 20260801 或 2026-08-01（默认今天）")
	flag.Parse()

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

// palette is the look mano starts with: the config file is written out on the
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
	fmt.Fprintf(os.Stderr, "mano: "+format+"\n", args...)
	os.Exit(1)
}
