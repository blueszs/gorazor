// This file is generated by gorazor 2.0.1
// DON'T modified manually
// Should edit source file and re-generate: cases/textarea.gohtml

package cases

import (
	"github.com/sipin/gorazor/gorazor/"
	"io"
	"strings"
)

func Textarea(count int) string {
	var _b strings.Builder
	RenderTextarea(&_b, count)
	return _b.String()
}

func RenderTextarea(_buffer io.StringWriter, count int) {
	_buffer.WriteString("\n<html>\n<body>\n<textarea rows=\"4\" cols=\"50\">\n        At w3schools.com ")
	_buffer.WriteString(gorazor.HTMLEscape(count))
	_buffer.WriteString(" you will learn how to make a website.\n  We offer free tutorials in all web development technologies.\n\n\n\n  At w3schools.com ")
	_buffer.WriteString(gorazor.HTMLEscape(count))
	_buffer.WriteString(" you will learn\n  how to make a website.\n  We offer free tutorials in all web development technologies.\n\n</body>\n</html>")

}
