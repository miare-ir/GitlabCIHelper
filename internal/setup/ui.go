package setup

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
)

type setupUI struct {
	out io.Writer

	banner       *color.Color
	heading      *color.Color
	muted        *color.Color
	accent       *color.Color
	info         *color.Color
	success      *color.Color
	warning      *color.Color
	danger       *color.Color
	prompt       *color.Color
	border       *color.Color
	tableHdr     *color.Color
	progressDone *color.Color
	progressNow  *color.Color
	progressTodo *color.Color
	diffMeta     *color.Color
	diffHunk     *color.Color
	diffAdd      *color.Color
	diffDel      *color.Color
}

func newSetupUI(out io.Writer) setupUI {
	useColor := outputSupportsColor(out)
	makeColor := func(attrs ...color.Attribute) *color.Color {
		c := color.New(attrs...)
		if !useColor {
			c.DisableColor()
		}
		return c
	}

	return setupUI{
		out:          out,
		banner:       makeColor(color.FgHiCyan, color.Bold),
		heading:      makeColor(color.FgHiBlue, color.Bold),
		muted:        makeColor(color.FgHiBlack),
		accent:       makeColor(color.FgHiBlue, color.Bold),
		info:         makeColor(color.FgHiCyan),
		success:      makeColor(color.FgHiGreen, color.Bold),
		warning:      makeColor(color.FgHiYellow, color.Bold),
		danger:       makeColor(color.FgHiRed, color.Bold),
		prompt:       makeColor(color.FgHiWhite, color.Bold),
		border:       makeColor(color.FgHiBlack),
		tableHdr:     makeColor(color.FgHiBlue, color.Bold),
		progressDone: makeColor(color.FgHiGreen, color.Bold),
		progressNow:  makeColor(color.FgHiBlue, color.Bold),
		progressTodo: makeColor(color.FgHiBlack),
		diffMeta:     makeColor(color.FgHiCyan),
		diffHunk:     makeColor(color.FgHiYellow),
		diffAdd:      makeColor(color.FgHiGreen),
		diffDel:      makeColor(color.FgHiRed),
	}
}

