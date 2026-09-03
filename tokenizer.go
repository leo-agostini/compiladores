package main

import (
	"fmt"
	"os"
	"strings"
	"unicode/utf8"
)

type Token struct {
	Type string
	// Concept e o papel semantico da palavra reservada (if, while, ...), usado
	// pelo parser. Vazio para tokens que nao sao palavras reservadas.
	Concept string
	Value   string
	Line    int
	Column  int
	// Message explica o ERROR com a causa exata. Vazio nos tokens validos.
	Message string
}

// Palavras reservadas: casadas como WORD e reclassificadas aqui. Fazer assim,
// em vez de um regex por palavra, garante que "casos" seja IDENTIFIER e nao
// KEYWORD "caso" seguida de "s", e impede usar reservadas como identificador.
var reserved = map[string][2]string{
	// Palavras da linguagem.
	"caso":        {"KEYWORD", "if"},
	"recurso":     {"KEYWORD", "else if"},
	"senao":       {"KEYWORD", "else"},
	"jornada":     {"KEYWORD", "for"},
	"plantao":     {"KEYWORD", "while"},
	"contratar":   {"KEYWORD", "function"},
	"rescindir":   {"KEYWORD", "return"},
	"bater_ponto": {"KEYWORD", "print"},
	"reclamacao":  {"KEYWORD", "input"},
	"carteira":    {"KEYWORD", "var"},
	"falta":       {"KEYWORD", "break"},
	"registrado":  {"BOOL_LITERAL", "true"},
	"pj":          {"BOOL_LITERAL", "false"},

	// Palavras-chave em ingles.
	"if":     {"KEYWORD", "if"},
	"else":   {"KEYWORD", "else"},
	"for":    {"KEYWORD", "for"},
	"while":  {"KEYWORD", "while"},
	"return": {"KEYWORD", "return"},
	"print":  {"KEYWORD", "print"},
	"true":   {"BOOL_LITERAL", "true"},
	"false":  {"BOOL_LITERAL", "false"},

	// Tipos.
	"int":    {"TYPE", "int"},
	"string": {"TYPE", "string"},
	"float":  {"TYPE", "float"},
	"bool":   {"TYPE", "bool"},
	"void":   {"TYPE", "void"},
}

// A ordem importa: o primeiro padrao que casar vence. Por isso os operadores
// de dois caracteres vem antes dos de um (== antes de =).
var twoCharOps = []string{"==", "!=", "<=", ">="}

// Operadores comuns em outras linguagens que esta nao reconhece.
// Detectados de proposito para a mensagem nao ser so "caractere invalido".
var unsupportedOps = []string{"&&", "||", "++", "--", "+=", "-=", "*=", "/=", "%=", "<<", ">>"}

var stringEscapes = map[rune]bool{'n': true, 't': true, 'r': true, '"': true, '\\': true, '0': true}

var extraErr = map[rune]string{
	'!': "operador '!' nao existe; comparacao de desigualdade e '!='",
	'%': "operador '%' (modulo) nao existe na linguagem",
	'&': "caractere '&' invalido; a linguagem nao tem '&' nem '&&'",
	'|': "caractere '|' invalido; a linguagem nao tem '|' nem '||'",
	'[': "delimitador '[' nao existe; a linguagem nao tem vetores",
	']': "delimitador ']' nao existe; a linguagem nao tem vetores",
	':': "caractere ':' invalido",
	'?': "caractere '?' invalido; nao ha operador ternario",
	'#': "caractere '#' invalido",
	'@': "caractere '@' invalido",
	'$': "caractere '$' invalido",
	'`': "caractere '`' invalido; strings usam aspas duplas",
}

type scanner struct {
	src       string
	pos       int
	line, col int
	tokens    []Token
}

func tokenize(code string) []Token {
	s := &scanner{src: code, line: 1, col: 1}
	for !s.eof() {
		s.takeWhile(isSpace)
		if s.eof() {
			break
		}
		line, col := s.line, s.col
		switch r := s.peek(); {
		case s.has("//"):
			start := s.pos
			s.takeWhile(func(r rune) bool { return r != '\n' })
			s.emit("COMMENT", s.src[start:s.pos], line, col, "")
		case s.has("/*"):
			s.blockComment(line, col)
		case r == '"':
			s.str(line, col)
		case r == '\'':
			s.charLit(line, col)
		case isDigit(r):
			s.number(line, col)
		case r == '.':
			s.dot(line, col)
		case isIdentStart(r):
			s.word(line, col)
		default:
			// Caractere fora do alfabeto da linguagem: registra o erro lexico e
			// segue, para reportar todos os problemas numa passada so.
			s.opOrErr(line, col)
		}
	}
	return s.tokens
}

