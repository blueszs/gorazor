package razorcore

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// GorazorNamespace defines util pkg namespace used in template
var GorazorNamespace = `"github.com/sipin/gorazor/gorazor"`

// TemplateNamespacePrefix record the namespace prefix for executing folder
var TemplateNamespacePrefix = ""

// QuickMode enabled to skip template optimization
var QuickMode = false

//------------------------------ Compiler ------------------------------ //
const (
	CMKP = iota
	CBLK
	CSTAT
)

var execDir string

func init() {
	// make sure running in source directory
	_, filename, _, _ := runtime.Caller(0)
	execDir = path.Dir(filename) + "/"
}

func getValStr(e interface{}) string {
	switch v := e.(type) {
	case *Ast:
		return v.TagName
	case Token:
		if !(v.Type == tkAt || v.Type == tkAtColon) {
			return v.Text
		}
		return ""
	default:
		panic(e)
	}
}

// Part represent gorazor template parts
type Part struct {
	ptype int
	value string
	line  int
}

// Compiler generate go code for gorazor template
type Compiler struct {
	inputPath  string
	tplPath    string
	ast        *Ast
	buf        string //the final result
	isLayout   bool
	layout     string
	firstBLK   int
	params     []string
	paramNames []string
	parts      []Part
	imports    map[string]bool
	options    Option
	dir        string
	file       string
}

func (cp *Compiler) addPart(part Part) {
	if len(cp.parts) == 0 {
		cp.parts = append(cp.parts, part)
		return
	}
	last := &cp.parts[len(cp.parts)-1]
	if last.ptype == part.ptype {
		last.value += part.value
	} else {
		cp.parts = append(cp.parts, part)
	}
}

func (cp *Compiler) isLayoutSectionPart(p Part) (is bool, val string) {
	if !cp.isLayout {
		return
	}

	if !strings.HasPrefix(p.value, "_buffer.WriteString((") {
		return
	}

	if !strings.HasSuffix(p.value, "))\n") {
		return
	}

	val = p.value[21 : len(p.value)-3]
	for _, name := range cp.paramNames {
		if val == name {
			return true, val
		}
	}

	return
}

func (cp *Compiler) isLayoutSectionTest(p Part) (is bool, val string) {
	if !cp.isLayout {
		return
	}

	line := strings.TrimSpace(p.value)
	line = strings.Replace(line, " ", "", -1)

	for _, p := range cp.paramNames {
		if line == "if"+p+`==""{` {
			return true, "if " + p + " == nil {\n"
		}
		if line == "if"+p+`!=""{\n` {
			return true, "if " + p + " != nil {\n"
		}
	}

	return
}

func (cp *Compiler) getLineHint(line int) string {
	if cp.options.NoLineNumber {
		return ""
	}
	return "// Line: " + strconv.Itoa(line) + "\n"
}

func (cp *Compiler) genPart() {
	res := ""

	for _, p := range cp.parts {
		if p.ptype == CMKP && p.value != "" {
			// do some escapings
			for strings.HasSuffix(p.value, "\n") {
				p.value = p.value[:len(p.value)-1]
			}
			if p.value != "" {
				p.value = fmt.Sprintf("%#v", p.value)
				if p.line > 0 {
					res += cp.getLineHint(p.line)
				}

				res += "_buffer.WriteString(" + p.value + ")\n"
			}
		} else if p.ptype == CBLK {
			if ok, val := cp.isLayoutSectionTest(p); ok {
				res += val
			} else {
				res += p.value + "\n"
			}
		} else if ok, val := cp.isLayoutSectionPart(p); ok {
			res += cp.getLineHint(p.line)
			res += val + "(_buffer)\n"
		} else {
			res += p.value
		}
	}
	cp.buf = res
}

func makeCompiler(ast *Ast, options Option, input string) *Compiler {
	dir := filepath.Base(filepath.Dir(input))
	file := strings.Replace(filepath.Base(input), gzExtension, "", 1)
	if !options.NameNotChange {
		file = Capitalize(file)
	}
	cp := &Compiler{
		ast:    ast,
		buf:    "",
		layout: "", firstBLK: 0,
		params: []string{}, parts: []Part{},
		imports: map[string]bool{},
		options: options,
		dir:     dir,
		file:    file,
	}

	if dir == "layout" {
		cp.isLayout = true
	}

	cp.inputPath = strings.Replace(input, "\\", "/", -1)
	cp.tplPath = strings.Replace(cp.inputPath, execDir, "", -1)
	return cp
}

