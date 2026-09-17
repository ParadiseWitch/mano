// Package config holds orgmaid's appearance settings: the themes it ships with,
// and the TOML file that picks one of them and overrides whichever colours the
// user wants different, so the UI never hard-codes a colour of its own.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Colors is every colour the UI paints with, as #rrggbb. The field names say
// which region of the screen each one belongs to. A list row carries no bands:
// its columns are told apart by these accents as text on the program ground, and
// the only two backgrounds on a row are the selected row's own and the block the
// cursor stands on.
type Colors struct {
	Canvas   string // unused since the background went transparent; kept so old configs still load
	Row      string // the selected row's ground
	Index    string // the item number
	Start    string // start time
	End      string // end time
	Duration string // elapsed span
	Crossed  string // an elapsed span that runs past midnight
	Field    string // the block under the cursor: half a reading, or the number
	Selected string // content of the selected row
	Editing  string // content while it is being edited
	Text     string // logged content
	Ink      string // the dark text sitting on the Field block
	Dim      string // secondary text, and a reading that is not there yet
	Title    string // page title
	Warn     string // the row marker, the help page's headings
	Status   string // the key-hint bar
	Divider  string // the rule between page regions
}

// entry is one line of the file: the key as it is written, the comment the
// template carries, and the field it lands in. Read and write share this table,
// so a colour cannot be added to one half and forgotten in the other.
type entry struct {
	key  string
	note string
	get  func(Colors) string
	set  func(*Colors, string)
}

var entries = []entry{
	{"background", "已不再生效：背景改为透明，跟随终端配色；保留此键只为兼容旧配置", func(c Colors) string { return c.Canvas }, func(c *Colors, v string) { c.Canvas = v }},
	{"row", "选中行的底色", func(c Colors) string { return c.Row }, func(c *Colors, v string) { c.Row = v }},
	{"index", "序号", func(c Colors) string { return c.Index }, func(c *Colors, v string) { c.Index = v }},
	{"start", "开始时间", func(c Colors) string { return c.Start }, func(c *Colors, v string) { c.Start = v }},
	{"end", "结束时间", func(c Colors) string { return c.End }, func(c *Colors, v string) { c.End = v }},
	{"duration", "耗时", func(c Colors) string { return c.Duration }, func(c *Colors, v string) { c.Duration = v }},
	{"crossed", "耗时，跨午夜时", func(c Colors) string { return c.Crossed }, func(c *Colors, v string) { c.Crossed = v }},
	{"field", "光标所在的块：时间列只铺半个数值", func(c Colors) string { return c.Field }, func(c *Colors, v string) { c.Field = v }},
	{"selected", "选中行的内容文字", func(c Colors) string { return c.Selected }, func(c *Colors, v string) { c.Selected = v }},
	{"editing", "内容文字，正在编辑时", func(c Colors) string { return c.Editing }, func(c *Colors, v string) { c.Editing = v }},
	{"text", "日志内容文字", func(c Colors) string { return c.Text }, func(c *Colors, v string) { c.Text = v }},
	{"ink", "field 那块底色上的深色文字", func(c Colors) string { return c.Ink }, func(c *Colors, v string) { c.Ink = v }},
	{"dim", "次要文字，与还没填的时间、算不出的耗时", func(c Colors) string { return c.Dim }, func(c *Colors, v string) { c.Dim = v }},
	{"title", "页面标题", func(c Colors) string { return c.Title }, func(c *Colors, v string) { c.Title = v }},
	{"warn", "行首的 >、日期页的「今天」、帮助页小标题", func(c Colors) string { return c.Warn }, func(c *Colors, v string) { c.Warn = v }},
	{"status", "底部按键提示条底色", func(c Colors) string { return c.Status }, func(c *Colors, v string) { c.Status = v }},
	{"divider", "分隔线", func(c Colors) string { return c.Divider }, func(c *Colors, v string) { c.Divider = v }},
}

// Catppuccin is Catppuccin Mocha: the palette's accents as text on its own base,
// the selected row raised on surface0 the way Catppuccin apps raise a panel, pink
// for the one block the cursor sits on, and the status bar sunk to mantle.
func Catppuccin() Colors {
	return Colors{
		Canvas:   "#1e1e2e",
		Row:      "#313244",
		Index:    "#f38ba8",
		Start:    "#fab387",
		End:      "#f9e2af",
		Duration: "#a6e3a1",
		Crossed:  "#eba0ac",
		Field:    "#f5c2e7",
		Selected: "#cdd6f4",
		Editing:  "#74c7ec",
		Text:     "#bac2de",
		Ink:      "#1e1e2e",
		Dim:      "#7f849c",
		Title:    "#cdd6f4",
		Warn:     "#cba6f7",
		Status:   "#181825",
		Divider:  "#45475a",
	}
}

