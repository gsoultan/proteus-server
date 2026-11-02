// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

package templates

import (
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	proteusv1 "github.com/gsoultan/proteus-server/gen/proteus/v1"
)

// A visual template is compiled into the same HTML with Go template actions
// that a code template holds, so rendering, limits and escaping are shared.
// Literal text from blocks is HTML-escaped at compile time; only the
// {{...}} actions a user typed pass through, and the template parser then
// checks those. Field paths become dotted lookups: "$.client.name" at the
// top level, ".qty" inside a repeated table or section.

// Compile limits.
const (
	maxBlocks      = 500
	maxLayoutDepth = 4
	maxBlockText   = 20_000
	maxColumns     = 20
	maxRows        = 50
	maxRegionMm    = 150
)

// LayoutError is a problem in a layout, pointing at the block.
type LayoutError struct {
	BlockID string // may be empty
	Where   string // such as "body.blocks[2]"
	Msg     string
}

func (e *LayoutError) Error() string {
	if e.BlockID != "" {
		return fmt.Sprintf("layout: block %q: %s", e.BlockID, e.Msg)
	}
	return fmt.Sprintf("layout: %s: %s", e.Where, e.Msg)
}

// Theme defaults.
const (
	defaultFontFamily = "Inter, Helvetica, Arial, sans-serif"
	defaultFontSize   = 10.0
	defaultAccent     = "#3b5bdb"
	defaultTextColor  = "#212529"
	defaultMarginMm   = 15.0
	defaultHeaderMm   = 25.0
	defaultFooterMm   = 15.0
)

var (
	cssColorRe  = regexp.MustCompile(`^(#[0-9a-fA-F]{3,8}|[a-zA-Z]+|rgba?\([0-9.,\s%]+\)|hsla?\([0-9.,\s%deg]+\))$`)
	cssFamilyRe = regexp.MustCompile(`^[A-Za-z0-9 ,'"\-]+$`)
	actionRe    = regexp.MustCompile(`\{\{.*?\}\}`)
	definesRe   = regexp.MustCompile(`^\{\{-?\s*(template|define|block)\b`)
)

// FormatOf returns the number and date formats a layout's theme asks for.
func FormatOf(l *proteusv1.Layout) Format {
	if l == nil {
		return Format{}
	}
	return Format{Locale: l.GetTheme().GetLocale(), Currency: l.GetTheme().GetCurrency()}
}

// Compile turns a layout into HTML with template actions. page gives the
// page size and margins the template defaults to; the compiled CSS @page
// rule carries them plus the header and footer bands.
func Compile(l *proteusv1.Layout, page *proteusv1.PageOptions) ([]byte, error) {
	c := &compiler{}
	if err := c.checkTheme(l.GetTheme()); err != nil {
		return nil, err
	}
	hdr, ftr := regionHeight(l.GetHeader(), defaultHeaderMm), regionHeight(l.GetFooter(), defaultFooterMm)
	if hdr > maxRegionMm || ftr > maxRegionMm {
		return nil, &LayoutError{Where: "header/footer", Msg: fmt.Sprintf("height_mm is above %d", maxRegionMm)}
	}
	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n<html>\n<head>\n<meta charset=\"utf-8\">\n<style>\n")
	c.writeCSS(&b, l.GetTheme(), page, hdr, ftr)
	b.WriteString("</style>\n</head>\n<body>\n")
	if len(l.GetHeader().GetBlocks()) > 0 {
		b.WriteString(`<header class="pt-header">` + "\n")
		if err := c.region(&b, l.GetHeader(), "header", "$", 1); err != nil {
			return nil, err
		}
		b.WriteString("</header>\n")
	}
	if len(l.GetFooter().GetBlocks()) > 0 {
		b.WriteString(`<footer class="pt-footer">` + "\n")
		if err := c.region(&b, l.GetFooter(), "footer", "$", 1); err != nil {
			return nil, err
		}
		b.WriteString("</footer>\n")
	}
	b.WriteString(`<main class="pt-body">` + "\n")
	if err := c.region(&b, l.GetBody(), "body", "$", 1); err != nil {
		return nil, err
	}
	b.WriteString("</main>\n</body>\n</html>\n")
	return []byte(b.String()), nil
}