func (cp *Compiler) visitBLK(child Token) {
	cp.addPart(Part{CBLK, getValStr(child), child.Line})
}

func (cp *Compiler) visitMKP(child Token) {
	cp.addPart(Part{CMKP, getValStr(child), child.Line})
}

func (cp *Compiler) settleLayout(layoutFunc string) {
	path := cp.layout + "/" + layoutFunc + ".gohtml"

	if !exists(path) && TemplateNamespacePrefix != "" {
		path = path[len(TemplateNamespacePrefix)+1:]
	}

	if !exists(path) {
		layoutFunc = strings.ToLower(layoutFunc[0:1]) + layoutFunc[1:]
		path = cp.layout + "/" + layoutFunc + ".gohtml"

		if !exists(path) && TemplateNamespacePrefix != "" {
			path = path[len(TemplateNamespacePrefix)+1:]
		}
	}

	cp.layout = cp.layout + "/" + layoutFunc
	if !exists(path) {
		panic("Can't find layout: " + cp.layout + " [" + cp.file + "]")
	}

	if len(LayoutArgs(path)) == 0 {
		//TODO, bad for performance
		_cp, err := run(path, cp.options)
		if err != nil {
			panic(err)
		}
		SetLayout(cp.layout, _cp.params)
	}
}

// First block contains imports and parameters, specific action for layout,
// NOTE, layout have some conventions.
func (cp *Compiler) visitFirstBLK(blk *Ast) {
	pre := cp.buf
	cp.buf = ""
	first := ""
	backup := cp.parts
	cp.parts = []Part{}
	cp.visitAst(blk)
	cp.genPart()
	first, cp.buf = cp.buf, pre
	cp.parts = backup

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", "package main\n"+first, parser.ImportsOnly)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	} else {
		for _, s := range f.Imports {
			v := s.Path.Value
			if s.Name != nil {
				v = s.Name.Name + " " + v
			}
			parts := strings.SplitN(v, "/", -1)

			if len(parts) >= 1 && parts[len(parts)-1] == `layout"` {
				cp.layout = strings.Replace(v, "\"", "", -1)
			}

			cp.imports[v] = true
		}
	}

	lines := strings.SplitN(first, "\n", -1)
	var layoutFunc string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "var") {
			vname := l[4:]
			if strings.HasSuffix(l, "gorazor.Widget") {
				cp.imports[GorazorNamespace] = true
				cp.params = append(cp.params, vname[:len(vname)-14]+"gorazor.Widget")
				name := strings.SplitN(vname, " ", 2)[0]
				cp.paramNames = append(cp.paramNames, name)
			} else if strings.HasPrefix(vname, "layout") {
				funcName := strings.SplitN(vname, ".", -1)
				layoutFunc = funcName[len(funcName)-1]
			} else {
				cp.params = append(cp.params, vname)
				name := strings.SplitN(vname, " ", 2)[0]
				cp.paramNames = append(cp.paramNames, name)
			}
		} else if strings.HasPrefix(l, "isLayout") {
			cp.isLayout = strings.HasSuffix(l, "true")
		} else if strings.HasPrefix(l, "layout:=") || strings.HasPrefix(l, "layout :=") {
			vname := strings.TrimSpace(strings.Split(l, ":=")[1])
			funcName := strings.SplitN(vname, ".", -1)
			layoutFunc = funcName[len(funcName)-1]
		}
	}
	if cp.layout != "" {
		cp.settleLayout(layoutFunc)
	}
}

func (cp *Compiler) isExpNeedEscape(val string) (needEsape bool) {
	switch {
	case val == "helper" || val == "html" || val == "raw":
		return false
	case cp.dir == "layout":
		for _, param := range cp.params {
			if strings.HasPrefix(param, val+" ") {
				return false
			}
		}
	}
	return true
}