func (s *scanner) eof() bool { return s.pos >= len(s.src) }

func (s *scanner) peek() rune {
	if s.eof() {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(s.src[s.pos:])
	return r
}

func (s *scanner) peekAt(off int) rune {
	i := s.pos + off
	if i >= len(s.src) {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(s.src[i:])
	return r
}

func (s *scanner) has(p string) bool {
	return strings.HasPrefix(s.src[s.pos:], p)
}

func (s *scanner) advance() rune {
	r, w := utf8.DecodeRuneInString(s.src[s.pos:])
	s.pos += w
	if r == '\n' {
		s.line++
		s.col = 1
	} else {
		s.col++
	}
	return r
}

func (s *scanner) take(n int) {
	for i := 0; i < n && !s.eof(); i++ {
		s.advance()
	}
}

func (s *scanner) takeWhile(ok func(rune) bool) {
	for !s.eof() && ok(s.peek()) {
		s.advance()
	}
}

func (s *scanner) emit(typ, val string, line, col int, msg string) {
	s.tokens = append(s.tokens, Token{Type: typ, Value: val, Line: line, Column: col, Message: msg})
}

func (s *scanner) lastCode() *Token {
	for i := len(s.tokens) - 1; i >= 0; i-- {
		if s.tokens[i].Type != "COMMENT" {
			return &s.tokens[i]
		}
	}
	return nil
}

func (s *scanner) blockComment(line, col int) {
	start := s.pos
	s.take(2)
	for !s.eof() && !s.has("*/") {
		s.advance()
	}
	if s.has("*/") {
		s.take(2)
		// Um comentario de bloco ocupa varias linhas: sem isso, todo
		// token depois dele reporta a linha errada.
		s.emit("COMMENT", s.src[start:s.pos], line, col, "")
		return
	}
	s.emit("ERROR", s.src[start:s.pos], line, col, "comentario de bloco nao terminado (falta */)")
}

func (s *scanner) str(line, col int) {
	start := s.pos
	s.advance()
	closed := false
	var bad rune
	badCol := col

	for !s.eof() && s.peek() != '\n' {
		if s.peek() == '\\' {
			escCol := s.col
			s.advance()
			if s.eof() || s.peek() == '\n' {
				break
			}
			if bad == 0 && !stringEscapes[s.peek()] {
				bad, badCol = s.peek(), escCol
			}
			s.advance()
			continue
		}
		if s.advance() == '"' {
			closed = true
			break
		}
	}

	val := s.src[start:s.pos]
	switch {
	case !closed:
		s.emit("ERROR", val, line, col, "string nao terminada (aspas de fechamento ausentes)")
	case bad != 0:
		s.emit("ERROR", val, line, badCol,
			fmt.Sprintf("escape invalido '\\%c' em string (validos: \\n \\t \\r \\\" \\\\ \\0)", bad))
	default:
		s.emit("STRING", val, line, col, "")
	}
}

func (s *scanner) charLit(line, col int) {
	start := s.pos
	s.advance()
	if !s.eof() && s.peek() != '\n' {
		if s.advance() == '\\' && !s.eof() && s.peek() != '\n' {
			s.advance()
		}
	}
	if s.peek() == '\'' {
		s.advance()
	}
	s.emit("ERROR", s.src[start:s.pos], line, col,
		"literal de caractere com aspas simples nao e suportado; use string com aspas duplas")
}

func (s *scanner) number(line, col int) {
	start := s.pos
	s.takeWhile(isDigit)
	// FLOAT antes de INT: senao "3.14" viraria INT 3 seguido de lixo.
	if s.peek() == '.' && isDigit(s.peekAt(1)) {
		s.advance()
		s.takeWhile(isDigit)
		s.finishNum(start, line, col, "FLOAT")
		return
	}
	if s.peek() == '.' {
		s.advance()
		s.emit("ERROR", s.src[start:s.pos], line, col, "float malformado: falta parte decimal depois do ponto")
		return
	}
	s.finishNum(start, line, col, "INT")
}

func (s *scanner) finishNum(start, line, col int, kind string) {
	if s.peek() == '.' {
		s.emit(kind, s.src[start:s.pos], line, col, "")
		s.emit("ERROR", ".", s.line, s.col, "ponto inesperado depois de literal numerico")
		s.advance()
		return
	}
	if s.peek() == 'e' || s.peek() == 'E' {
		pos, c := s.pos, s.col
		s.advance()
		if s.peek() == '+' || s.peek() == '-' {
			s.advance()
		}
		if isDigit(s.peek()) {
			s.takeWhile(isDigit)
			s.emit("ERROR", s.src[start:s.pos], line, col, "notacao cientifica nao suportada")
			return
		}
		s.pos, s.col = pos, c
	}
	if isIdentStart(s.peek()) {
		s.takeWhile(isIdentPart)
		s.emit("ERROR", s.src[start:s.pos], line, col,
			fmt.Sprintf("literal %s colado em identificador; separe com espaco", strings.ToLower(kind)))
		return
	}
	s.emit(kind, s.src[start:s.pos], line, col, "")
}

func (s *scanner) dot(line, col int) {
	if isDigit(s.peekAt(1)) {
		start := s.pos
		s.advance()
		s.takeWhile(isDigit)
		s.emit("ERROR", s.src[start:s.pos], line, col,
			"float malformado: falta parte inteira antes do ponto (use 0.5, nao .5)")
		return
	}
	s.advance()
	s.emit("ERROR", ".", line, col, "caractere '.' nao pertence ao alfabeto da linguagem")
}

func (s *scanner) word(line, col int) {
	start := s.pos
	s.takeWhile(isIdentPart)
	val := s.src[start:s.pos]
	tok := Token{Type: "IDENTIFIER", Value: val, Line: line, Column: col}
	if w, ok := reserved[val]; ok {
		tok.Type, tok.Concept = w[0], w[1]
	}

	// "else if" nao pode casar como WORD (identificador nao aceita
	// espaco), entao else seguido de if e fundido aqui num token
	// so, igualando "senao caso" ao "recurso" de uma palavra.
	// Comentarios no meio sao ignorados na hora de fundir.
	if tok.Concept == "if" {
		if prev := s.lastCode(); prev != nil && prev.Concept == "else" {
			prev.Concept = "else if"
			prev.Value += " " + val
			return
		}
	}
	s.tokens = append(s.tokens, tok)
}

func (s *scanner) opOrErr(line, col int) {
	for _, op := range twoCharOps {
		if s.has(op) {
			s.take(len(op))
			s.emit("OPERATOR", op, line, col, "")
			return
		}
	}
	for _, op := range unsupportedOps {
		if s.has(op) {
			s.take(len(op))
			s.emit("ERROR", op, line, col, fmt.Sprintf("operador '%s' nao existe na linguagem", op))
			return
		}
	}
	r := s.peek()
	s.advance()
	switch {
	case strings.ContainsRune("+-*/<>=", r):
		s.emit("OPERATOR", string(r), line, col, "")
	case strings.ContainsRune("{}();,", r):
		s.emit("DELIMITER", string(r), line, col, "")
	default:
		msg := extraErr[r]
		if msg == "" {
			msg = fmt.Sprintf("caractere invalido %q", r)
		}
		s.emit("ERROR", string(r), line, col, msg)
	}
}

func isSpace(r rune) bool      { return r == ' ' || r == '\t' || r == '\r' || r == '\n' }
func isDigit(r rune) bool      { return r >= '0' && r <= '9' }
func isIdentStart(r rune) bool { return r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') }
func isIdentPart(r rune) bool  { return isIdentStart(r) || isDigit(r) }

// display achata o lexema em uma linha so e corta o excesso, para que um
// comentario de bloco multilinha nao quebre a tabela da saida.
func display(v string) string {
	flat := strings.Join(strings.Fields(v), " ")
	if flat == "" {
		flat = strings.ReplaceAll(v, "\n", "\\n")
	}
	if rs := []rune(flat); len(rs) > 22 {
		return string(rs[:19]) + "..."
	}
	return flat
}

func main() {
	path := "teste.clt"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	code, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "erro ao ler o arquivo: %v\n", err)
		os.Exit(1)
	}

	var errs []Token
	for _, t := range tokenize(string(code)) {
		concept := ""
		if t.Concept != "" {
			concept = "(" + t.Concept + ")"
		}
		fmt.Printf("linha %2d, col %-3d | %-22s -> %-12s %s\n",
			t.Line, t.Column, display(t.Value), t.Type, concept)
		if t.Type == "ERROR" {
			errs = append(errs, t)
		}
	}
	if len(errs) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "\n%d erro(s) lexico(s):\n", len(errs))
	for i, e := range errs {
		fmt.Fprintf(os.Stderr, "  %d) linha %d, col %d: %s\n     lexema: %s\n",
			i+1, e.Line, e.Column, e.Message, display(e.Value))
	}
	os.Exit(1)
}