func regionHeight(r *proteusv1.Region, def float64) float64 {
	if len(r.GetBlocks()) == 0 {
		return 0
	}
	if h := r.GetHeightMm(); h > 0 {
		return h
	}
	return def
}

type compiler struct {
	blocks int
	ids    map[string]bool
}

func (c *compiler) checkTheme(t *proteusv1.Theme) error {
	fail := func(msg string) error { return &LayoutError{Where: "theme", Msg: msg} }
	if f := t.GetFontFamily(); f != "" && (len(f) > 200 || !cssFamilyRe.MatchString(f)) {
		return fail("font_family must be a list of font names")
	}
	if s := t.GetFontSizePt(); s < 0 || s > 72 {
		return fail("font_size_pt must be between 0 and 72")
	}
	for name, v := range map[string]string{"accent_color": t.GetAccentColor(), "text_color": t.GetTextColor()} {
		if v != "" && !cssColorRe.MatchString(v) {
			return fail(name + " must be a CSS colour")
		}
	}
	if l := t.GetLocale(); l != "" && !KnownLocale(l) {
		return fail("unknown locale " + strconv.Quote(l))
	}
	if cur := t.GetCurrency(); cur != "" && (len(cur) != 3 || strings.ToUpper(cur) != cur) {
		return fail("currency must be a three-letter ISO 4217 code")
	}
	return nil
}

func (c *compiler) writeCSS(b *strings.Builder, t *proteusv1.Theme, page *proteusv1.PageOptions, hdr, ftr float64) {
	font, size, accent, text := t.GetFontFamily(), t.GetFontSizePt(), t.GetAccentColor(), t.GetTextColor()
	if font == "" {
		font = defaultFontFamily
	}
	if size <= 0 {
		size = defaultFontSize
	}
	if accent == "" {
		accent = defaultAccent
	}
	if text == "" {
		text = defaultTextColor
	}
	top, right, bottom, left := defaultMarginMm, defaultMarginMm, defaultMarginMm, defaultMarginMm
	if m := page.GetMargins(); m != nil {
		top, right, bottom, left = m.GetTop(), m.GetRight(), m.GetBottom(), m.GetLeft()
	} else if page != nil && page.MarginMm != nil {
		top, right, bottom, left = page.GetMarginMm(), page.GetMarginMm(), page.GetMarginMm(), page.GetMarginMm()
	}
	sizeName := map[proteusv1.PageSize]string{
		proteusv1.PageSize_PAGE_SIZE_A3: "A3", proteusv1.PageSize_PAGE_SIZE_A4: "A4", proteusv1.PageSize_PAGE_SIZE_A5: "A5",
		proteusv1.PageSize_PAGE_SIZE_LETTER: "letter", proteusv1.PageSize_PAGE_SIZE_LEGAL: "legal",
	}[page.GetSize()]
	if sizeName == "" {
		sizeName = "A4"
	}
	if page.GetLandscape() {
		sizeName += " landscape"
	}
	mm := func(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) + "mm" }
	fmt.Fprintf(b, "@page { size: %s; margin: %s %s %s %s; }\n", sizeName, mm(top+hdr), mm(right), mm(bottom+ftr), mm(left))
	fmt.Fprintf(b, ":root { --pt-accent: %s; --pt-text: %s; --pt-muted: #495057; --pt-rule: #dee2e6; }\n", accent, text)
	fmt.Fprintf(b, "body { margin: 0; font-family: %s; font-size: %spt; color: var(--pt-text); line-height: 1.4; }\n",
		font, strconv.FormatFloat(size, 'f', -1, 64))
	if hdr > 0 {
		fmt.Fprintf(b, ".pt-header { position: fixed; top: -%s; left: 0; right: 0; height: %s; box-sizing: border-box; padding-bottom: 3mm; }\n", mm(hdr), mm(hdr))
	}
	if ftr > 0 {
		fmt.Fprintf(b, ".pt-footer { position: fixed; bottom: -%s; left: 0; right: 0; height: %s; box-sizing: border-box; padding-top: 3mm; font-size: 0.85em; color: var(--pt-muted); }\n", mm(ftr), mm(ftr))
	}
	b.WriteString(layoutCSS)
}

