package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// fonte -> TAC, falhando se alguma fase anterior reclamar.
func compileTAC(t *testing.T, code string) *TACProgram {
	t.Helper()
	tree, errs := Parse(tokenize(code))
	if len(errs) > 0 {
		t.Fatalf("erro lexico/sintatico:\n%s", strings.Join(errs, "\n"))
	}
	if _, semErrs := Analyze(tree); len(semErrs) > 0 {
		t.Fatalf("erro semantico:\n%s", strings.Join(semErrs, "\n"))
	}
	tac, genErrs := GenerateTAC(tree)
	if len(genErrs) > 0 {
		t.Fatalf("erro na geracao:\n%s", strings.Join(genErrs, "\n"))
	}
	return tac
}

func codeLines(f *TACFunc) []string {
	lines := make([]string, len(f.Code))
	for i, in := range f.Code {
		lines[i] = in.String()
	}
	return lines
}

// teste inicial da aula: o TAC de d = (a + b) * c, na ordem certa. A soma
// vem antes da multiplicacao (os parenteses), e a copia para d por ultimo.
func TestTACSlide(t *testing.T) {
	tac := compileTAC(t, `
carteira int a = 1;
carteira int b = 2;
carteira int c = 3;
carteira int d;
d = (a + b) * c;
`)
	got := codeLines(tac.Funcs[0])
	want := []string{
		"a = 1",
		"b = 2",
		"c = 3",
		"d = 0",
		"t1 = a + b",
		"t2 = t1 * c",
		"d = t2",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("TAC errado\nveio:\n  %s\nesperado:\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

func TestTACFormas(t *testing.T) {
	cases := []struct {
		name string
		code string
		want []string
	}{
		{
			name: "precedencia sem parenteses",
			code: "carteira int a; carteira int b; carteira int c; carteira int d; d = a + b * c;",
			want: []string{"t1 = b * c", "t2 = a + t1", "d = t2"},
		},
		{
			name: "int vira float antes da operacao",
			code: "carteira int i = 1; carteira float f = 2.5; f = f + i;",
			want: []string{"t1 = (float) i", "t2 = f + t1", "f = t2"},
		},
		{
			name: "constante int convertida na hora",
			code: "carteira float f = 2;",
			want: []string{"f = 2.0"},
		},
		{
			name: "atribuicao encadeada da direita para a esquerda",
			code: "carteira int a; carteira int b; a = b = 5;",
			want: []string{"b = 5", "a = b"},
		},
		{
			name: "while com comparacao direto no ifFalse",
			code: "carteira int n = 3; plantao (n > 0) { n = n - 1; }",
			want: []string{"L1:", "ifFalse n > 0 goto L2", "t1 = n - 1", "n = t1", "goto L1", "L2:"},
		},
		{
			name: "if/else",
			code: "carteira bool b; caso (b) { bater_ponto(1); } senao { bater_ponto(2); }",
			want: []string{"ifFalse b goto L1", "print 1", "println", "goto L2", "L1:", "print 2", "println", "L2:"},
		},
		{
			name: "for com break",
			code: "jornada (carteira int i = 0; i < 3; i = i + 1) { falta; }",
			want: []string{"i = 0", "L1:", "ifFalse i < 3 goto L2", "goto L2", "t1 = i + 1", "i = t1", "goto L1", "L2:"},
		},
		{
			name: "argumentos avaliados antes dos param",
			code: "int f(int x, int y) { rescindir x; } carteira int r = f(1, f(2, 3));",
			want: []string{"param 2", "param 3", "t1 = call f, 2", "param 1", "param t1", "t2 = call f, 2", "r = t2"},
		},
		{
			name: "sombreamento ganha outro nome",
			code: "carteira int x = 1; { carteira int x = 2; x = x + 1; }",
			want: []string{"x = 1", "x.1 = 2", "t1 = x.1 + 1", "x.1 = t1"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tac := compileTAC(t, c.code)
			got := codeLines(tac.Funcs[0])
			// declaracoes sem valor geram "x = 0" antes: compara so o fim.
			if len(got) > len(c.want) {
				got = got[len(got)-len(c.want):]
			}
			if strings.Join(got, "\n") != strings.Join(c.want, "\n") {
				t.Errorf("veio:\n  %s\nesperado:\n  %s", strings.Join(got, "\n  "), strings.Join(c.want, "\n  "))
			}
		})
	}
}

func TestTACFuncao(t *testing.T) {
	tac := compileTAC(t, `
int fatorial(int n) {
    caso (n <= 1) {
        rescindir 1;
    }
    rescindir n * fatorial(n - 1);
}
`)
	if len(tac.Funcs) != 2 {
		t.Fatalf("esperado programa + fatorial, veio %d funcoes", len(tac.Funcs))
	}
	got := codeLines(tac.Funcs[1])
	want := []string{
		"ifFalse n <= 1 goto L1",
		"return 1",
		"L1:",
		"t1 = n - 1",
		"param t1",
		"t2 = call fatorial, 1",
		"t3 = n * t2",
		"return t3",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("veio:\n  %s\nesperado:\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

// alocacao ingenua: cada nome uma casa propria, global no .data, local e
// temporario abaixo do $fp, parametro acima.
func TestAlocacao(t *testing.T) {
	tac := compileTAC(t, `
carteira int g = 1;
int f(int a, int b) {
    carteira int x = a + b;
    rescindir x * g;
}
`)
	fr := Allocate(tac.Funcs[1])
	want := map[string]string{
		"a":  "12($fp)",
		"b":  "8($fp)",
		"t1": "-4($fp)",
		"x":  "-8($fp)",
		"g":  "v_g",
		"t2": "-12($fp)",
	}
	for name, loc := range want {
		if got := fr.Homes[name].String(); got != loc {
			t.Errorf("%s: casa %s, esperado %s", name, got, loc)
		}
	}
	if fr.Size != 12 {
		t.Errorf("quadro de %d bytes, esperado 12", fr.Size)
	}
}

func TestNaoSuportado(t *testing.T) {
	tree, _ := Parse(tokenize("int f(int v) { int g() { rescindir v; } rescindir g(); }"))
	if _, errs := GenerateTAC(tree); len(errs) != 1 || !strings.Contains(errs[0], "funcao aninhada") {
		t.Errorf("esperado erro de funcao aninhada, veio %v", errs)
	}
}

// compila codigo.clt, roda no spim e compara com o bloco "saida esperada".
// Sem spim instalado, o teste e pulado.
func TestCodigoNoSpim(t *testing.T) {
	spim, err := exec.LookPath("spim")
	if err != nil {
		t.Skip("spim nao instalado")
	}
	code, err := os.ReadFile("codigo.clt")
	if err != nil {
		t.Fatal(err)
	}
	asm, _ := GenerateMIPS(compileTAC(t, string(code)))
	path := filepath.Join(t.TempDir(), "codigo.asm")
	if err := os.WriteFile(path, []byte(asm), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command(spim, "-file", path).CombinedOutput()
	if err != nil {
		t.Fatalf("spim falhou: %v\n%s", err, out)
	}
	// o spim abre com um cabecalho que termina na linha "Loaded: ...".
	got := string(out)
	if i := strings.Index(got, "Loaded:"); i >= 0 {
		got = got[i:]
		got = got[strings.Index(got, "\n")+1:]
	}

	m := regexp.MustCompile(`(?s)/\* saida esperada:\n(.*?)\*/`).FindStringSubmatch(string(code))
	if m == nil {
		t.Fatal("codigo.clt sem o bloco /* saida esperada: ... */")
	}
	if strings.TrimSpace(got) != strings.TrimSpace(m[1]) {
		t.Errorf("saida do spim\n%s\nesperada\n%s", got, m[1])
	}
}
