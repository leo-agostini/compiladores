package main

import (
	"fmt"
	"os"
	"regexp"
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
	// Message explica a causa de um ERROR. Vazio nos tokens validos.
	Message string
}

type rule struct {
	tokenType string
	pattern   *regexp.Regexp
	// message so e preenchido nas regras de ERROR: descreve por que o lexema
	// casado e invalido.
	message string
}

// anchored ancora o padrao no inicio do texto restante, para casar token a
// token conforme a varredura avanca.
func anchored(pattern string) *regexp.Regexp {
	return regexp.MustCompile(`^(?:` + pattern + `)`)
}

// A ordem importa: o primeiro padrao que casar vence. Por isso os operadores
// de dois caracteres vem antes dos de um (== antes de =) e FLOAT antes de INT
// (senao "3.14" viraria INT 3 seguido de lixo).
//
// As regras de ERROR moram na mesma tabela: um lexema malformado e consumido
// inteiro por um padrao proprio, o que da a mensagem e evita a cascata (o texto
// ruim nao volta para o laco virando identificadores soltos). Como Go usa RE2,
// que nao tem lookahead, onde seria preciso "casar X mas nao Y" a regra de erro
// vem depois da regra valida e fica so com o que sobrou.
var rules = []rule{
	{tokenType: "COMMENT", pattern: anchored(`//[^\n]*`)},
	{tokenType: "COMMENT", pattern: anchored(`/\*[\s\S]*?\*/`)},
	// Depois do bloco fechado, senao todo /* ... */ valido viraria erro.
	{tokenType: "ERROR", pattern: anchored(`/\*[\s\S]*`),
		message: "comentario de bloco aberto e nunca fechado com */"},

	{tokenType: "WORD", pattern: anchored(`[a-zA-Z_][a-zA-Z0-9_]*`)},

	// Numeros malformados, todos antes de FLOAT/INT para vencer a leitura
	// parcial ("1.2.3" seria FLOAT 1.2 seguido de lixo).
	{tokenType: "ERROR", pattern: anchored(`0[xXbBoO][0-9a-fA-F_]*`),
		message: "notacao hexadecimal/binaria nao existe; use numeros decimais"},
	{tokenType: "ERROR", pattern: anchored(`\d+(?:\.\d+)?[eE][+-]?\d+`),
		message: "notacao cientifica nao existe na linguagem"},
	{tokenType: "ERROR", pattern: anchored(`\d+(?:\.\d+){2,}`),
		message: "float com mais de um ponto decimal"},
	{tokenType: "ERROR", pattern: anchored(`\d+(?:\.\d+)?[a-zA-Z_][a-zA-Z0-9_]*`),
		message: "numero colado em identificador; falta um operador ou espaco"},
	{tokenType: "ERROR", pattern: anchored(`\.\d+`),
		message: "float sem parte inteira; escreva 0.5 em vez de .5"},

	{tokenType: "FLOAT", pattern: anchored(`\d+\.\d+`)},
	// Depois de FLOAT: "1.5" ja casou acima, entao aqui so cai o "1." solto.
	{tokenType: "ERROR", pattern: anchored(`\d+\.`),
		message: "float sem parte decimal; escreva 1.0 em vez de 1."},
	{tokenType: "INT", pattern: anchored(`\d+`)},

	{tokenType: "STRING", pattern: anchored(`"(\\.|[^"\\\n])*"`)},
	// Depois de STRING: sobra a abertura sem fechamento, consumida ate o fim
	// da linha para o conteudo nao virar identificador.
	{tokenType: "ERROR", pattern: anchored(`"(?:\\.|[^"\\\n])*`),
		message: "string nao terminada; falta fechar as aspas na mesma linha"},
	{tokenType: "ERROR", pattern: anchored(`'(?:\\.|[^'\\\n])*'?`),
		message: "aspas simples nao existem na linguagem; use aspas duplas"},

	// Operadores de outras linguagens: antes de OPERATOR, senao "++" viraria
	// dois "+". Em cada alternativa o ramo mais longo vem primeiro, porque Go
	// casa a primeira alternativa que der certo, nao a mais longa.
	{tokenType: "ERROR", pattern: anchored(`&&|\|\|`),
		message: "operadores logicos && e || nao existem na linguagem"},
	{tokenType: "ERROR", pattern: anchored(`\+\+|--`),
		message: "incremento/decremento nao existe; use x = x + 1"},
	{tokenType: "ERROR", pattern: anchored(`\+=|-=|\*=|/=|%=`),
		message: "atribuicao composta nao existe; use x = x + 1"},
	{tokenType: "ERROR", pattern: anchored(`<<|>>`),
		message: "operadores de deslocamento de bits nao existem na linguagem"},

	{tokenType: "OPERATOR", pattern: anchored(`==|!=|<=|>=|[+\-*/<>=]`)},

	// Depois de OPERATOR: "!=" ja casou acima, entao aqui so cai o "!" sozinho.
	{tokenType: "ERROR", pattern: anchored(`!`),
		message: "negacao '!' so e valida em '!='"},
	{tokenType: "ERROR", pattern: anchored(`%`),
		message: "operador de modulo '%' nao existe na linguagem"},
	{tokenType: "ERROR", pattern: anchored(`[&|]`),
		message: "caractere invalido; a linguagem nao tem operadores de bits"},
	{tokenType: "ERROR", pattern: anchored(`\.`),
		message: "'.' invalido; a linguagem nao tem acesso a campo"},

	{tokenType: "DELIMITER", pattern: anchored(`[{}();,]`)},
}

