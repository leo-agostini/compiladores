package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

type Token struct {
	Type string
	// Concept e o papel semantico da palavra reservada (if, while, ...), usado
	// pelo parser. Vazio para tokens que nao sao palavras reservadas.
	Concept string
	Value   string
	Line    int
}

type rule struct {
	tokenType string
	pattern   *regexp.Regexp
}

// anchored ancora o padrao no inicio do texto restante, para casar token a
// token conforme a varredura avanca.
func anchored(pattern string) *regexp.Regexp {
	return regexp.MustCompile(`^(?:` + pattern + `)`)
}

// A ordem importa: o primeiro padrao que casar vence. Por isso os operadores
// de dois caracteres vem antes dos de um (== antes de =) e FLOAT antes de INT
// (senao "3.14" viraria INT 3 seguido de lixo).
var rules = []rule{
	{"COMMENT", anchored(`//[^\n]*`)},
	{"COMMENT", anchored(`/\*[\s\S]*?\*/`)},
	{"WORD", anchored(`[a-zA-Z_][a-zA-Z0-9_]*`)},
	{"FLOAT", anchored(`\d+\.\d+`)},
	{"INT", anchored(`\d+`)},
	{"STRING", anchored(`"(\\.|[^"\\\n])*"`)},
	{"OPERATOR", anchored(`==|!=|<=|>=|[+\-*/<>=]`)},
	{"DELIMITER", anchored(`[{}();,]`)},
}

type reservedWord struct {
	tokenType string
	concept   string
}

// Palavras reservadas: casadas como WORD e reclassificadas aqui. Fazer assim,
// em vez de um regex por palavra, garante que "casos" seja IDENTIFIER e nao
// KEYWORD "caso" seguida de "s", e impede usar reservadas como identificador.
var reserved = map[string]reservedWord{
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
	"if":      {"KEYWORD", "if"},
	"else if": {"KEYWORD", "else if"},
	"else":    {"KEYWORD", "else"},
	"for":     {"KEYWORD", "for"},
	"while":   {"KEYWORD", "while"},
	"return":  {"KEYWORD", "return"},
	"print":   {"KEYWORD", "print"},
	"true":    {"BOOL_LITERAL", "true"},
	"false":   {"BOOL_LITERAL", "false"},

	// Tipos.
	"int":    {"TYPE", "int"},
	"string": {"TYPE", "string"},
	"float":  {"TYPE", "float"},
	"bool":   {"TYPE", "bool"},
}

var whitespace = anchored(`[ \t\r\n]+`)

func tokenize(code string) []Token {
	var tokens []Token
	line := 1

	for pos := 0; pos < len(code); {
		rest := code[pos:]

		if ws := whitespace.FindString(rest); ws != "" {
			line += strings.Count(ws, "\n")
			pos += len(ws)
			continue
		}

		matched := false
		for _, r := range rules {
			value := r.pattern.FindString(rest)
			if value == "" {
				continue
			}

			token := Token{Type: r.tokenType, Value: value, Line: line}
			if token.Type == "WORD" {
				token.Type = "IDENTIFIER"
				if word, ok := reserved[value]; ok {
					token.Type = word.tokenType
					token.Concept = word.concept
				}
			}

			tokens = append(tokens, token)
			// Um comentario de bloco ocupa varias linhas: sem isso, todo
			// token depois dele reporta a linha errada.
			line += strings.Count(value, "\n")
			pos += len(value)
			matched = true
			break
		}

		// Caractere fora do alfabeto da linguagem: registra o erro lexico e
		// segue, para reportar todos os problemas numa passada so.
		if !matched {
			tokens = append(tokens, Token{Type: "ERROR", Value: string(code[pos]), Line: line})
			pos++
		}
	}

	return tokens
}

// display achata o lexema em uma linha so e corta o excesso, para que um
// comentario de bloco multilinha nao quebre a tabela da saida.
func display(value string) string {
	flat := strings.Join(strings.Fields(value), " ")
	if runes := []rune(flat); len(runes) > 22 {
		flat = string(runes[:19]) + "..."
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

	errors := 0
	for _, token := range tokenize(string(code)) {
		concept := ""
		if token.Concept != "" {
			concept = "(" + token.Concept + ")"
		}
		fmt.Printf("linha %2d | %-22s -> %-12s %s\n", token.Line, display(token.Value), token.Type, concept)

		if token.Type == "ERROR" {
			errors++
		}
	}

	if errors > 0 {
		fmt.Fprintf(os.Stderr, "\n%d erro(s) lexico(s) encontrado(s)\n", errors)
		os.Exit(1)
	}
}
