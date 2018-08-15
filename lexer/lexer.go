package lexer

import (
	"fmt"
	"unicode/utf8"
	"unsafe"

	"github.com/bucketd/go-graphqlparser/token"
)

const (
	// er represents an "empty" rune, but is also an invalid one.
	er = rune(-1)
	// eof represents the end of input.
	eof = rune(0)

	bom = rune(0xFEFF) // Unicode BOM.
	ws  = rune(0x0020) // Literal ' '.
	com = rune(0x002C) // Literal ','.
	dq  = rune(0x0022) // '\"' double quote.
	fsl = rune(0x002F) // '\/' solidus (forward slash).
	bck = rune(0x0008) // '\b' backspace.
	ff  = rune(0x000C) // '\f' form feed.
	lf  = rune(0x000A) // '\n' line feed (new line).
	cr  = rune(0x000D) // '\r' carriage return.
	tab = rune(0x0009) // '\t' horizontal tab.
	bsl = rune(0x005C) // Literal reverse solidus (backslash).

	// nothing to see here
	rune1Max = 1<<7 - 1
	rune2Max = 1<<11 - 1
	rune3Max = 1<<16 - 1

	// nothing to see here
	maskx = 0x3F // 0011 1111

	// nothing to see here
	runeError    = '\uFFFD'
	maxRune      = '\U0010FFFF'
	surrogateMin = 0xD800
	surrogateMax = 0xDFFF

	// these are not the bytes you're looking for
	t1 = 0x00 // 0000 0000
	tx = 0x80 // 1000 0000
	t2 = 0xC0 // 1100 0000
	t3 = 0xE0 // 1110 0000
	t4 = 0xF0 // 1111 0000
	t5 = 0xF8 // 1111 1000
)

// Token represents a small, easily categorisable data structure that is fed to the parser to
// produce the abstract syntax tree (AST).
type Token struct {
	Type     token.Type // The token type.
	Literal  string     // The literal value consumed.
	Position int        // The starting position, in runes, of this token in the input.
	Line     int        // The line number at the start of this item.
}

// Lexer holds the state of a state machine for lexically analysing GraphQL queries.
type Lexer struct {
	input    []byte // Raw input is just a byte slice. It is expected to be UTF-8 encoded characters.
	inputLen int    // Length of the input, in bytes.

	// Positional information.
	pos  int // The start position of the last rune read, in bytes.
	lpos int // The start position of the last rune read, in runes, on the current line.
	line int // The current line number.
}

// New returns a new lexer, for lexically analysing GraphQL queries from a given reader.
func New(input []byte) *Lexer {
	return &Lexer{
		input:    input,
		inputLen: len(input),
		line:     1,
	}
}

// Scan attempts to read the next significant token from the input. Tokens that are not understood
// will yield an "illegal" token.
func (l *Lexer) Scan() Token {
	r, w := l.readNextSignificant()

	switch {
	case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_':
		return l.scanName(r)

	case r == '{' || r == '}' || r == '[' || r == ']' || r == '!' || r == '$' || r == '(' || r == ')' || r == '.' || r == ':' || r == '=' || r == '@' || r == '|':
		return l.scanPunctuator(r, w)

	case (r >= '0' && r <= '9') || r == '-':
		return l.scanNumber(r)

	case r == '#':
		return l.scanComment(r)

	case r == '"':
		r1, w1 := l.read()
		r2, w2 := l.read()
		if r1 == '"' && r2 == '"' {
			return l.scanBlockString(r)
		}
		l.unread(w2)
		l.unread(w1)

		return l.scanString(r)

	case r == eof:
		return Token{
			Type:     token.EOF,
			Position: l.lpos,
			Line:     l.line,
		}

	default:
		return Token{
			Type:     token.Illegal,
			Literal:  string(r),
			Position: l.lpos,
			Line:     l.line,
		}
	}
}