// layoutCSS styles the pt- classes the compiler emits.
const layoutCSS = `.pt-block { margin: 0 0 5mm; }
.pt-block.sp-none { margin-bottom: 0; } .pt-block.sp-small { margin-bottom: 2mm; } .pt-block.sp-large { margin-bottom: 10mm; }
.al-center { text-align: center; } .al-right { text-align: right; }
.pt-h1, .pt-h2, .pt-h3 { margin: 0; color: var(--pt-accent); font-weight: 700; line-height: 1.2; }
.pt-h1 { font-size: 2.2em; letter-spacing: 0.02em; } .pt-h2 { font-size: 1.5em; } .pt-h3 { font-size: 1.15em; }
.pt-text p { margin: 0 0 0.6em; } .pt-text p:last-child { margin-bottom: 0; }
.pt-label { display: block; font-size: 0.75em; font-weight: 700; color: var(--pt-muted); text-transform: uppercase; letter-spacing: 0.06em; margin-bottom: 1pt; }
.pt-field .pt-value { font-weight: 600; }
.pt-img { display: block; max-width: 100%; }
.pt-table { width: 100%; border-collapse: collapse; }
.pt-table thead { display: table-header-group; }
.pt-table th { background: var(--pt-accent); color: #fff; text-align: left; padding: 4pt 6pt; font-size: 0.9em; font-weight: 600; }
.pt-table td { padding: 4pt 6pt; border-bottom: 1px solid var(--pt-rule); vertical-align: top; }
.pt-table tr { page-break-inside: avoid; }
.pt-table.striped tbody tr:nth-child(even) td { background: #f8f9fa; }
.pt-table .num { text-align: right; } .pt-table .ctr { text-align: center; }
.pt-empty { color: var(--pt-muted); font-style: italic; }
.pt-kv { display: grid; grid-template-columns: auto 1fr; gap: 2pt 10pt; }
.pt-kv .k { color: var(--pt-muted); } .pt-kv .v { font-weight: 600; }
.pt-totals { margin-left: auto; width: 45%; }
.pt-totals .row { display: flex; justify-content: space-between; padding: 2pt 0; }
.pt-totals .row .k { color: var(--pt-muted); }
.pt-totals .row.em { font-weight: 700; font-size: 1.2em; border-top: 1px solid #ced4da; margin-top: 2pt; padding-top: 4pt; color: var(--pt-text); }
.pt-totals .row.em .k { color: var(--pt-text); }
.pt-cols { display: flex; gap: 8mm; }
.pt-col { flex: 1 1 0; min-width: 0; }
.pt-sig { display: flex; gap: 12mm; margin-top: 10mm; }
.pt-sig .line { flex: 1 1 0; border-top: 1px solid var(--pt-muted); padding-top: 3pt; font-size: 0.85em; color: var(--pt-muted); }
.pt-divider { border: 0; border-top: 1px solid var(--pt-accent); margin: 0; }
.pt-qr { display: block; }
.pt-break { page-break-before: always; }
`

