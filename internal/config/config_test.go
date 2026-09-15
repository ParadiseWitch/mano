package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// paletteMap flattens a Colors into file-key -> value so failures name the
// colour that went wrong instead of dumping a 17-field struct.
func paletteMap(c Colors) map[string]string {
	m := make(map[string]string, len(entries))
	for _, e := range entries {
		m[e.key] = e.get(c)
	}
	return m
}

// diffPalette lists the keys where got and want disagree. An empty result means
// the two palettes are identical.
func diffPalette(got, want Colors) []string {
	g, w := paletteMap(got), paletteMap(want)
	var bad []string
	for k, v := range g {
		if w[k] != v {
			bad = append(bad, fmt.Sprintf("%s: got %q want %q", k, v, w[k]))
		}
	}
	sort.Strings(bad)
	return bad
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	return path
}

func TestEnsureThenLoadRoundTrip(t *testing.T) {
	// Nested on purpose: Ensure has to create the directory it writes into.
	path := filepath.Join(t.TempDir(), "sub", "dir", "config.toml")
	if err := Ensure(path); err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load of the template we just wrote: %v", err)
	}
	if bad := diffPalette(got, Default()); len(bad) > 0 {
		t.Errorf("template does not load back as the shipped palette: %s", strings.Join(bad, ", "))
	}

	if runtime.GOOS == "windows" {
		// Windows has no permission bits: os.Stat reports every file as 0666
		// no matter what mode was asked for.
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Errorf("config file mode = %v, want no group or other access", info.Mode().Perm())
	}
	dir, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if dir.Mode().Perm()&0o077 != 0 {
		t.Errorf("config dir mode = %v, want no group or other access", dir.Mode().Perm())
	}
}

func TestLoadMissingFileIsNotAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "never", "written", "config.toml")
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load of a missing file = %v, want nil", err)
	}
	if bad := diffPalette(got, Default()); len(bad) > 0 {
		t.Errorf("missing file should leave the defaults, got: %s", strings.Join(bad, ", "))
	}
}

func TestLoadTakesTheThemeItIsGiven(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    Colors
	}{
		{name: "the shipped one", content: "theme = \"catppuccin\"\n", want: Catppuccin()},
		{name: "the other one", content: "theme = \"onedark\"\n", want: OneDark()},
		{name: "case does not matter", content: "theme = \"OneDark\"\n", want: OneDark()},
		{name: "crlf", content: "theme = \"onedark\"\r\n", want: OneDark()},
		{name: "the shipped template", content: Template(), want: Default()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Load(writeConfig(t, tc.content))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if bad := diffPalette(got, tc.want); len(bad) > 0 {
				t.Errorf("palette came back wrong: %s", strings.Join(bad, ", "))
			}
		})
	}
}

// TestColorsBeatTheTheme is the reason the template keeps its colours commented
// out: one uncommented colour is a choice about that colour, not about the theme.
func TestColorsBeatTheTheme(t *testing.T) {
	content := "theme = \"onedark\"\n\n[colors]\n# 序号列\nindex = \"#ff0000\"\n"
	got, err := Load(writeConfig(t, content))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	want := OneDark()
	want.Index = "#ff0000"
	if bad := diffPalette(got, want); len(bad) > 0 {
		t.Errorf("the theme should carry every colour but the one written down: %s", strings.Join(bad, ", "))
	}
}

func TestUnknownThemeFallsBackAndSaysWhatThereIs(t *testing.T) {
	content := "theme = \"solarized\"\n\n[colors]\nindex = \"#ff0000\"\n"
	got, err := Load(writeConfig(t, content))
	if err == nil {
		t.Fatal("Load accepted a theme nobody ships")
	}
	for _, frag := range []string{"第 1 行", "没有这个主题", "solarized", "catppuccin", "onedark"} {
		if !strings.Contains(err.Error(), frag) {
			t.Errorf("error %q does not mention %q", err, frag)
		}
	}

	// Only the theme is lost; the rest of the file still counts.
	want := Default()
	want.Index = "#ff0000"
	if bad := diffPalette(got, want); len(bad) > 0 {
		t.Errorf("a rejected theme should leave the defaults standing: %s", strings.Join(bad, ", "))
	}
}

// TestThemesAreComplete keeps a palette from shipping half-finished: a colour
// left empty paints nothing at all, and nothing else here would notice.
func TestThemesAreComplete(t *testing.T) {
	for _, th := range themes {
		t.Run(th.name, func(t *testing.T) {
			for _, e := range entries {
				if v := e.get(th.colors); !hex.MatchString(v) {
					t.Errorf("%s = %q, want #rrggbb", e.key, v)
				}
			}
		})
	}

	if bad := diffPalette(Default(), themes[0].colors); len(bad) > 0 {
		t.Errorf("Default is not the first theme on the shelf: %s", strings.Join(bad, ", "))
	}
	if got := Names(); len(got) != len(themes) || got[0] != themes[0].name {
		t.Errorf("Names() = %v, want one name per theme in shelf order", got)
	}
}

