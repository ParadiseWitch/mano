package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mano/internal/keys"
)

type helpEntry struct {
	key  string
	desc string
}

var helpSections = []struct {
	title   string
	entries []helpEntry
}{
	{"日志页 | 选择模式", []helpEntry{
		{"j / k", "下移、上移一项（上下方向键同效）"},
		{"gg", "跳到第一项"},
		{"G", "跳到最后一项"},
		{"数字键", "跳到对应序号；300ms 内连按可拼成多位数"},
		{"i / a", "编辑当前项内容（光标落在开头 / 结尾）"},
		{"o", "在下一行插入新项，开始时间填当前时间，并直接进入编辑"},
		{"dd", "删除当前项"},
		{"s", "选中开始时间；缺失时先填入当前时间"},
		{"e", "选中结束时间；缺失时先填入当前时间"},
		{"y", "复制当前项（含内容与起止时间）"},
		{"p", "把复制的项粘贴为最后一项并选中"},
		{"c", "打开日期选择页"},
		{"?", "打开本页"},
		{"q", "退出 mano"},
	}},
	{"日志页 | 时间模式（s 进入开始时间，e 进入结束时间）", []helpEntry{
		{"左右键", "在小时和分钟之间切换"},
		{"上下键", "调整数字：小时每次加 1，分钟每次加 5"},
		{"h / l", "同左右键（仅结束时间模式）"},
		{"k / j", "同上下键（仅结束时间模式）"},
		{"数字键", "直接填入，连按两位组成数值；只按一位则 1 秒后生效"},
		{"Esc", "退出时间模式"},
	}},
	{"日志页 | 编辑模式", []helpEntry{
		{"左右键", "移动光标"},
		{"Backspace", "删除光标前一个字符"},
		{"Enter / Esc", "提交并回到选择模式"},
	}},
	{"日期选择页", []helpEntry{
		{"j / k", "上移、下移一个日期（上下方向键同效）"},
		{"h / l", "上一页、下一页（左右方向键同效）"},
		{"/", "开始搜索，边输入边过滤"},
		{"Enter", "打开选中的日期"},
		{"c / Esc", "返回日志页"},
	}},
	{"搜索与新建日期", []helpEntry{
		{"2026-08", "按日期过滤，紧凑写法 202608 同样有效"},
		{"关键词", "按日志内容过滤，找出写过该词的日子"},
		{"20260801", "输入一个没有记录的完整日期，Enter 直接新建"},
		{"Esc", "清空搜索并取消"},
	}},
	{"通用", []helpEntry{
		{"Ctrl+C", "在任意页面强制退出"},
		{"", "所有修改即时写入 ~/.mano/mano.md，退出无需保存"},
	}},
}

const helpKeyWidth = 12

func (a *App) updateHelp(k tea.KeyMsg) tea.Cmd {
	a.status = ""
	step := 0

	if r, ok := keys.SingleRune(k); ok {
		switch r {
		case 'j':
			step = 1
		case 'k':
			step = -1
		case 'q', '?':
			a.page = pageLog
			return nil
		}
	}

	switch k.Type {
	case tea.KeyDown:
		step = 1
	case tea.KeyUp:
		step = -1
	case tea.KeyPgDown:
		step = a.listHeight()
	case tea.KeyPgUp:
		step = -a.listHeight()
	case tea.KeyHome:
		a.helpOffset = 0
		return nil
	case tea.KeyEnd:
		a.helpOffset = a.maxHelpOffset()
		return nil
	case tea.KeyEsc:
		a.page = pageLog
		return nil
	case tea.KeyCtrlC:
		return tea.Quit
	}

	if step != 0 {
		a.helpOffset = clamp(a.helpOffset+step, 0, a.maxHelpOffset())
	}
	return nil
}

func (a *App) maxHelpOffset() int {
	if n := len(helpLines(a.width)) - a.listHeight(); n > 0 {
		return n
	}
	return 0
}

func (a *App) viewHelp() string {
	lines := helpLines(a.width)
	height := a.listHeight()

	window := make([]string, 0, height)
	for i := a.helpOffset; i < len(lines) && len(window) < height; i++ {
		window = append(window, lines[i])
	}
	for len(window) < height {
		window = append(window, "")
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		spread(a.width, titleStyle.Render("快捷键"), dimStyle.Render("mano")),
		divider(a.width),
		lipgloss.JoinVertical(lipgloss.Left, window...),
		divider(a.width),
		statusStyle.Width(a.width).Render(fit(
			"帮助 | j/k 或 上下键 滚动 | PgUp/PgDn 翻页 | q/Esc/? 返回日志页", a.width)),
	)
}

func helpLines(width int) []string {
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	descWidth := width - rowMargin - helpKeyWidth - 2

	var lines []string
	for _, section := range helpSections {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, strings.Repeat(" ", rowMargin)+sectionStyle.Render(fit(section.title, width-rowMargin)))

		for _, entry := range section.entries {
			key := lipgloss.NewStyle().Width(helpKeyWidth).Foreground(fgText).
				Render(fit(entry.key, helpKeyWidth))
			desc := dimStyle.Render(fit(entry.desc, descWidth))
			lines = append(lines, strings.Repeat(" ", rowMargin)+key+"  "+desc)
		}
	}
	return lines
}