// region writes the blocks of r. scope is "$" at the top level and "."
// inside a repeated section, where paths refer to the item.
func (c *compiler) region(b *strings.Builder, r *proteusv1.Region, where, scope string, depth int) error {
	if depth > maxLayoutDepth {
		return &LayoutError{Where: where, Msg: fmt.Sprintf("blocks nest more than %d deep", maxLayoutDepth)}
	}
	for i, blk := range r.GetBlocks() {
		if err := c.block(b, blk, fmt.Sprintf("%s.blocks[%d]", where, i), scope, depth); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) block(b *strings.Builder, blk *proteusv1.Block, where, scope string, depth int) error {
	c.blocks++
	if c.blocks > maxBlocks {
		return &LayoutError{Where: where, Msg: fmt.Sprintf("more than %d blocks", maxBlocks)}
	}
	fail := func(msg string) error { return &LayoutError{BlockID: blk.GetId(), Where: where, Msg: msg} }
	if id := blk.GetId(); id != "" {
		if len(id) > 64 || !identRe.MatchString(strings.ReplaceAll(id, "-", "_")) {
			return fail("id must be letters, digits, _ or -")
		}
		if c.ids == nil {
			c.ids = map[string]bool{}
		}
		if c.ids[id] {
			return fail("id is used twice")
		}
		c.ids[id] = true
	}
	if blk.GetKind() == nil {
		return fail("block has no kind")
	}
	cond, err := c.condition(blk.GetShowIf(), scope, fail)
	if err != nil {
		return err
	}
	classes, style, err := c.style(blk.GetStyle(), fail)
	if err != nil {
		return err
	}
	if cond != "" {
		b.WriteString("{{if " + cond + "}}")
	}
	switch k := blk.GetKind().(type) {
	case *proteusv1.Block_PageBreak:
		b.WriteString(`<div class="pt-break"></div>` + "\n")
	case *proteusv1.Block_Spacer:
		h := k.Spacer.GetHeightMm()
		if h < 0 || h > 200 {
			return fail("spacer height_mm must be between 0 and 200")
		}
		if h == 0 {
			h = 5
		}
		fmt.Fprintf(b, `<div class="pt-spacer" style="height: %smm"></div>`+"\n", strconv.FormatFloat(h, 'f', -1, 64))
	default:
		open := `<div class="pt-block` + classes + `"`
		if id := blk.GetId(); id != "" {
			open += ` data-block="` + html.EscapeString(id) + `"`
		}
		if style != "" {
			open += ` style="` + style + `"`
		}
		b.WriteString(open + ">")
		if err := c.kind(b, blk, where, scope, depth, fail); err != nil {
			return err
		}
		b.WriteString("</div>\n")
	}
	if cond != "" {
		b.WriteString("{{end}}")
	}
	return nil
}

func (c *compiler) style(s *proteusv1.BlockStyle, fail func(string) error) (classes, style string, err error) {
	if s == nil {
		return "", "", nil
	}
	switch s.GetAlign() {
	case proteusv1.Align_ALIGN_CENTER:
		classes += " al-center"
	case proteusv1.Align_ALIGN_RIGHT:
		classes += " al-right"
	case proteusv1.Align_ALIGN_UNSPECIFIED, proteusv1.Align_ALIGN_LEFT:
	default:
		return "", "", fail("unknown align")
	}
	switch s.GetSpaceBelow() {
	case proteusv1.Spacing_SPACING_NONE:
		classes += " sp-none"
	case proteusv1.Spacing_SPACING_SMALL:
		classes += " sp-small"
	case proteusv1.Spacing_SPACING_LARGE:
		classes += " sp-large"
	case proteusv1.Spacing_SPACING_UNSPECIFIED, proteusv1.Spacing_SPACING_MEDIUM:
	default:
		return "", "", fail("unknown space_below")
	}
	var parts []string
	if fs := s.GetFontSizePt(); fs != 0 {
		if fs < 4 || fs > 120 {
			return "", "", fail("font_size_pt must be between 4 and 120")
		}
		parts = append(parts, "font-size: "+strconv.FormatFloat(fs, 'f', -1, 64)+"pt")
	}
	if s.GetBold() {
		parts = append(parts, "font-weight: 700")
	}
	if col := s.GetColor(); col != "" {
		if !cssColorRe.MatchString(col) {
			return "", "", fail("color must be a CSS colour")
		}
		parts = append(parts, "color: "+col)
	}
	if bg := s.GetBackground(); bg != "" {
		if !cssColorRe.MatchString(bg) {
			return "", "", fail("background must be a CSS colour")
		}
		parts = append(parts, "background: "+bg, "padding: 3mm")
	}
	return classes, strings.Join(parts, "; "), nil
}

// ref returns the template expression for a field path in scope.
func ref(scope, path string) string {
	if scope == "$" {
		return "$." + path
	}
	return "." + path
}

func (c *compiler) condition(cond *proteusv1.Condition, scope string, fail func(string) error) (string, error) {
	if cond == nil {
		return "", nil
	}
	if !validPath(cond.GetPath()) {
		return "", fail("show_if path must be dotted names such as status")
	}
	r := ref(scope, cond.GetPath())
	v := cond.GetValue()
	switch cond.GetOp() {
	case proteusv1.ConditionOp_CONDITION_OP_UNSPECIFIED, proteusv1.ConditionOp_CONDITION_OP_IS_SET:
		return r, nil
	case proteusv1.ConditionOp_CONDITION_OP_IS_EMPTY:
		return "not " + r, nil
	case proteusv1.ConditionOp_CONDITION_OP_EQUALS:
		return fmt.Sprintf("eq (printf \"%%v\" %s) %s", r, strconv.Quote(v)), nil
	case proteusv1.ConditionOp_CONDITION_OP_NOT_EQUALS:
		return fmt.Sprintf("ne (printf \"%%v\" %s) %s", r, strconv.Quote(v)), nil
	case proteusv1.ConditionOp_CONDITION_OP_GREATER, proteusv1.ConditionOp_CONDITION_OP_LESS:
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return "", fail("show_if value must be a number for greater and less")
		}
		op := "gt"
		if cond.GetOp() == proteusv1.ConditionOp_CONDITION_OP_LESS {
			op = "lt"
		}
		return fmt.Sprintf("%s (num %s) %s", op, r, strconv.FormatFloat(f, 'f', -1, 64)), nil
	}
	return "", fail("unknown show_if op")
}