// OneDark is Atom's One Dark: the syntax colours as text on the editor
// background, a number in the orange One Dark gives constants, the selected row
// on the line-highlight grey, and the yellow of a class name for the block the
// cursor sits on. Two accents are asked to serve twice over — the title and the
// selected content both want the brightest grey, the marker and the cursor block
// both want the yellow — and each pair sits far enough apart on screen to read.
func OneDark() Colors {
	return Colors{
		Canvas:   "#282c34",
		Row:      "#3e4451",
		Index:    "#d19a66",
		Start:    "#61afef",
		End:      "#56b6c2",
		Duration: "#98c379",
		Crossed:  "#e06c75",
		Field:    "#e5c07b",
		Selected: "#d7dae0",
		Editing:  "#c678dd",
		Text:     "#abb2bf",
		Ink:      "#282c34",
		Dim:      "#7f848e",
		Title:    "#d7dae0",
		Warn:     "#e5c07b",
		Status:   "#21252b",
		Divider:  "#4b5263",
	}
}

// theme is one of the looks orgmaid carries, named as the config file names it.
type theme struct {
	name   string
	colors Colors
}

// themes is the shelf of built-in looks. The first one is what a file that
// names no theme gets.
var themes = []theme{
	{"catppuccin", Catppuccin()},
	{"onedark", OneDark()},
}

// Default is the palette orgmaid starts from, and the one it falls back to when
// the file cannot be read: the first theme on the shelf.
func Default() Colors { return themes[0].colors }

// Names lists the built-in themes in the order they are offered.
func Names() []string {
	names := make([]string, len(themes))
	for i, t := range themes {
		names[i] = t.name
	}
	return names
}

// themeColors resolves a name written in the file. Case does not matter.
func themeColors(name string) (Colors, bool) {
	for _, t := range themes {
		if strings.EqualFold(t.name, strings.TrimSpace(name)) {
			return t.colors, true
		}
	}
	return Colors{}, false
}

var hex = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// Template is the file written on first run. The theme is the one live line in
// it, and the colours come commented out, each under its own label: switching
// looks costs one word, and taking over a single colour costs one uncomment.
// Writing the colours live instead would pin all seventeen of them and leave
// the theme line with nothing to say.
func Template() string {
	ship := themes[0]

	var b strings.Builder
	b.WriteString("# orgmaid 界面配色。改完保存，重启 orgmaid 生效。\n\n")
	fmt.Fprintf(&b, "# 主题，可选 %s。\ntheme = %q\n\n", strings.Join(Names(), "、"), ship.name)
	fmt.Fprintf(&b, "# 下面是 %s 的取值：去掉行首的 # 改掉，就盖过主题里的这一个颜色。\n[colors]\n", ship.name)
	for _, e := range entries {
		fmt.Fprintf(&b, "# %s\n# %s = %q\n", e.note, e.key, e.get(ship.colors))
	}
	return b.String()
}

// DefaultPath is ~/.orgmaid/config.toml.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".orgmaid", "config.toml"), nil
}

// Load reads path. A file that is not there yet is not an error: the defaults
// stand. Every line that can be understood is applied, and whatever could not
// come back as one error, so a typo costs one colour rather than the program.
//
// The theme line picks the whole palette and the [colors] lines that follow
// take over single colours of it.
func Load(path string) (Colors, error) {
	c := Default()

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}

	var problems []string
	section := ""
	for n, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			section = strings.Trim(line, "[]")
			continue
		}

		key, value, ok := assign(line)
		if !ok {
			problems = append(problems, fmt.Sprintf("第 %d 行读不懂：%s", n+1, line))
			continue
		}
		if key == "theme" {
			if section != "" {
				problems = append(problems, fmt.Sprintf("第 %d 行的 theme 要写在 [colors] 之前", n+1))
				continue
			}
			picked, found := themeColors(value)
			if !found {
				problems = append(problems, fmt.Sprintf("第 %d 行没有这个主题：%s（可选 %s）",
					n+1, value, strings.Join(Names(), "、")))
				continue
			}
			// Only a line above every section can name a theme, so no colour
			// applied so far is lost by starting over from this palette.
			c = picked
			continue
		}
		if section != "colors" {
			problems = append(problems, fmt.Sprintf("第 %d 行在 [colors] 段之外：%s", n+1, key))
			continue
		}
		entry := find(key)
		if entry == nil {
			problems = append(problems, fmt.Sprintf("第 %d 行没有这个颜色：%s", n+1, key))
			continue
		}
		if !hex.MatchString(value) {
			problems = append(problems, fmt.Sprintf("第 %d 行的 %s 不是 #rrggbb：%s", n+1, key, value))
			continue
		}
		entry.set(&c, value)
	}

	if len(problems) > 0 {
		return c, errors.New(strings.Join(problems, "；"))
	}
	return c, nil
}

// Ensure writes the template to path when nothing is there yet, so the settings
// are there to edit without anyone looking them up. An existing file is never
// touched.
func Ensure(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(Template()), 0o600)
}

func find(key string) *entry {
	for i := range entries {
		if entries[i].key == key {
			return &entries[i]
		}
	}
	return nil
}

// assign splits `key = "value"`. Only a quoted value counts, which keeps the
// leading # of a hex colour from reading as the start of a comment.
func assign(line string) (key, value string, ok bool) {
	before, after, found := strings.Cut(line, "=")
	if !found {
		return "", "", false
	}
	after = strings.TrimSpace(after)
	if !strings.HasPrefix(after, `"`) {
		return "", "", false
	}
	quoted, _, found := strings.Cut(after[1:], `"`)
	if !found {
		return "", "", false
	}
	return strings.TrimSpace(before), quoted, true
}
