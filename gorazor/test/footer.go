// This file is generated by gorazor 2.0
// DON'T modified manually
// Should edit source file and re-generate: cases/footer.gohtml

package cases

import (
	"io"
	"strings"
)

func Footer() string {
	var _b strings.Builder
	RenderFooter(&_b)
	return _b.String()
}

func RenderFooter(_buffer io.StringWriter) {
	_buffer.WriteString("<div>copyright 2014</div>")

}