// scanString ...
func (l *Lexer) scanString(r rune) Token {
	var w int

	startPos := l.pos
	startLPos := l.lpos
	startLine := l.line

	var bc int
	var hasEscape bool

	var done bool
	for !done {
		r, w = l.read()
		bc += w

		switch {
		case r == '"':
			done = true

		case r < ws && r != tab:
			return Token{
				Type:     token.Illegal,
				Literal:  fmt.Sprintf("invalid character within string: %q", r),
				Position: startLPos,
				Line:     startLine,
			}

		case r == bsl:
			hasEscape = true

			r, w = l.read()

			// No need to increment bc here, if we hit backslash, we should already have incremented
			// the counter by 1. That one byte increment should satisfy the width of any escape
			// sequence other than unicode escape sequences when decoded as a rune. We handle the
			// unicode escape sequence case further down.
			//bc += w

			if r == 'u' {
				_, _ = l.read()
				_, _ = l.read()
				_, _ = l.read()
				_, _ = l.read()

				// Increment bc by 3, because we've already incremented by 1 above at the start of
				// this loop iteration. We increment by 3 here because we want to have incremented
				// by 4 in total. 4 bytes being the maximum width of a valid unicode escape sequence
				// supported by GraphQL.
				bc += 3
			}
		}
	}

	if !hasEscape {
		return Token{
			Type:     token.StringValue,
			Literal:  btos(l.input[startPos : l.pos-1]),
			Position: startLPos,
			Line:     startLine,
		}
	}

	l.pos = startPos
	l.lpos = startLPos
	l.line = startLine

	// Sadly, allocations cannot be avoided here unless we modify the input byte slice to make
	// string scanning work. This is because we have to replace the escape sequences with their
	// actual rune counterparts and use that as the token's literal value. To store that data, we
	// need bytes to be allocated.
	bs := make([]byte, 0, bc)
	for {
		r, _ = l.read()

		switch {
		case r == '"' || r == eof:
			return Token{
				Type:     token.StringValue,
				Literal:  btos(bs),
				Position: startLPos,
				Line:     startLine,
			}

		case r == bsl:
			r, err := escapedChar(l)
			if err != nil {
				return Token{
					Type:     token.Illegal,
					Literal:  err.Error(),
					Position: startLPos,
					Line:     startLine,
				}
			}

			encodeRune(r, func(b byte) {
				bs = append(bs, b)
			})

		default:
			encodeRune(r, func(b byte) {
				bs = append(bs, b)
			})
		}
	}
}

// scanBlockString ...
func (l *Lexer) scanBlockString(r rune) Token {
	var w int

	startPos := l.pos
	startLPos := l.lpos
	startLine := l.line

	var bc int
	var hasEscape bool

	var done bool
	for !done {
		r, w = l.read()
		bc += w

		switch {
		case r == '"':
			if isTripQuotes(l) {
				done = true
			}

		case r < ws && r != tab && r != lf && r != cr:
			return Token{
				Type:     token.Illegal,
				Literal:  fmt.Sprintf("invalid character within string: %q", r),
				Position: startLPos,
				Line:     startLine,
			}

		case r == bsl:
			r, _ = l.read()
			if r == '"' && isTripQuotes(l) {
				hasEscape = true
				bc += 2
			}
			bc++
		}
	}

	endPos := l.pos - 3

	// If the first character in the string is a newline, ignore it.
	// TODO(seeruk): What about CRLF?
	if rune(l.input[startPos]) == cr {
		startPos++
	}
	if rune(l.input[startPos]) == lf {
		startPos++
	}

	// If the last character in the string is a newline, ignore it.
	// TODO(seeruk): What about CRLF?
	if rune(l.input[endPos-1]) == lf {
		endPos--
	}
	if rune(l.input[endPos-1]) == cr {
		endPos--
	}

	if !hasEscape {
		return Token{
			Type:     token.StringValue,
			Literal:  btos(l.input[startPos:endPos]),
			Position: startLPos,
			Line:     startLine,
		}
	}

	l.pos = startPos
	l.lpos = startLPos
	l.line = startLine

	bs := make([]byte, 0, bc)
	for {
		r, _ = l.read()

		switch {
		case r == '"':
			if isTripQuotes(l) {
				bsend := len(bs) - 1
				// If the last character in the string is a newline, ignore it.
				if rune(bs[bsend]) == lf {
					bsend--
				}
				if rune(bs[bsend]) == cr {
					bsend--
				}

				return Token{
					Type:     token.StringValue,
					Literal:  btos(bs[:bsend+1]),
					Position: startLPos,
					Line:     startLine,
				}
			}
			encodeRune(r, func(b byte) {
				bs = append(bs, b)
			})

		case r == bsl:
			r, _ := l.read()
			if r == '"' && isTripQuotes(l) {
				encodeRune(r, func(b byte) {
					bs = append(bs, b)
					bs = append(bs, b)
					bs = append(bs, b)
				})
				continue
			}
			encodeRune(r, func(b byte) {
				bs = append(bs, byte(0x005C))
				bs = append(bs, b)
			})

		default:
			encodeRune(r, func(b byte) {
				bs = append(bs, b)
			})
		}
	}
}

