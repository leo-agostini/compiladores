package main

import (
	"fmt"
	"os"
	"strings"
)

// display achata o lexema em uma linha so e corta o excesso, para que um
// comentario de bloco multilinha nao quebre a tabela da saida.
func display(value string) string {
	flat := strings.Join(strings.Fields(value), " ")
	if runes := []rune(flat); len(runes) > 22 {
		flat = string(runes[:19]) + "..."
	}
	return flat
}

// printTree imprime a AST indentada, dois espacos por nivel. Depende so da
// interface Node, entao um no novo aparece sem mexer aqui.
func printTree(node Node, depth int) {
	fmt.Printf("%s%s\n", strings.Repeat("  ", depth), node.Label())
	for _, child := range node.Children() {
		printTree(child, depth+1)
	}
}

func main() {
	path := "testes/teste.clt"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}

	code, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "erro ao ler o arquivo: %v\n", err)
		os.Exit(1)
	}

	// Fase 1: analise lexica.
	tokens := tokenize(string(code))

	fmt.Println("== analise lexica ==")
	var lexicalErrors []Token
	for _, token := range tokens {
		concept := ""
		if token.Concept != "" {
			concept = "(" + token.Concept + ")"
		}
		fmt.Printf("linha %2d, col %2d | %-22s -> %-12s %s\n", token.Line, token.Column, display(token.Value), token.Type, concept)

		if token.Type == "ERROR" {
			lexicalErrors = append(lexicalErrors, token)
		}
	}

	// Um token ERROR nao encaixa em nenhuma regra da gramatica: seguir para o
	// parser so produziria erros sintaticos derivados do erro lexico.
	if len(lexicalErrors) > 0 {
		fmt.Fprintf(os.Stderr, "\n%d erro(s) lexico(s) encontrado(s):\n", len(lexicalErrors))
		for i, token := range lexicalErrors {
			fmt.Fprintf(os.Stderr, "  %d) linha %d, col %d: %s\n     lexema: %s\n",
				i+1, token.Line, token.Column, token.Message, display(token.Value))
		}
		fmt.Fprintln(os.Stderr, "\nanalise sintatica nao executada: corrija os erros lexicos primeiro.")
		os.Exit(1)
	}

	// Fase 2: analise sintatica.
	program, syntaxErrors := parse(tokens)

	if len(syntaxErrors) > 0 {
		fmt.Fprintf(os.Stderr, "\n%d erro(s) sintatico(s) encontrado(s):\n", len(syntaxErrors))
		for i, syntaxError := range syntaxErrors {
			fmt.Fprintf(os.Stderr, "  %d) linha %d, col %d: %s\n",
				i+1, syntaxError.Line, syntaxError.Column, syntaxError.Message)
		}
		os.Exit(1)
	}

	fmt.Println("\n== analise sintatica ==")
	printTree(program, 0)
}
