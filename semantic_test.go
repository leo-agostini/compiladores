package main

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// le, tokeniza e analisa. Falha se o arquivo nem passar do parser.
func analyzeFile(t *testing.T, path string) (string, []Symbol, []string) {
	t.Helper()
	code, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("erro ao ler %s: %v", path, err)
	}
	tree, errs := Parse(tokenize(string(code)))
	if len(errs) > 0 {
		t.Fatalf("%s tem erro lexico/sintatico, a semantica nao roda:\n%s", path, strings.Join(errs, "\n"))
	}
	symbols, semErrs := Analyze(tree)
	return string(code), symbols, semErrs
}

func TestSemanticaValida(t *testing.T) {
	_, symbols, errs := analyzeFile(t, "semantica.clt")
	if len(errs) > 0 {
		t.Fatalf("esperado nenhum erro semantico, veio:\n%s", strings.Join(errs, "\n"))
	}

	got := map[string]bool{}
	for _, s := range symbols {
		got[fmt.Sprintf("%s %s %s %s", s.Name, kindLabel(s.Kind), typeLabel(s), s.Scope)] = true
	}
	// a tabela do slide 19, mais parametro, sombreamento e for.
	want := []string{
		"gg variavel int global",
		"main funcao int() global",
		"a variavel int local (main)",
		"b variavel int local (main)",
		"c variavel int aninhado (main)",
		"fatorial funcao int(int) global",
		"n parametro int local (fatorial)",
		"media funcao float(int, int) global",
		"x variavel int local (escopos)",
		"x variavel int aninhado (escopos)",
		"i variavel int aninhado (escopos)",
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("tabela sem a entrada %q\n%s", w, FormatSymbols(symbols))
		}
	}
}

var esperaRe = regexp.MustCompile(`//\s*espera:\s*(.+)$`)
var linhaRe = regexp.MustCompile(`^linha (\d+), col \d+: `)

// cada linha com "// espera: a | b" deve gerar exatamente esses erros.
func TestSemanticaErros(t *testing.T) {
	code, _, errs := analyzeFile(t, "semantica_erros.clt")

	var want []string
	for i, line := range strings.Split(code, "\n") {
		m := esperaRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		for _, msg := range strings.Split(m[1], " | ") {
			want = append(want, fmt.Sprintf("linha %d: %s", i+1, strings.TrimSpace(msg)))
		}
	}
	if len(want) == 0 {
		t.Fatal("nenhum '// espera:' encontrado no arquivo")
	}

	// a coluna fica de fora: o comentario marca a linha.
	got := make([]string, len(errs))
	for i, e := range errs {
		got[i] = linhaRe.ReplaceAllString(e, "linha ${1}: ")
	}

	sort.Strings(want)
	sort.Strings(got)
	if strings.Join(got, "\n") == strings.Join(want, "\n") {
		return
	}
	missing, extra := diff(want, got)
	t.Errorf("erros nao batem\nfaltando:\n  %s\nsobrando:\n  %s",
		strings.Join(missing, "\n  "), strings.Join(extra, "\n  "))
}

// o que esta so em a e o que esta so em b, contando repeticao.
func diff(a, b []string) (onlyA, onlyB []string) {
	count := map[string]int{}
	for _, s := range a {
		count[s]++
	}
	for _, s := range b {
		count[s]--
	}
	for s, n := range count {
		for ; n > 0; n-- {
			onlyA = append(onlyA, s)
		}
		for ; n < 0; n++ {
			onlyB = append(onlyB, s)
		}
	}
	return onlyA, onlyB
}