func isTripQuotes(l *Lexer) bool {
	r1, w1 := l.read()
	r2, w2 := l.read()
	if r1 == '"' && r2 == '"' {
		return true
	}

	if r1 != eof {
		l.unread(w2)
	}
	if r2 != eof {
		l.unread(w1)
	}
	return false
}

func escapedChar(l *Lexer) (rune, error) {
	r, _ := l.read()
	switch r {
	case '"':
		return dq, nil
	case '/':
		return fsl, nil
	case '\\': // escaped single backslash '\' == U+005C
		return bsl, nil
	case 'b':
		return bck, nil
	case 'f':
		return ff, nil
	case 'n':
		return lf, nil
	case 'r':
		return cr, nil
	case 't':
		return tab, nil

	case 'u':
		r1, _ := l.read()
		r2, _ := l.read()
		r3, _ := l.read()
		r4, _ := l.read()

		r := unicodeCodePointToRune(r1, r2, r3, r4)
		if r < 0 {
			return 0, fmt.Errorf("invalid character escape sequence: %s", "\\u"+string([]rune{r1, r2, r3, r4}))
		}
		return r, nil
	}

	return 0, fmt.Errorf("invalid character escape sequence: %s", "\\"+string(r))
}

// encodeRune is a copy of the utf8.EncodeRune function, but instead of passing in a byte slice as
// the first argument, a callback is given. This callback may be called multiple times. This allows
// individual bytes to be passed back to the caller, one at a time. This enables the caller to do
// things like encode a rune into an existing byte slice.
func encodeRune(r rune, cb func(a byte)) {
	// Negative values are erroneous. Making it unsigned addresses the problem.
	switch i := uint32(r); {
	case i <= rune1Max:
		cb(byte(r))
		return
	case i <= rune2Max:
		cb(t2 | byte(r>>6))
		cb(tx | byte(r)&maskx)
		return
	case i > maxRune, surrogateMin <= i && i <= surrogateMax:
		r = runeError
		fallthrough
	case i <= rune3Max:
		cb(t3 | byte(r>>12))
		cb(tx | byte(r>>6)&maskx)
		cb(tx | byte(r)&maskx)
		return
	default:
		cb(t4 | byte(r>>18))
		cb(tx | byte(r>>12)&maskx)
		cb(tx | byte(r>>6)&maskx)
		cb(tx | byte(r)&maskx)
		return
	}
}

// TODO(seeruk): Here: https://github.com/graphql/graphql-js/blob/master/src/language/lexer.js#L689
func unicodeCodePointToRune(ar, br, cr, dr rune) rune {
	ai, bi, ci, di := hexRuneToInt(ar), hexRuneToInt(br), hexRuneToInt(cr), hexRuneToInt(dr)
	return rune(ai<<12 | bi<<8 | ci<<4 | di<<0)
}

