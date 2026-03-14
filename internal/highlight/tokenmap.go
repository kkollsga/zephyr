package highlight

import "image/color"

// TokenType represents a syntax highlighting token category.
type TokenType string

const (
	TokenKeyword  TokenType = "keyword"
	TokenString   TokenType = "string"
	TokenComment  TokenType = "comment"
	TokenFunction TokenType = "function"
	TokenType_    TokenType = "type"
	TokenNumber   TokenType = "number"
	TokenOperator TokenType = "operator"
	TokenVariable TokenType = "variable"
)

// Token represents a highlighted span of text.
type Token struct {
	StartByte int
	EndByte   int
	Type      TokenType
}

// TokenColorMap maps token types to colors.
type TokenColorMap map[TokenType]color.NRGBA

// CaptureNameToTokenType converts a tree-sitter capture name to a TokenType.
func CaptureNameToTokenType(name string) TokenType {
	switch name {
	case "keyword":
		return TokenKeyword
	case "string":
		return TokenString
	case "comment":
		return TokenComment
	case "function":
		return TokenFunction
	case "type":
		return TokenType_
	case "number":
		return TokenNumber
	case "operator":
		return TokenOperator
	case "variable":
		return TokenVariable
	default:
		return ""
	}
}