func outputSupportsColor(out io.Writer) bool {
	if force, ok := os.LookupEnv("CLICOLOR_FORCE"); ok {
		if force != "0" {
			return true
		}
		return false
	}
	if os.Getenv("NO_COLOR") != "" || os.Getenv("CLICOLOR") == "0" || strings.EqualFold(os.Getenv("TERM"), "dumb") {
		return false
	}
	file, ok := out.(*os.File)
	if !ok {
		return false
	}
	fd := file.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

func (ui setupUI) bannerPanel(title string, lines []string) {
	lineWidth := 54
	if w := runeLen(title) + 4; w > lineWidth {
		lineWidth = w
	}
	for _, line := range lines {
		if w := runeLen(line) + 4; w > lineWidth {
			lineWidth = w
		}
	}
	border := strings.Repeat("━", lineWidth)
	fmt.Fprintln(ui.out, ui.banner.Sprintf("┏%s┓", border))
	fmt.Fprintf(ui.out, "%s %s\n", ui.banner.Sprint("┃"), ui.banner.Sprint(centerText(title, lineWidth)))
	fmt.Fprintln(ui.out, ui.banner.Sprintf("┣%s┫", border))
	for _, line := range lines {
		fmt.Fprintf(ui.out, "%s %s\n", ui.banner.Sprint("┃"), padRight(line, lineWidth))
	}
	fmt.Fprintln(ui.out, ui.banner.Sprintf("┗%s┛", border))
}

func (ui setupUI) section(title string, subtitle string) {
	fmt.Fprintln(ui.out, "")
	fmt.Fprintln(ui.out, ui.heading.Sprintf("◆ %s", title))
	if strings.TrimSpace(subtitle) != "" {
		fmt.Fprintln(ui.out, ui.muted.Sprintf("  %s", subtitle))
	}
	fmt.Fprintln(ui.out, ui.border.Sprint(strings.Repeat("─", maxInt(24, runeLen(title)+6))))
}

func (ui setupUI) progress(step int, total int, label string) {
	if total < 1 {
		total = 1
	}
	if step < 0 {
		step = 0
	}
	if step > total {
		step = total
	}

	var track strings.Builder
	for idx := 1; idx <= total; idx++ {
		if idx < step || (idx == step && step == total) {
			track.WriteString(ui.progressDone.Sprint("●"))
			continue
		}
		if idx == step {
			track.WriteString(ui.progressNow.Sprint("◉"))
		} else {
			track.WriteString(ui.progressTodo.Sprint("○"))
		}
	}

	prefix := ui.progressNow.Sprintf("STEP %d/%d", step, total)
	fmt.Fprintf(ui.out, "%s %s %s\n", prefix, track.String(), label)
}

func (ui setupUI) infoLine(msg string) string {
	return fmt.Sprintf("%s %s", ui.info.Sprint("ℹ"), msg)
}

func (ui setupUI) successLine(msg string) string {
	return fmt.Sprintf("%s %s", ui.success.Sprint("✓"), msg)
}

func (ui setupUI) warningLine(msg string) string {
	return fmt.Sprintf("%s %s", ui.warning.Sprint("⚠"), msg)
}

func (ui setupUI) dangerLine(msg string) string {
	return fmt.Sprintf("%s %s", ui.danger.Sprint("✗"), msg)
}

func (ui setupUI) promptLabel(label string) string {
	return ui.prompt.Sprintf("❯ %s", label)
}

func (ui setupUI) printPanel(title string, lines []string) {
	width := runeLen(title)
	for _, line := range lines {
		if w := runeLen(line); w > width {
			width = w
		}
	}
	width = maxInt(width, 24)

	fmt.Fprintln(ui.out, "")
	fmt.Fprintln(ui.out, ui.border.Sprintf("┌%s┐", strings.Repeat("─", width+2)))
	fmt.Fprintf(ui.out, "%s %s %s\n", ui.border.Sprint("│"), ui.heading.Sprint(padRight(title, width)), ui.border.Sprint("│"))
	fmt.Fprintln(ui.out, ui.border.Sprintf("├%s┤", strings.Repeat("─", width+2)))
	for _, line := range lines {
		fmt.Fprintf(ui.out, "%s %s %s\n", ui.border.Sprint("│"), padRight(line, width), ui.border.Sprint("│"))
	}
	fmt.Fprintln(ui.out, ui.border.Sprintf("└%s┘", strings.Repeat("─", width+2)))
}

func (ui setupUI) printTable(headers []string, rows [][]string) {
	if len(headers) == 0 {
		return
	}

	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = runeLen(h)
	}
	for _, row := range rows {
		for i := range headers {
			value := tableCell(row, i)
			if w := runeLen(value); w > widths[i] {
				widths[i] = w
			}
		}
	}

	drawDivider := func(left, middle, right string) {
		parts := make([]string, len(widths))
		for i, w := range widths {
			parts[i] = strings.Repeat("─", w+2)
		}
		fmt.Fprintf(ui.out, "%s%s%s\n", left, strings.Join(parts, middle), right)
	}
	drawRow := func(values []string, header bool) {
		parts := make([]string, len(widths))
		for i, w := range widths {
			cell := padRight(tableCell(values, i), w)
			if header {
				cell = ui.tableHdr.Sprint(cell)
			} else if i == 0 {
				cell = ui.styleStatusCell(cell)
			}
			parts[i] = fmt.Sprintf(" %s ", cell)
		}
		fmt.Fprintf(ui.out, "│%s│\n", strings.Join(parts, "│"))
	}

	fmt.Fprintln(ui.out, "")
	drawDivider("┌", "┬", "┐")
	drawRow(headers, true)
	drawDivider("├", "┼", "┤")
	for _, row := range rows {
		drawRow(row, false)
	}
	drawDivider("└", "┴", "┘")
}

func (ui setupUI) styleStatusCell(cell string) string {
	trimmed := strings.ToLower(strings.TrimSpace(cell))
	switch {
	case strings.Contains(trimmed, "create"):
		return ui.success.Sprint(cell)
	case strings.Contains(trimmed, "update"):
		return ui.warning.Sprint(cell)
	case strings.Contains(trimmed, "keep"):
		return ui.info.Sprint(cell)
	default:
		return cell
	}
}

func (ui setupUI) formatDiffLine(line string) string {
	switch {
	case strings.HasPrefix(line, "---"), strings.HasPrefix(line, "+++"):
		return ui.diffMeta.Sprint(line)
	case strings.HasPrefix(line, "@@"):
		return ui.diffHunk.Sprint(line)
	case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
		return ui.diffAdd.Sprint(line)
	case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
		return ui.diffDel.Sprint(line)
	default:
		return line
	}
}

func centerText(value string, width int) string {
	if width <= 0 {
		return value
	}
	diff := width - runeLen(value)
	if diff <= 0 {
		return value
	}
	left := diff / 2
	right := diff - left
	return strings.Repeat(" ", left) + value + strings.Repeat(" ", right)
}

func padRight(value string, width int) string {
	if width <= 0 {
		return value
	}
	padding := width - runeLen(value)
	if padding <= 0 {
		return value
	}
	return value + strings.Repeat(" ", padding)
}

func tableCell(row []string, idx int) string {
	if idx >= 0 && idx < len(row) {
		return row[idx]
	}
	return ""
}

func runeLen(value string) int {
	return utf8.RuneCountInString(value)
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func parseBoolInput(value string) (bool, bool) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return false, false
	}
	switch normalized {
	case "y", "yes", "true", "t", "1", "on":
		return true, true
	case "n", "no", "false", "f", "0", "off":
		return false, true
	default:
		asInt, err := strconv.Atoi(normalized)
		if err == nil {
			return asInt != 0, true
		}
	}
	return false, false
}