// hexRuneToInt changes a character into its integer value in hexadecimal. For example:
// the character 'A' is 65 in decimal representation but its value is 10 in hexadecimal.
func hexRuneToInt(r rune) int {
	switch {
	case r >= '0' && r <= '9':
		return int(r - 48)
	case r >= 'A' && r <= 'F':
		return int(r - 55)
	case r >= 'a' && r <= 'f':
		return int(r - 87)
	}
	return -1
}

// scanComment scans valid GraphQL comments.
func (l *Lexer) scanComment(r rune) Token {
	var was000D bool
	var w int

	for {
		r, w = l.read()
		if r == eof {
			return l.Scan()
		}

		// If on the last iteration we saw a CR, then we should check if we just read an LF on this
		// iteration. If we did, reset line position as the next character is still the start of the
		// next line, then scan.
		if was000D && r == lf {
			l.lpos = 0

			return l.Scan()
		}

		// Otherwise, if we saw a CR, and this rune isn't an LF, then we have started reading the
		// next line's runes, so unread the rune we read, and scan the next token.
		if was000D && r != lf {
			l.unread(w)

			return l.Scan()
		}

		// If we encounter a CR at any point, this will be true.
		was000D = r == cr
		if was000D {
			// Carriage return, i.e. '\r'.
			l.line++
			l.lpos = 0
			continue
		}

		// If we encounter a LF without a proceeding CR, this will be true.
		if r == lf {
			// Line feed, i.e. '\n'.
			l.line++
			l.lpos = 0

			return l.Scan()
		}
	}
}