func (cp *Compiler) visitExp(child interface{}, parent *Ast, idx int, isHomo bool) {
	start := ""
	end := ""
	ppNotExp := true
	ppChildCnt := len(parent.Children)
	if parent.Parent != nil && parent.Parent.Mode == EXP {
		ppNotExp = false
	}
	val := getValStr(child)

	if ppNotExp && idx == 0 && isHomo {
		if cp.isExpNeedEscape(val) {
			start += "gorazor.HTMLEscape("
			cp.imports[GorazorNamespace] = true
		} else {
			start += "("
		}
	}
	if ppNotExp && idx == ppChildCnt-1 && isHomo {
		end += ")"
	}

	lineHint := ""
	lineNumber := 0
	if ppNotExp && idx == 0 {
		if token, ok := child.(Token); ok {
			lineNumber = token.Line
			lineHint = cp.getLineHint(token.Line)
		}
		start = "_buffer.WriteString(" + start
	}
	if ppNotExp && idx == ppChildCnt-1 {
		end += ")\n"
	}

	if val == "raw" {
		cp.addPart(Part{CSTAT, lineHint + start + end, lineNumber})
	} else {
		p := Part{CSTAT, start + val + end, lineNumber}
		if ok, _ := cp.isLayoutSectionPart(p); !ok {
			p.value = lineHint + p.value
		}
		cp.addPart(p)
	}
}

func (cp *Compiler) visitAstBlk(ast *Ast) {
	if cp.firstBLK == 0 {
		cp.firstBLK = 1
		cp.visitFirstBLK(ast)
	} else {
		remove := false
		if len(ast.Children) >= 2 {
			first := ast.Children[0]
			last := ast.Children[len(ast.Children)-1]
			v1, ok1 := first.(Token)
			v2, ok2 := last.(Token)
			if ok1 && ok2 && v1.Text == "{" && v2.Text == "}" {
				remove = true
			}
		}
		for idx, c := range ast.Children {
			if remove && (idx == 0 || idx == len(ast.Children)-1) {
				continue
			}
			if token, ok := c.(Token); ok {
				cp.visitBLK(token)
			} else {
				cp.visitAst(c.(*Ast))
			}
		}
	}
}

func (cp *Compiler) visitAst(ast *Ast) {
	switch ast.Mode {
	case MKP:
		cp.firstBLK = 1
		for _, c := range ast.Children {
			if token, ok := c.(Token); ok {
				cp.visitMKP(token)
			} else {
				cp.visitAst(c.(*Ast))
			}
		}
	case BLK:
		cp.visitAstBlk(ast)
	case EXP:
		cp.firstBLK = 1
		nonExp := ast.hasNonExp()
		for i, c := range ast.Children {
			if _, ok := c.(Token); ok {
				cp.visitExp(c, ast, i, !nonExp)
			} else {
				cp.visitAst(c.(*Ast))
			}
		}
	case PRG:
		for _, c := range ast.Children {
			cp.visitAst(c.(*Ast))
		}
	}
}

func (cp *Compiler) hasLayout() bool {
	return cp.layout != ""
}

func (cp *Compiler) generateFoot(sections []string) string {
	foot := ""
	if cp.hasLayout() {
		foot += "\n"
		parts := strings.SplitN(cp.layout, "/", -1)
		base := Capitalize(parts[len(parts)-1])
		foot += "layout.Render" + base + "("
		foot += "_buffer, _body"
	} else if len(sections) > 0 {
		fmt.Println("expect layout for sections: " + cp.file)
		os.Exit(1)
	}

	args := LayoutArgs(cp.layout)
	if len(args) == 0 {
		for _, sec := range sections {
			foot += ", " + sec + "()"
		}
	} else {
		for _, arg := range args[1:] {
			arg = strings.Replace(arg, "string", "", -1)
			arg = strings.TrimSpace(arg)
			found := false
			for _, sec := range sections {
				if sec == arg {
					found = true
					foot += ", _" + sec
					break
				}
			}
			if !found {
				foot += ", " + `nil`
			}
		}
	}
	if cp.layout != "" {
		foot += ")"
	}

	return foot
}

func (cp *Compiler) processLayout() {
	lines := strings.SplitN(cp.buf, "\n", -1)
	out := ""
	sections := []string{}
	scope := 0
	hasBodyClosed := false

	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "section") && strings.HasSuffix(l, "{") {
			if hasBodyClosed == false {
				hasBodyClosed = true
				out += "\n}\n"
			}

			name := l
			name = strings.TrimSpace(name[7 : len(name)-1])
			out += "\n _" + name + " := func(_buffer io.StringWriter) {\n"
			scope = 1
			sections = append(sections, name)
		} else if scope > 0 {
			if strings.HasSuffix(l, "{") {
				scope++
			} else if strings.HasSuffix(l, "}") {
				scope--
			}
			if scope == 0 {
				out += "\n}\n"
				scope = 0
			} else {
				out += l + "\n"
			}
		} else {
			out += l + "\n"
		}
	}

	if cp.hasLayout() && hasBodyClosed == false {
		out += "\n}\n"
	}

	cp.buf = out

	foot := cp.generateFoot(sections)

	cp.buf += foot
}