func TestLoadKeepsGoodLinesWhenOneIsBad(t *testing.T) {
	content := `# mano 界面配色。改完保存，重启 mano 生效。

[colors]
index = "#ff0000"
这一行根本不是赋值
title = "#00ff00"
`
	path := writeConfig(t, content)
	got, err := Load(path)
	if err == nil {
		t.Fatal("Load accepted a file with a garbage line")
	}
	if !strings.Contains(err.Error(), "第 5 行") || !strings.Contains(err.Error(), "这一行根本不是赋值") {
		t.Errorf("error should point at the offending line, got: %v", err)
	}

	want := Default()
	want.Index = "#ff0000"
	want.Title = "#00ff00"
	if bad := diffPalette(got, want); len(bad) > 0 {
		t.Errorf("the two good lines must land and nothing else may move: %s", strings.Join(bad, ", "))
	}
}

func TestLoadAggregatesEveryProblemIntoOneError(t *testing.T) {
	content := "[colors]\nindex = \"nope\"\nbogus = \"#ff0000\"\nstart = \"#00ff00\"\n"
	got, err := Load(writeConfig(t, content))
	if err == nil {
		t.Fatal("want an error for the two bad lines")
	}
	if n := strings.Count(err.Error(), "；"); n != 1 {
		t.Errorf("two problems should be joined by exactly one separator, got %d in %q", n, err)
	}
	for _, frag := range []string{"第 2 行", "第 3 行", "index", "bogus"} {
		if !strings.Contains(err.Error(), frag) {
			t.Errorf("error %q does not mention %q", err, frag)
		}
	}
	want := Default()
	want.Start = "#00ff00"
	if bad := diffPalette(got, want); len(bad) > 0 {
		t.Errorf("only start should have changed: %s", strings.Join(bad, ", "))
	}
}

func TestLoadRejectsLinesItShouldNotApply(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name:    "unquoted value would swallow the hex as a comment",
			content: "[colors]\nindex = #ff0000\n",
			wantErr: "读不懂",
		},
		{
			name:    "unterminated quote",
			content: "[colors]\nindex = \"#ff0000\n",
			wantErr: "读不懂",
		},
		{
			name:    "unknown key",
			content: "[colors]\nnotacolour = \"#ff0000\"\n",
			wantErr: "没有这个颜色",
		},
		{
			name:    "key before any section header",
			content: "index = \"#ff0000\"\n[colors]\n",
			wantErr: "在 [colors] 段之外",
		},
		{
			name:    "key under a different section",
			content: "[palette]\nindex = \"#ff0000\"\n",
			wantErr: "在 [colors] 段之外",
		},
		{
			name:    "three digit shorthand",
			content: "[colors]\nindex = \"#f00\"\n",
			wantErr: "不是 #rrggbb",
		},
		{
			name:    "empty value",
			content: "[colors]\nindex = \"\"\n",
			wantErr: "不是 #rrggbb",
		},
		{
			name:    "non hex digits",
			content: "[colors]\nindex = \"#gggggg\"\n",
			wantErr: "不是 #rrggbb",
		},
		{
			name:    "theme below the section it should be picking for",
			content: "[colors]\ntheme = \"onedark\"\n",
			wantErr: "要写在 [colors] 之前",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Load(writeConfig(t, tc.content))
			if err == nil {
				t.Fatalf("Load(%q) = nil error, want a complaint naming the line", tc.content)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not contain %q", err, tc.wantErr)
			}
			if bad := diffPalette(got, Default()); len(bad) > 0 {
				t.Errorf("a rejected line must not move any colour: %s", strings.Join(bad, ", "))
			}
		})
	}
}