// Caracteres que nao iniciam nenhum lexema. Servem so para trocar o
// "fora do alfabeto" generico por uma explicacao do que o programador tentou.
var unknownChar = map[rune]string{
	'@':  "caractere '@' nao faz parte do alfabeto da linguagem",
	'#':  "caractere '#' invalido; comentario e // ou /* */",
	'$':  "caractere '$' nao faz parte do alfabeto da linguagem",
	'`':  "crase invalida; strings usam aspas duplas",
	'[':  "'[' invalido; a linguagem nao tem vetores",
	']':  "']' invalido; a linguagem nao tem vetores",
	'?':  "'?' invalido; a linguagem nao tem operador ternario",
	':':  "caractere ':' nao faz parte do alfabeto da linguagem",
	'\\': "'\\' so e valido dentro de uma string",
	'~':  "caractere '~' nao faz parte do alfabeto da linguagem",
	'^':  "caractere '^' nao faz parte do alfabeto da linguagem",
}

// Escapes aceitos dentro de uma string.
var validEscapes = map[byte]bool{'n': true, 't': true, 'r': true, '0': true, '"': true, '\\': true}

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

			token := Token{Type: r.tokenType, Value: value, Line: line, Column: columnOf(code, pos), Message: r.message}
			if token.Type == "WORD" {
				token.Type = "IDENTIFIER"
				if word, ok := reserved[value]; ok {
					token.Type = word.tokenType
					token.Concept = word.concept
				}
			}

			// O regex de STRING aceita qualquer coisa depois da barra, entao
			// o escape so da para validar com o lexema ja em maos.
			if token.Type == "STRING" {
				if escape, bad := invalidEscape(value); bad {
					token.Type = "ERROR"
					token.Message = `escape invalido \` + escape + ` na string; validos: \n \t \r \0 \" \\`
				}
			}

			// "else if" nao pode casar como WORD (o regex nao aceita
			// espaco), entao else seguido de if e fundido aqui num token
			// so, igualando "senao caso" ao "recurso" de uma palavra.
			if token.Concept == "if" && len(tokens) > 0 {
				if prev := &tokens[len(tokens)-1]; prev.Concept == "else" {
					prev.Concept = "else if"
					prev.Value += " " + value
					line += strings.Count(value, "\n")
					pos += len(value)
					matched = true
					break
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
		// segue, para reportar todos os problemas numa passada so. Consome uma
		// rune inteira, senao um caractere acentuado viraria dois erros com
		// bytes quebrados no lexema.
		if !matched {
			r, size := utf8.DecodeRuneInString(rest)
			message, ok := unknownChar[r]
			if !ok {
				message = fmt.Sprintf("caractere %q fora do alfabeto da linguagem", r)
			}
			tokens = append(tokens, Token{
				Type:    "ERROR",
				Value:   string(r),
				Line:    line,
				Column:  columnOf(code, pos),
				Message: message,
			})
			pos += size
		}
	}

	return tokens
}

// invalidEscape devolve o primeiro escape nao suportado do lexema de uma
// string. O lexema chega ja delimitado pelas aspas e com os escapes casados aos
// pares, entao basta olhar o caractere seguinte a cada barra.
func invalidEscape(value string) (string, bool) {
	for i := 0; i < len(value)-1; i++ {
		if value[i] != '\\' {
			continue
		}
		if !validEscapes[value[i+1]] {
			return string(value[i+1]), true
		}
		// Pula o caractere escapado: em "\\\\z" a segunda barra ja foi
		// consumida e o z nao e um escape.
		i++
	}
	return "", false
}

// columnOf conta a coluna (1-based) da posicao dentro da linha atual. Calcular
// sob demanda evita ter que manter um contador de coluna sincronizado com os
// saltos que o laco da ao consumir lexemas multilinha.
func columnOf(code string, pos int) int {
	return pos - strings.LastIndex(code[:pos], "\n")
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

	var errors []Token
	for _, token := range tokenize(string(code)) {
		concept := ""
		if token.Concept != "" {
			concept = "(" + token.Concept + ")"
		}
		fmt.Printf("linha %2d, col %2d | %-22s -> %-12s %s\n", token.Line, token.Column, display(token.Value), token.Type, concept)

		if token.Type == "ERROR" {
			errors = append(errors, token)
		}
	}

	if len(errors) > 0 {
		fmt.Fprintf(os.Stderr, "\n%d erro(s) lexico(s) encontrado(s):\n", len(errors))
		for i, token := range errors {
			fmt.Fprintf(os.Stderr, "  %d) linha %d, col %d: %s\n     lexema: %s\n",
				i+1, token.Line, token.Column, token.Message, display(token.Value))
		}
		os.Exit(1)
	}
}