func (cp *Compiler) getLayoutOverload() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf(`
	// %s generates %s
	func %s(%s) string {
		var _b strings.Builder

	`, cp.file, cp.tplPath, cp.file, strings.Join(cp.params, ", ")))

	var funcNames []string
	for _, name := range cp.paramNames {
		b.WriteString(fmt.Sprintf(`
		_%s := func(_buffer io.StringWriter) {
			_buffer.WriteString(%s)
		}
		`, name, name))
		funcNames = append(funcNames, "_"+name)
	}

	b.WriteString(fmt.Sprintf(`
		Render%s(&_b, %s)
		return _b.String()
	}

	`, cp.file, strings.Join(funcNames, ", ")))
	return b.String()
}

func (cp *Compiler) visit() {
	cp.visitAst(cp.ast)
	cp.genPart()

	pack := cp.dir
	fun := cp.file

	cp.imports[`"io"`] = true
	cp.imports[`"strings"`] = true

	head := fmt.Sprintf(`// This file is generated by gorazor %s
// DON'T modified manually
// Should edit source file and re-generate: %s

`, VERSION, cp.tplPath)

	head += "package " + pack + "\n import (\n"
	for k := range cp.imports {
		head += k + "\n"
	}

	funcArgs := strings.Join(cp.params, ", ")

	head += "\n)"

	if cp.isLayout {
		head += cp.getLayoutOverload()
		head += fmt.Sprintf(`
	// Render%s render %s
	`, fun, cp.tplPath)

		head += "func Render" + fun + "(_buffer io.StringWriter, " +
			strings.Replace(funcArgs, " string", " func(_buffer io.StringWriter)", -1) + ") {\n"
	} else {
		head += fmt.Sprintf(`
	// %s generates %s
	func %s(%s) string {
		var _b strings.Builder
		Render%s(&_b, %s)
		return _b.String()
	}

	`, fun, cp.tplPath, fun, funcArgs, fun, strings.Join(cp.paramNames, ", "))

		head += fmt.Sprintf(`
	// Render%s render %s
	`, fun, cp.tplPath)

		head += "func Render" + fun + "(_buffer io.StringWriter, " + funcArgs + ") {\n"
	}

	if cp.hasLayout() {
		head += "\n_body := func(_buffer io.StringWriter) {\n"
	}

	cp.buf = head + cp.buf
	cp.processLayout()
	foot := "\n}\n"
	cp.buf += foot
}

func run(path string, Options Option) (*Compiler, error) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := string(content)
	lex := &Lexer{text, Tests}

	res, err := lex.Scan()
	if err != nil {
		return nil, err
	}

	//DEBUG
	if Options.IsDebug {
		fmt.Println("------------------- TOKEN START -----------------")
		for _, elem := range res {
			elem.P()
		}
		fmt.Println("--------------------- TOKEN END -----------------")
	}

	parser := &Parser{&Ast{}, nil, res, []Token{}, false, UNK}
	err = parser.Run()
	if err != nil {
		fmt.Println(path, ":", err)
		os.Exit(2)
	}

	//DEBUG
	if Options.IsDebug {
		fmt.Println("--------------------- AST START -----------------")
		parser.ast.debug(0, 20)
		fmt.Println("--------------------- AST END -----------------")
		if parser.ast.Mode != PRG {
			panic("TYPE")
		}
	}
	cp := makeCompiler(parser.ast, Options, path)
	cp.visit()
	return cp, nil
}

func generate(path string, output string, Options Option) error {
	cp, err := run(path, Options)
	if err != nil || cp == nil {
		panic(err)
	}

	code := FormatBuffer(cp.buf)
	if !QuickMode {
		_, code = optimize(output, cp.dir, code)
	}

	return ioutil.WriteFile(output, []byte(code), 0644)
}