func TestEnsureLeavesAnExistingFileAlone(t *testing.T) {
	// A user who has edited the file must never lose the edit to a later run.
	const seeded = "[colors]\nindex = \"#ffffff\"\n"
	path := writeConfig(t, seeded)

	if err := Ensure(path); err != nil {
		t.Fatalf("Ensure over an existing file: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(raw) != seeded {
		t.Errorf("Ensure rewrote the file:\n got: %q\nwant: %q", raw, seeded)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Index != "#ffffff" {
		t.Errorf("Index = %q, want the user's white to survive", got.Index)
	}
}

// TestTemplateIsLoadableAndLabelsEveryEntry keeps the first-run file honest: it
// loads clean, it names the shipped theme once, and it offers every colour under
// its own label — commented out, because a live colour would overrule the theme.
func TestTemplateIsLoadableAndLabelsEveryEntry(t *testing.T) {
	text := Template()

	got, err := Load(writeConfig(t, text))
	if err != nil {
		t.Fatalf("the shipped template does not load clean: %v", err)
	}
	if bad := diffPalette(got, Default()); len(bad) > 0 {
		t.Errorf("template does not carry the defaults: %s", strings.Join(bad, ", "))
	}

	var live, labels, offered int
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case line == "" || strings.HasPrefix(line, "["):
			continue
		case !strings.HasPrefix(line, "#"):
			live++
			key, value, ok := assign(line)
			if !ok || key != "theme" || value != themes[0].name {
				t.Errorf("the one live line should be theme = %q, got %q", themes[0].name, line)
			}
		default:
			key, value, ok := assign(strings.TrimSpace(strings.TrimPrefix(line, "#")))
			if !ok {
				labels++
				continue
			}
			offered++
			e := find(key)
			if e == nil {
				t.Fatalf("template offers unknown key %q", key)
			}
			if !hex.MatchString(value) {
				t.Errorf("template value for %s is not #rrggbb: %q", key, value)
			}
			if want := e.get(Default()); value != want {
				t.Errorf("template offers %s = %q, want the shipped theme's %q", key, value, want)
			}
		}
	}

	if live != 1 {
		t.Errorf("template has %d live lines, want the theme and nothing else", live)
	}
	// One label and one commented value per colour, plus the three comments that
	// introduce the file. A colour added to Colors has to show up here or these
	// counts drift apart from len(entries).
	if offered != len(entries) {
		t.Errorf("template offers %d colours, want len(entries) = %d", offered, len(entries))
	}
	if labels != len(entries)+3 {
		t.Errorf("template has %d comment lines, want %d labels plus three introductions", labels, len(entries))
	}
	for _, e := range entries {
		if !strings.Contains(text, "# "+e.note) {
			t.Errorf("template never labels %q with its note %q", e.key, e.note)
		}
	}
}

// TestEntriesCoverEveryColorField keeps the shared read/write table honest: a
// colour added to Colors but not to entries would silently stop being
// configurable, and nothing else here would notice.
func TestEntriesCoverEveryColorField(t *testing.T) {
	kind := reflect.TypeOf(Colors{})
	seen := map[string]string{}
	for _, e := range entries {
		field := fieldOf(t, e)
		if prev, dup := seen[field]; dup {
			t.Errorf("keys %q and %q both write Colors.%s", prev, e.key, field)
		}
		seen[field] = e.key
	}
	for i := 0; i < kind.NumField(); i++ {
		if name := kind.Field(i).Name; seen[name] == "" {
			t.Errorf("Colors.%s has no config key, so the user cannot change it", name)
		}
	}
}

func fieldOf(t *testing.T, e entry) string {
	t.Helper()
	const sentinel = "#010203"
	for _, v := range paletteMap(Default()) {
		if v == sentinel {
			t.Fatalf("sentinel %s collides with a default colour", sentinel)
		}
	}
	c := Default()
	e.set(&c, sentinel)
	v := reflect.ValueOf(c)
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).String() == sentinel {
			return v.Type().Field(i).Name
		}
	}
	t.Fatalf("entry %q writes no field of Colors", e.key)
	return ""
}

func TestLoadAcceptsCRLF(t *testing.T) {
	// The file lives in ~/.mano/config.toml and gets edited in a Windows
	// editor, so carriage returns are the normal case, not a corner one.
	twoKeys := Default()
	twoKeys.Index = "#aabbcc"
	twoKeys.Title = "#ddeeff"

	cases := []struct {
		name    string
		content string
		want    Colors
	}{
		{
			name:    "one assignment line",
			content: "[colors]\r\nindex = \"#aabbcc\"\r\n",
			want:    withIndex(Default()),
		},
		{
			name:    "no final newline",
			content: "[colors]\r\nindex = \"#aabbcc\"",
			want:    withIndex(Default()),
		},
		{
			name:    "several lines",
			content: "[colors]\r\nindex = \"#aabbcc\"\r\ntitle = \"#ddeeff\"\r\n",
			want:    twoKeys,
		},
		{
			name:    "the whole template",
			content: strings.ReplaceAll(Template(), "\n", "\r\n"),
			want:    Default(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Load(writeConfig(t, tc.content))
			if err != nil {
				t.Fatalf("Load of CRLF content: %v", err)
			}
			if bad := diffPalette(got, tc.want); len(bad) > 0 {
				t.Errorf("CRLF palette came back wrong: %s", strings.Join(bad, ", "))
			}
		})
	}
}

func withIndex(c Colors) Colors {
	c.Index = "#aabbcc"
	return c
}

func TestDefaultPathPointsAtManoDir(t *testing.T) {
	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	want := filepath.Join(".mano", "config.toml")
	if !strings.HasSuffix(path, want) {
		t.Errorf("DefaultPath = %q, want it to end in %q", path, want)
	}
}