// text escapes literal text and keeps {{...}} actions, so a block can hold
// placeholders but never markup.
func (c *compiler) text(s string, fail func(string) error) (string, error) {
	if utf8.RuneCountInString(s) > maxBlockText {
		return "", fail(fmt.Sprintf("text is longer than %d characters", maxBlockText))
	}
	var b strings.Builder
	last := 0
	for _, m := range actionRe.FindAllStringIndex(s, -1) {
		if definesRe.MatchString(s[m[0]:m[1]]) {
			return "", fail("text may not use {{template}}, {{define}} or {{block}}")
		}
		b.WriteString(html.EscapeString(s[last:m[0]]))
		b.WriteString(s[m[0]:m[1]])
		last = m[1]
	}
	b.WriteString(html.EscapeString(s[last:]))
	return b.String(), nil
}

// value returns the action printing a field path with a format.
func value(scope, path string, f proteusv1.FieldFormat) string {
	r := ref(scope, path)
	switch f {
	case proteusv1.FieldFormat_FIELD_FORMAT_NUMBER:
		return "{{fnumber " + r + " 2}}"
	case proteusv1.FieldFormat_FIELD_FORMAT_INTEGER:
		return "{{fnumber " + r + " 0}}"
	case proteusv1.FieldFormat_FIELD_FORMAT_MONEY:
		return "{{money " + r + "}}"
	case proteusv1.FieldFormat_FIELD_FORMAT_DATE:
		return "{{fdate " + r + "}}"
	case proteusv1.FieldFormat_FIELD_FORMAT_PERCENT:
		return "{{percent " + r + "}}"
	}
	return "{{" + r + "}}"
}

// formatted returns the action for a column or row that is either a path
// (formatted) or text with placeholders.
func (c *compiler) formatted(scope, s string, f proteusv1.FieldFormat, fail func(string) error) (string, error) {
	if validPath(s) {
		return value(scope, s, f), nil
	}
	if f != proteusv1.FieldFormat_FIELD_FORMAT_UNSPECIFIED && f != proteusv1.FieldFormat_FIELD_FORMAT_TEXT {
		return "", fail("a format needs a plain field path, not text with placeholders")
	}
	return c.text(s, fail)
}

func alignClass(a proteusv1.Align, f proteusv1.FieldFormat) string {
	switch a {
	case proteusv1.Align_ALIGN_RIGHT:
		return "num"
	case proteusv1.Align_ALIGN_CENTER:
		return "ctr"
	case proteusv1.Align_ALIGN_LEFT:
		return ""
	}
	switch f {
	case proteusv1.FieldFormat_FIELD_FORMAT_NUMBER, proteusv1.FieldFormat_FIELD_FORMAT_INTEGER,
		proteusv1.FieldFormat_FIELD_FORMAT_MONEY, proteusv1.FieldFormat_FIELD_FORMAT_PERCENT:
		return "num"
	}
	return ""
}

func checkFormat(f proteusv1.FieldFormat, fail func(string) error) error {
	if _, ok := proteusv1.FieldFormat_name[int32(f)]; !ok {
		return fail("unknown format")
	}
	return nil
}