// scanName scans valid GraphQL name tokens.
func (l *Lexer) scanName(r rune) Token {
	byteStart := l.pos - 1
	runeStart := l.lpos

Loop:
	for {
		r, w := l.read()

		switch {
		case (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || r == '_':
			continue
		case r == eof:
			break Loop
		default:
			l.unread(w)
			break Loop
		}
	}

	return Token{
		Type:     token.Name,
		Literal:  btos(l.input[byteStart:l.pos]),
		Position: runeStart,
		Line:     l.line,
	}
}

// scanPunctuator scans valid GraphQL punctuation tokens.
func (l *Lexer) scanPunctuator(r rune, w int) Token {
	byteStart := l.pos
	runeStart := l.lpos

	if r == '.' {
		r2, _ := l.read()
		r3, _ := l.read()

		rs := []rune{r, r2, r3}
		if rs[1] != '.' || rs[2] != '.' {
			return Token{
				Type:     token.Illegal,
				Literal:  fmt.Sprintf("invalid punctuator, expected \"...\" but got: %q", string(rs)),
				Position: runeStart,
				Line:     l.line,
			}
		}

		return Token{
			Type:     token.Punctuator,
			Literal:  "...",
			Position: runeStart,
			Line:     l.line,
		}
	}

	// TODO(seeruk): Using other token types for each type of punctuation may actually be faster.
	return Token{
		Type:     token.Punctuator,
		Literal:  btos(l.input[byteStart-w : byteStart]),
		Position: runeStart,
		Line:     l.line,
	}
}

// scanNumber scans valid GraphQL integer and float value tokens.
func (l *Lexer) scanNumber(r rune) Token {
	byteStart := l.pos - 1
	runeStart := l.lpos

	var float bool // If true, number is float.
	var err error  // So no shadowing of r.

	// Check for preceding minus sign
	if r == '-' {
		r, _ = l.read()
	}

	// Check if digits begins with zero
	if r == '0' {
		r, _ = l.read()

		// If there is another digit after zero, error.
		if r >= '0' && r <= '9' {
			return Token{
				Type:     token.Illegal,
				Literal:  fmt.Sprintf("invalid number, unexpected digit after 0: %q", r),
				Position: runeStart,
				Line:     l.line,
			}
		}

		// If number does not begin with zero, read the digits.
		// If the first character is not a digit, error.
	} else {
		r, err = l.readDigits(r)
		if err != nil {
			return Token{
				Type:     token.Illegal,
				Literal:  err.Error(),
				Position: runeStart,
				Line:     l.line,
			}
		}
	}

	// Check for a decimal place, if there is a decimal place this number is a float.
	if r == '.' {
		float = true

		r, _ = l.read()

		// Read the digits after the decimal place if the first character is not a digit, error.
		r, err = l.readDigits(r)
		if err != nil {
			return Token{
				Type:     token.Illegal,
				Literal:  err.Error(),
				Position: runeStart,
				Line:     l.line,
			}
		}
	}

	// Check for exponent sign, if there is an exponent sign this number is a float.
	if r == 'e' || r == 'E' {
		float = true

		r, _ = l.read()

		// Check for positive or negative symbol infront of the value.
		if r == '+' || r == '-' {
			r, _ = l.read()
		}

		// Read the exponent digitas, if the first character is not a digit, error.
		r, err = l.readDigits(r)
		if err != nil {
			return Token{
				Type:     token.Illegal,
				Literal:  err.Error(),
				Position: runeStart,
				Line:     l.line,
			}
		}
	}

	// TODO(seeruk): This may not be correct.
	if r != eof {
		l.unread(1)
	}

	t := Token{
		Literal:  btos(l.input[byteStart:l.pos]),
		Line:     l.line,
		Position: runeStart,
	}

	t.Type = token.IntValue
	if float {
		t.Type = token.FloatValue
	}

	return t
}

// readDigits reads up until the next non-numeric character in the input.
func (l *Lexer) readDigits(r rune) (rune, error) {
	if !(r >= '0' && r <= '9') {
		return eof, fmt.Errorf("invalid number, expected digit but got: %q", r)
	}

	var done bool
	for !done {
		r, _ = l.read()

		switch {
		case r >= '0' && r <= '9':
			continue
		default:
			// No need to unread here. We actually want to read the character after the numbers.
			done = true
		}
	}

	return r, nil
}

// readNextSignificant reads runes until a "significant" rune is read, i.e. a rune that could be a
// significant token (not whitespace, not tabs, not newlines, not commas, not encoding-specific
// characters, etc.). It also does part of the work for identifying when new lines are encountered
// to increment the line counter.
func (l *Lexer) readNextSignificant() (rune, int) {
	var done bool
	var was000D bool

	r := er
	w := 0

	for !done && r != eof {
		r, w = l.read()

		was000D = r == cr

		switch {
		case was000D:
			// Carriage return, i.e. '\r'.
			l.line++
			l.lpos = 0
		case r == lf:
			// Line feed, i.e. '\n'.
			if !was000D {
				// \r\n is not 2 newlines, so we must check what the last rune was.
				l.line++
				l.lpos = 0
			}
		case r == tab || r == ws || r == com || r == bom:
			// Skip!
		default:
			// Done, this run was significant.
			done = true
		}
	}

	return r, w
}

// read moves forward in the input, and returns the next rune available. This function also updates
// the position(s) that the lexer keeps track of in the input so the next read continues from where
// the last left off. Returns the EOF rune if we hit the end of the input.
func (l *Lexer) read() (rune, int) {
	if l.pos >= l.inputLen {
		return eof, 0
	}

	r, w := rune(l.input[l.pos]), 1
	if r >= utf8.RuneSelf {
		r, w = utf8.DecodeRune(l.input[l.pos:])
	}

	l.pos += w
	l.lpos++

	return r, w
}

// unread goes back one rune's worth of bytes in the input, changing the
// positions we keep track of.
// Does not currently go back a line.
func (l *Lexer) unread(width int) {
	l.pos -= width

	if l.lpos > 0 {
		l.lpos--
	}
}

// btos takes the given bytes, and turns them into a string.
// Q: naming btos or bbtos? :D
// TODO(seeruk): Is this actually portable then?
func btos(bs []byte) string {
	return *(*string)(unsafe.Pointer(&bs))
}
