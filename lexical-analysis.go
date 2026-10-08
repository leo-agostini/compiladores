package main

import (
	"fmt"
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
	// O sufixo aceita qualquer letra: so [0-9a-f] deixaria "0xG" como
	// ERROR "0x" seguido de IDENTIFIER "G".
	{tokenType: "ERROR", pattern: anchored(`0[xXbBoO][0-9a-zA-Z_]*`),
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
	// Quebra de linha com a aspa de fechamento na linha seguinte: um erro
	// so. Sem exigir a aspa, "x\ny = 1;" engoliria o comando de baixo.
	{tokenType: "ERROR", pattern: anchored(`"(?:\\.|[^"\\\n])*\n[ \t]*(?:[a-zA-Z_][a-zA-Z0-9_]*)?"`),
		message: "string quebrada em duas linhas; para quebra de linha use \\n dentro das aspas"},
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
	// Uma por conceito acima, para que todo conceito tenha os dois nomes.
	"if":       {"KEYWORD", "if"},
	"else":     {"KEYWORD", "else"},
	"for":      {"KEYWORD", "for"},
	"while":    {"KEYWORD", "while"},
	"function": {"KEYWORD", "function"},
	"return":   {"KEYWORD", "return"},
	"print":    {"KEYWORD", "print"},
	"input":    {"KEYWORD", "input"},
	"var":      {"KEYWORD", "var"},
	"break":    {"KEYWORD", "break"},
	"true":     {"BOOL_LITERAL", "true"},
	"false":    {"BOOL_LITERAL", "false"},

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
			if token.Concept == "if" {
				if prev := lastMeaningful(tokens); prev != nil && prev.Concept == "else" {
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

// lastMeaningful devolve o ultimo token que nao e comentario. A fusao de
// "senao caso" precisa disso: um comentario entre as duas palavras
// ("senao /*x*/ caso") nao pode quebrar a construcao, ja que ele nem chega ao
// analisador sintatico.
func lastMeaningful(tokens []Token) *Token {
	for i := len(tokens) - 1; i >= 0; i-- {
		if tokens[i].Type != "COMMENT" {
			return &tokens[i]
		}
	}
	return nil
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

// columnOf conta a coluna (1-based) da posicao dentro da linha atual, em runes
// e nao em bytes: "á" ocupa dois bytes, e sem contar runes todo token depois
// dele na linha reportaria uma coluna a mais. Calcular sob demanda evita manter
// um contador de coluna sincronizado com os saltos dos lexemas multilinha.
func columnOf(code string, pos int) int {
	lineStart := strings.LastIndex(code[:pos], "\n") + 1
	return utf8.RuneCountInString(code[lineStart:pos]) + 1
}