func (c *compiler) kind(b *strings.Builder, blk *proteusv1.Block, where, scope string, depth int, fail func(string) error) error {
	switch k := blk.GetKind().(type) {
	case *proteusv1.Block_Heading:
		lvl := k.Heading.GetLevel()
		if lvl == 0 {
			lvl = 1
		}
		if lvl < 1 || lvl > 3 {
			return fail("heading level must be 1 to 3")
		}
		t, err := c.text(k.Heading.GetText(), fail)
		if err != nil {
			return err
		}
		fmt.Fprintf(b, `<h%d class="pt-h%d">%s</h%d>`, lvl, lvl, t, lvl)
	case *proteusv1.Block_Text:
		t, err := c.text(k.Text.GetText(), fail)
		if err != nil {
			return err
		}
		b.WriteString(`<div class="pt-text">`)
		for _, para := range strings.Split(strings.ReplaceAll(t, "\r\n", "\n"), "\n\n") {
			para = strings.TrimSpace(para)
			if para == "" {
				continue
			}
			b.WriteString("<p>" + strings.ReplaceAll(para, "\n", "<br>") + "</p>")
		}
		b.WriteString("</div>")
	case *proteusv1.Block_Field:
		if !validPath(k.Field.GetPath()) {
			return fail("field path must be dotted names such as client.name")
		}
		if err := checkFormat(k.Field.GetFormat(), fail); err != nil {
			return err
		}
		b.WriteString(`<div class="pt-field">`)
		if l := k.Field.GetLabel(); l != "" {
			t, err := c.text(l, fail)
			if err != nil {
				return err
			}
			b.WriteString(`<span class="pt-label">` + t + `</span>`)
		}
		pre, err := c.text(k.Field.GetPrefix(), fail)
		if err != nil {
			return err
		}
		suf, err := c.text(k.Field.GetSuffix(), fail)
		if err != nil {
			return err
		}
		b.WriteString(`<span class="pt-value">` + pre + value(scope, k.Field.GetPath(), k.Field.GetFormat()) + suf + `</span></div>`)
	case *proteusv1.Block_Image:
		var src string
		switch s := k.Image.GetSource().(type) {
		case *proteusv1.Image_Asset:
			if s.Asset == "" || strings.Contains(s.Asset, "..") || strings.ContainsAny(s.Asset, "\"<>{}") || len(s.Asset) > 200 {
				return fail("image asset must be a relative path such as assets/logo.png")
			}
			src = html.EscapeString(s.Asset)
		case *proteusv1.Image_Path:
			if !validPath(s.Path) {
				return fail("image path must be a field path")
			}
			src = "{{" + ref(scope, s.Path) + "}}"
		default:
			return fail("image needs an asset or a field path")
		}
		w, h := k.Image.GetWidthMm(), k.Image.GetHeightMm()
		if w < 0 || w > 300 || h < 0 || h > 300 {
			return fail("image width_mm and height_mm must be between 0 and 300")
		}
		var dims []string
		if w > 0 {
			dims = append(dims, "width: "+strconv.FormatFloat(w, 'f', -1, 64)+"mm")
		}
		if h > 0 {
			dims = append(dims, "height: "+strconv.FormatFloat(h, 'f', -1, 64)+"mm")
		}
		alt, err := c.text(k.Image.GetAlt(), fail)
		if err != nil {
			return err
		}
		fmt.Fprintf(b, `<img class="pt-img" src="%s" alt="%s"`, src, alt)
		if len(dims) > 0 {
			b.WriteString(` style="` + strings.Join(dims, "; ") + `"`)
		}
		b.WriteString(">")
	case *proteusv1.Block_Table:
		return c.table(b, k.Table, scope, fail)
	case *proteusv1.Block_KeyValue:
		if len(k.KeyValue.GetRows()) > maxRows {
			return fail(fmt.Sprintf("more than %d rows", maxRows))
		}
		b.WriteString(`<div class="pt-kv">`)
		for _, row := range k.KeyValue.GetRows() {
			if err := checkFormat(row.GetFormat(), fail); err != nil {
				return err
			}
			l, err := c.text(row.GetLabel(), fail)
			if err != nil {
				return err
			}
			v, err := c.formatted(scope, row.GetValue(), row.GetFormat(), fail)
			if err != nil {
				return err
			}
			b.WriteString(`<span class="k">` + l + `</span><span class="v">` + v + `</span>`)
		}
		b.WriteString("</div>")
	case *proteusv1.Block_Totals:
		return c.totals(b, k.Totals, scope, fail)
	case *proteusv1.Block_Columns:
		cols := k.Columns.GetColumns()
		if len(cols) < 1 || len(cols) > 6 {
			return fail("columns needs 1 to 6 columns")
		}
		widths := k.Columns.GetWidths()
		if len(widths) != 0 && len(widths) != len(cols) {
			return fail("widths must have one entry per column")
		}
		var total float64
		for _, w := range widths {
			if w <= 0 {
				return fail("widths must be positive")
			}
			total += w
		}
		b.WriteString(`<div class="pt-cols">`)
		for i, col := range cols {
			if len(widths) > 0 {
				fmt.Fprintf(b, `<div class="pt-col" style="flex: 0 0 %s%%">`, strconv.FormatFloat(widths[i]/total*100-1, 'f', 2, 64))
			} else {
				b.WriteString(`<div class="pt-col">`)
			}
			if err := c.region(b, col, fmt.Sprintf("%s.columns[%d]", where, i), scope, depth+1); err != nil {
				return err
			}
			b.WriteString("</div>")
		}
		b.WriteString("</div>")
	case *proteusv1.Block_Signature:
		lines := k.Signature.GetLines()
		if len(lines) == 0 || len(lines) > 4 {
			return fail("signature needs 1 to 4 lines")
		}
		b.WriteString(`<div class="pt-sig">`)
		for _, l := range lines {
			t, err := c.text(l.GetLabel(), fail)
			if err != nil {
				return err
			}
			b.WriteString(`<div class="line">` + t + `</div>`)
		}
		b.WriteString("</div>")
	case *proteusv1.Block_Qr:
		v := k.Qr.GetValue()
		var expr string
		switch {
		case validPath(v):
			expr = ref(scope, v)
		case strings.Contains(v, "{{"):
			return fail("qr value must be a field path or plain text")
		case utf8.RuneCountInString(v) > 2000:
			return fail("qr value is longer than 2000 characters")
		default:
			expr = strconv.Quote(v)
		}
		size := k.Qr.GetSizeMm()
		if size < 0 || size > 100 {
			return fail("qr size_mm must be between 0 and 100")
		}
		if size == 0 {
			size = 25
		}
		fmt.Fprintf(b, `<img class="pt-qr" src="{{qr %s}}" alt="QR code" style="width: %[2]smm; height: %[2]smm">`,
			expr, strconv.FormatFloat(size, 'f', -1, 64))
	case *proteusv1.Block_Divider:
		b.WriteString(`<hr class="pt-divider">`)
	case *proteusv1.Block_Section:
		s := k.Section
		if rep := s.GetRepeat(); rep != "" {
			if !validPath(rep) {
				return fail("section repeat must be a list field path")
			}
			b.WriteString("{{range $i, $it := " + ref(scope, rep) + "}}")
			if s.GetPagePerItem() {
				b.WriteString(`<div class="pt-item{{if $i}} pt-break{{end}}">`)
			} else {
				b.WriteString(`<div class="pt-item">`)
			}
			if err := c.region(b, s.GetContent(), where+".content", ".", depth+1); err != nil {
				return err
			}
			b.WriteString("</div>{{end}}")
		} else {
			b.WriteString(`<div class="pt-section">`)
			if err := c.region(b, s.GetContent(), where+".content", scope, depth+1); err != nil {
				return err
			}
			b.WriteString("</div>")
		}
	default:
		return fail("unknown block kind")
	}
	return nil
}

