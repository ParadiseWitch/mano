package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

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

	program := tea.NewProgram(ui.New(journal, day), tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		fail("%v", err)
	}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "mano: "+format+"\n", args...)
	os.Exit(1)
}
