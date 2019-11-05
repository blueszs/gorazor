// This file is generated by gorazor 1.2.1
// DON'T modified manually
// Should edit source file and re-generate: cases/base_layout.gohtml

package cases

import (
	"github.com/sipin/gorazor/gorazor"
	"io"
	"strings"
)

// Base_layout generates cases/base_layout.gohtml
func Base_layout(body string, title string) string {
	var _b strings.Builder

	_body := func(_buffer io.StringWriter) {
		_buffer.WriteString(body)
	}

	_title := func(_buffer io.StringWriter) {
		_buffer.WriteString(title)
	}

	RenderBase_layout(&_b, _body, _title)
	return _b.String()
}

// RenderBase_layout render cases/base_layout.gohtml
func RenderBase_layout(_buffer io.StringWriter, body func(_buffer io.StringWriter), title func(_buffer io.StringWriter)) {
	_buffer.WriteString("\n<html>\n    <head>\n        <title>")
	_buffer.WriteString(gorazor.HTMLEscape(title))
	_buffer.WriteString("</title>\n    </head>\n    <body>")
	_buffer.WriteString(gorazor.HTMLEscape(body))
	_buffer.WriteString("</body>\n</html>")

}