func (c *compiler) table(b *strings.Builder, t *proteusv1.Table, scope string, fail func(string) error) error {
	if !validPath(t.GetList()) {
		return fail("table list must be a field path such as items")
	}
	cols := t.GetColumns()
	if len(cols) == 0 || len(cols) > maxColumns {
		return fail(fmt.Sprintf("table needs 1 to %d columns", maxColumns))
	}
	var pct float64
	for _, col := range cols {
		if err := checkFormat(col.GetFormat(), fail); err != nil {
			return err
		}
		if w := col.GetWidthPct(); w < 0 || w > 100 {
			return fail("column width_pct must be between 0 and 100")
		} else {
			pct += w
		}
		if col.GetPath() == "" {
			return fail("every column needs a path")
		}
	}
	if pct > 100 {
		return fail("column widths add up to more than 100%")
	}
	class := "pt-table"
	if t.GetStriped() {
		class += " striped"
	}
	empty, err := c.text(t.GetEmptyText(), fail)
	if err != nil {
		return err
	}
	if empty == "" {
		empty = "No items"
	}
	b.WriteString("{{if " + ref(scope, t.GetList()) + "}}")
	b.WriteString(`<table class="` + class + `"><thead><tr>`)
	if t.GetNumbered() {
		b.WriteString(`<th class="ctr" style="width: 8%">#</th>`)
	}
	for _, col := range cols {
		h, err := c.text(col.GetHeader(), fail)
		if err != nil {
			return err
		}
		cls := alignClass(col.GetAlign(), col.GetFormat())
		attrs := ""
		if cls != "" {
			attrs += ` class="` + cls + `"`
		}
		if w := col.GetWidthPct(); w > 0 {
			attrs += ` style="width: ` + strconv.FormatFloat(w, 'f', -1, 64) + `%"`
		}
		b.WriteString("<th" + attrs + ">" + h + "</th>")
	}
	b.WriteString("</tr></thead><tbody>{{range $i, $it := " + ref(scope, t.GetList()) + "}}<tr>")
	if t.GetNumbered() {
		b.WriteString(`<td class="ctr">{{add $i 1}}</td>`)
	}
	for _, col := range cols {
		v, err := c.formatted(".", col.GetPath(), col.GetFormat(), fail)
		if err != nil {
			return err
		}
		cls := alignClass(col.GetAlign(), col.GetFormat())
		if cls != "" {
			b.WriteString(`<td class="` + cls + `">` + v + "</td>")
		} else {
			b.WriteString("<td>" + v + "</td>")
		}
	}
	b.WriteString("</tr>{{end}}</tbody></table>")
	b.WriteString(`{{else}}<p class="pt-empty">` + empty + "</p>{{end}}")
	return nil
}

