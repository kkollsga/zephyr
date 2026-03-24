package highlight

// jsonTokenize produces syntax tokens for JSON source.
// Keys are colored as variables, string values as strings.
func jsonTokenize(source []byte, startRow, endRow int) []Token {
	var tokens []Token
	i := 0
	row := 0
	n := len(source)

	for i < n {
		if row > endRow {
			break
		}

		ch := source[i]

		if ch == '\n' {
			row++
			i++
			continue
		}

		if row < startRow {
			i++
			continue
		}

		// Skip whitespace
		if ch == ' ' || ch == '\t' || ch == '\r' {
			i++
			continue
		}

		// String (key or value)
		if ch == '"' {
			start := i
			i++
			for i < n && source[i] != '"' && source[i] != '\n' {
				if source[i] == '\\' {
					i++ // skip escaped char
				}
				i++
			}
			if i < n && source[i] == '"' {
				i++
			}

			// Lookahead: if followed by ':', this is a key
			isKey := false
			for j := i; j < n; j++ {
				c := source[j]
				if c == ' ' || c == '\t' || c == '\r' {
					continue
				}
				if c == ':' {
					isKey = true
				}
				break
			}

			if isKey {
				tokens = append(tokens, Token{StartByte: start, EndByte: i, Type: TokenVariable})
			} else {
				tokens = append(tokens, Token{StartByte: start, EndByte: i, Type: TokenString})
			}
			continue
		}

		// Number
		if ch == '-' || (ch >= '0' && ch <= '9') {
			start := i
			if ch == '-' {
				i++
			}
			for i < n && source[i] >= '0' && source[i] <= '9' {
				i++
			}
			if i < n && source[i] == '.' {
				i++
				for i < n && source[i] >= '0' && source[i] <= '9' {
					i++
				}
			}
			if i < n && (source[i] == 'e' || source[i] == 'E') {
				i++
				if i < n && (source[i] == '+' || source[i] == '-') {
					i++
				}
				for i < n && source[i] >= '0' && source[i] <= '9' {
					i++
				}
			}
			tokens = append(tokens, Token{StartByte: start, EndByte: i, Type: TokenNumber})
			continue
		}

		// Keywords: true, false, null
		if ch == 't' && i+4 <= n && string(source[i:i+4]) == "true" {
			tokens = append(tokens, Token{StartByte: i, EndByte: i + 4, Type: TokenKeyword})
			i += 4
			continue
		}
		if ch == 'f' && i+5 <= n && string(source[i:i+5]) == "false" {
			tokens = append(tokens, Token{StartByte: i, EndByte: i + 5, Type: TokenKeyword})
			i += 5
			continue
		}
		if ch == 'n' && i+4 <= n && string(source[i:i+4]) == "null" {
			tokens = append(tokens, Token{StartByte: i, EndByte: i + 4, Type: TokenKeyword})
			i += 4
			continue
		}

		// Colon
		if ch == ':' {
			tokens = append(tokens, Token{StartByte: i, EndByte: i + 1, Type: TokenOperator})
			i++
			continue
		}

		// Everything else (brackets, commas) — skip
		i++
	}

	return tokens
}
