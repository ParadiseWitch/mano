package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// The help page is the only place a key is explained at length, so a line cut at
// the right edge hides the very key a reader is looking for.
func TestEveryHelpLineFitsAnEightyColumnTerminal(t *testing.T) {
	const width = 80

	for _, section := range helpSections {
		if w := lipgloss.Width(section.title) + rowMargin; w > width {
			t.Errorf("标题 %q 占 %d 格，超过 %d", section.title, w, width)
		}
	}

	lines := helpLines(width)
	for i, line := range lines {
		if w := lipgloss.Width(line); w > width {
			t.Errorf("第 %d 行占 %d 格，超过 %d：%s", i+1, w, width, strings.TrimSpace(line))
		}
	}
}

// A description that no row of the rendered page carries in full was cut by fit,
// which is exactly what the line width test cannot see on its own.
func TestNoHelpEntryIsCutShort(t *testing.T) {
	lines := strings.Join(helpLines(80), "\n")

	for _, section := range helpSections {
		for _, entry := range section.entries {
			if entry.desc == "" {
				continue
			}
			if !strings.Contains(lines, entry.desc) {
				t.Errorf("说明被截断了：%q（键 %q）", entry.desc, entry.key)
			}
		}
	}
}

// The keys the redesign retired must not survive on the page a reader trusts.
func TestHelpPageDocumentsNoRetiredKeys(t *testing.T) {
	for _, section := range helpSections {
		if strings.Contains(section.title, "时间模式") {
			t.Errorf("段落 %q 还在说已删除的时间模式", section.title)
		}
		for _, entry := range section.entries {
			key := strings.ReplaceAll(entry.key, " ", "")
			if key == "s" || key == "e" || key == "s/e" || key == "e/s" {
				t.Errorf("键 %q 已被删除，帮助页仍在介绍它", entry.key)
			}
		}
	}
}

// The help page is scrolled by line, so an empty entry list would leave a reader
// staring at a blank page with no way to know why.
func TestHelpPageHasSomethingToShow(t *testing.T) {
	if len(helpSections) == 0 {
		t.Fatal("helpSections 是空的")
	}
	for _, section := range helpSections {
		if len(section.entries) == 0 {
			t.Errorf("段落 %q 没有条目", section.title)
		}
	}
}