func (c *compiler) totals(b *strings.Builder, t *proteusv1.Totals, scope string, fail func(string) error) error {
	rows := t.GetRows()
	if len(rows) == 0 || len(rows) > maxRows {
		return fail(fmt.Sprintf("totals needs 1 to %d rows", maxRows))
	}
	b.WriteString(`<div class="pt-totals">`)
	for _, row := range rows {
		if err := checkFormat(row.GetFormat(), fail); err != nil {
			return err
		}
		f := row.GetFormat()
		if f == proteusv1.FieldFormat_FIELD_FORMAT_UNSPECIFIED {
			f = proteusv1.FieldFormat_FIELD_FORMAT_MONEY
		}
		l, err := c.text(row.GetLabel(), fail)
		if err != nil {
			return err
		}
		var v string
		switch val := row.GetValue().(type) {
		case *proteusv1.TotalRow_Path:
			if !validPath(val.Path) {
				return fail("totals row path must be a field path")
			}
			v = value(scope, val.Path, f)
		case *proteusv1.TotalRow_Aggregate:
			a := val.Aggregate
			if !validPath(a.GetList()) {
				return fail("aggregate list must be a field path")
			}
			fn := "sum"
			switch a.GetFunc() {
			case proteusv1.AggregateFunc_AGGREGATE_FUNC_COUNT:
				fn = "count"
			case proteusv1.AggregateFunc_AGGREGATE_FUNC_AVERAGE:
				fn = "avg"
			case proteusv1.AggregateFunc_AGGREGATE_FUNC_UNSPECIFIED, proteusv1.AggregateFunc_AGGREGATE_FUNC_SUM:
			default:
				return fail("unknown aggregate func")
			}
			expr := "(" + fn + " " + ref(scope, a.GetList())
			if fn != "count" {
				if !identRe.MatchString(a.GetColumn()) {
					return fail("aggregate column must be a single name")
				}
				expr += " " + strconv.Quote(a.GetColumn())
			}
			expr += ")"
			if fn == "count" {
				v = "{{" + expr + "}}"
			} else {
				v = strings.Replace(value(scope, "x", f), ref(scope, "x"), expr, 1)
			}
		default:
			return fail("totals row needs a path or an aggregate")
		}
		cls := "row"
		if row.GetEmphasized() {
			cls += " em"
		}
		b.WriteString(`<div class="` + cls + `"><span class="k">` + l + `</span><span class="v">` + v + "</span></div>")
	}
	b.WriteString("</div>")
	return nil
}
