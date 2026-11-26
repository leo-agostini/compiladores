package main

import (
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"
)

// ---------------------------------------------------------------------------
// Geracao de codigo: TAC -> MIPS (SPIM / MARS)
// ---------------------------------------------------------------------------
//
// Alocacao ingenua: todo nome do TAC (variavel ou temporario) tem uma casa
// fixa na memoria, e os registradores so seguram valores durante uma unica
// instrucao do TAC. Para "x = y op z":
//
//	lw   $t0, y        # carrega os operandos
//	lw   $t1, z
//	op   $t0, $t0, $t1 # opera so em registrador
//	sw   $t0, x        # guarda o resultado na casa de x
//
// Nenhum valor sobrevive num registrador de uma instrucao para a outra, entao
// nao ha o que salvar em chamada de funcao nem o que derramar (spill). E o
// codigo mais lento possivel, e o ponto de partida das alocacoes melhores.
//
// Onde mora cada nome:
//   - variavel declarada no topo do arquivo: rotulo no .data (global);
//   - parametro: acima do $fp, onde quem chamou empilhou (8($fp), 12($fp)...);
//   - variavel local e temporario: abaixo do $fp, no quadro da funcao
//     (-4($fp), -8($fp)...). O corpo do programa tambem tem quadro.
//
// Registradores usados: $t0 e $t1 para int, bool e string (endereco); $f0 e
// $f2 para float (precisao simples); $v0 e $f0 no retorno; $a0 e $f12 nas
// chamadas de sistema.

// Location e a casa de um nome: um rotulo global ou um deslocamento do $fp.
type Location struct {
	Label  string
	Offset int
}

func (l Location) String() string {
	if l.Label != "" {
		return l.Label
	}
	return fmt.Sprintf("%d($fp)", l.Offset)
}

// Frame e o resultado da alocacao de uma funcao.
type Frame struct {
	Func *TACFunc
	// casa de cada nome usado na funcao (variavel, parametro, temporario).
	Homes map[string]Location
	// ordem em que os nomes apareceram, para imprimir a tabela.
	Order []string
	// bytes reservados abaixo do $fp para locais e temporarios.
	Size int
}

// Allocate distribui as casas de uma funcao, de forma ingenua: cada nome
// novo ganha os proximos 4 bytes do quadro, na ordem em que aparece no TAC.
// Nenhum espaco e reaproveitado, nem de temporario que ja morreu.
func Allocate(f *TACFunc) *Frame {
	fr := &Frame{Func: f, Homes: map[string]Location{}}

	// quem chama empilha os argumentos em ordem, entao o ultimo fica mais
	// perto do $fp. Os 8 primeiros bytes acima do $fp sao o $fp e o $ra
	// salvos pelo prologo.
	n := len(f.Params)
	for i, p := range f.Params {
		fr.add(p.Name, Location{Offset: 8 + 4*(n-1-i)})
	}

	place := func(o Operand) {
		if o.Kind != OpVar && o.Kind != OpTemp {
			return
		}
		if _, ok := fr.Homes[o.Name]; ok {
			return
		}
		if o.Global {
			fr.add(o.Name, Location{Label: globalLabel(o.Name)})
			return
		}
		fr.Size += 4
		fr.add(o.Name, Location{Offset: -fr.Size})
	}
	for _, in := range f.Code {
		place(in.Dst)
		place(in.A)
		place(in.B)
	}
	return fr
}

func (fr *Frame) add(name string, loc Location) {
	fr.Homes[name] = loc
	fr.Order = append(fr.Order, name)
}

// rotulo de variavel global no .data. O "x.1" do TAC vira "v1_x": o ponto
// nao e aceito em todo montador, e o prefixo com numero nao colide com o
// "v_" dos nomes sem versao.
func globalLabel(name string) string {
	if base, version, ok := strings.Cut(name, "."); ok {
		return "v" + version + "_" + base
	}
	return "v_" + name
}

// funcao do fonte ganha prefixo: uma funcao chamada "main" nao pode
// colidir com o ponto de entrada.
func funcLabel(name string) string {
	if base, version, ok := strings.Cut(name, "."); ok {
		return "f" + version + "_" + base
	}
	return "f_" + name
}

// FormatAllocation imprime onde mora cada nome, funcao por funcao.
func FormatAllocation(frames []*Frame) string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "FUNCAO\tNOME\tTIPO\tCASA")
	for _, fr := range frames {
		types := map[string]string{}
		kinds := map[string]string{}
		for _, p := range fr.Func.Params {
			types[p.Name], kinds[p.Name] = p.Type, "parametro"
		}
		note := func(o Operand) {
			if o.Kind != OpVar && o.Kind != OpTemp {
				return
			}
			types[o.Name] = o.Type
			if _, ok := kinds[o.Name]; ok {
				return
			}
			switch {
			case o.Kind == OpTemp:
				kinds[o.Name] = "temporario"
			case o.Global:
				kinds[o.Name] = "global"
			default:
				kinds[o.Name] = "local"
			}
		}
		for _, in := range fr.Func.Code {
			note(in.Dst)
			note(in.A)
			note(in.B)
		}
		name := fr.Func.Name
		for _, n := range fr.Order {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s (%s)\n", name, n, types[n], fr.Homes[n], kinds[n])
		}
	}
	w.Flush()
	return b.String()
}

// ---------------------------------------------------------------------------
// Emissor
// ---------------------------------------------------------------------------

type mipsGen struct {
	text strings.Builder
	fr   *Frame
	// constantes que precisam de .data: strings e floats.
	strings map[string]string
	floats  map[string]string
	strList []string
	fltList []string
	// rotulos internos (comparacao de float), unicos no arquivo.
	local int
	// rotinas de apoio que o programa usou.
	needRead, needStrEq, needBool bool
}

// GenerateMIPS traduz o TAC para assembly MIPS e devolve tambem os quadros
// (a tabela de alocacao).
func GenerateMIPS(prog *TACProgram) (string, []*Frame) {
	g := &mipsGen{strings: map[string]string{}, floats: map[string]string{}}

	var frames []*Frame
	for _, f := range prog.Funcs {
		fr := Allocate(f)
		frames = append(frames, fr)
		g.function(fr)
	}

	var out strings.Builder
	out.WriteString("# gerado pelo compilador CLT: TAC -> MIPS, alocacao ingenua\n")
	out.WriteString("# cada instrucao do TAC aparece como comentario antes do seu codigo\n\n")
	out.WriteString("    .data\n")
	for _, gl := range prog.Globals {
		if gl.Type == TypeFloat {
			fmt.Fprintf(&out, "%s: .float 0.0\n", globalLabel(gl.Name))
			continue
		}
		fmt.Fprintf(&out, "%s: .word 0\n", globalLabel(gl.Name))
	}
	for _, lit := range g.fltList {
		fmt.Fprintf(&out, "%s: .float %s\n", g.floats[lit], lit)
	}
	for _, lit := range g.strList {
		fmt.Fprintf(&out, "%s: %s\n", g.strings[lit], asciiz(lit))
	}
	if g.needBool {
		out.WriteString("_s_true: .asciiz \"true\"\n_s_false: .asciiz \"false\"\n")
	}
	out.WriteString("\n    .text\n    .globl main\n")
	out.WriteString(g.text.String())
	out.WriteString(g.runtime())
	return out.String(), frames
}

func (g *mipsGen) emit(format string, args ...any) {
	fmt.Fprintf(&g.text, "    "+format+"\n", args...)
}

func (g *mipsGen) labelHere(l string) { fmt.Fprintf(&g.text, "%s:\n", l) }

// os rotulos do TAC (L1, L2...) ja sao unicos no programa inteiro e nao
// colidem com os prefixos v_, f_ e _ usados aqui, entao passam como estao.
func (g *mipsGen) tacLabel(l string) string { return l }

func (g *mipsGen) newLocal() string {
	g.local++
	return fmt.Sprintf("_c%d", g.local)
}

// ---------------------------------------------------------------------------
// Prologo e epilogo
// ---------------------------------------------------------------------------

// quadro de uma funcao, depois do prologo:
//
//	 ...            argumentos empilhados por quem chamou
//	 8($fp)         ultimo argumento
//	 4($fp)         $ra salvo
//	 0($fp)         $fp de quem chamou
//	-4($fp)         primeira local/temporario
//	 ...
//	 0($sp)
func (g *mipsGen) function(fr *Frame) {
	g.fr = fr
	f := fr.Func
	fmt.Fprintf(&g.text, "\n")
	if f.Main {
		g.text.WriteString("# corpo do programa (comandos do topo do arquivo)\n")
		g.labelHere("main")
	} else {
		fmt.Fprintf(&g.text, "# func %s\n", f.Name)
		g.labelHere(funcLabel(f.Name))
	}
	g.emit("addiu $sp, $sp, -8")
	g.emit("sw $ra, 4($sp)")
	g.emit("sw $fp, 0($sp)")
	g.emit("move $fp, $sp")
	if fr.Size > 0 {
		g.emit("addiu $sp, $sp, -%d", fr.Size)
	}

	for _, in := range f.Code {
		if in.Kind == ILabel {
			g.labelHere(g.tacLabel(in.Label))
			continue
		}
		g.emit("# %s", in)
		g.instr(in)
	}

	if f.Main {
		g.emit("# fim do programa")
		g.emit("li $v0, 10")
		g.emit("syscall")
	}
}

func (g *mipsGen) epilogue() {
	g.emit("move $sp, $fp")
	g.emit("lw $fp, 0($sp)")
	g.emit("lw $ra, 4($sp)")
	g.emit("addiu $sp, $sp, 8")
	g.emit("jr $ra")
}

// ---------------------------------------------------------------------------
// Carga e armazenamento: o unico lugar que conhece as casas
// ---------------------------------------------------------------------------

func (g *mipsGen) home(o Operand) Location {
	loc, ok := g.fr.Homes[o.Name]
	if !ok {
		panic("sem casa para " + o.Name)
	}
	return loc
}

// load coloca um valor int, bool ou string (endereco) em reg.
func (g *mipsGen) load(reg string, o Operand) {
	if o.Kind == OpConst {
		switch o.Type {
		case TypeString:
			g.emit("la %s, %s", reg, g.stringConst(o.Name))
		case TypeBool:
			v := 0
			if o.Name == "true" {
				v = 1
			}
			g.emit("li %s, %d", reg, v)
		default:
			g.emit("li %s, %s", reg, o.Name)
		}
		return
	}
	g.emit("lw %s, %s", reg, g.home(o))
}

func (g *mipsGen) store(reg string, o Operand) {
	g.emit("sw %s, %s", reg, g.home(o))
}

// loadF coloca um float em freg.
func (g *mipsGen) loadF(freg string, o Operand) {
	if o.Kind == OpConst {
		g.emit("l.s %s, %s", freg, g.floatConst(o.Name))
		return
	}
	g.emit("l.s %s, %s", freg, g.home(o))
}

func (g *mipsGen) storeF(freg string, o Operand) {
	g.emit("s.s %s, %s", freg, g.home(o))
}

func (g *mipsGen) stringConst(lit string) string {
	if l, ok := g.strings[lit]; ok {
		return l
	}
	l := fmt.Sprintf("_str%d", len(g.strList)+1)
	g.strings[lit] = l
	g.strList = append(g.strList, lit)
	return l
}

func (g *mipsGen) floatConst(lit string) string {
	if l, ok := g.floats[lit]; ok {
		return l
	}
	l := fmt.Sprintf("_flt%d", len(g.fltList)+1)
	g.floats[lit] = l
	g.fltList = append(g.fltList, lit)
	return l
}

// asciiz reescreve o literal do fonte (com aspas e escapes da linguagem) como
// diretiva do montador. Com \r, \0 ou \\ no meio, vai byte a byte: o SPIM
// e o MARS nao tratam esses escapes do mesmo jeito no .asciiz.
func asciiz(lit string) string {
	raw := unquote(lit)
	if !strings.ContainsAny(raw, "\r\x00\\") {
		var b strings.Builder
		for _, c := range raw {
			switch c {
			case '\n':
				b.WriteString(`\n`)
			case '\t':
				b.WriteString(`\t`)
			case '"':
				b.WriteString(`\"`)
			default:
				b.WriteRune(c)
			}
		}
		return `.asciiz "` + b.String() + `"`
	}
	bytes := make([]string, 0, len(raw)+1)
	for i := 0; i < len(raw); i++ {
		bytes = append(bytes, strconv.Itoa(int(raw[i])))
	}
	bytes = append(bytes, "0")
	return ".byte " + strings.Join(bytes, ", ")
}

// tira as aspas e resolve os escapes validos da linguagem.
func unquote(lit string) string {
	body := strings.TrimSuffix(strings.TrimPrefix(lit, `"`), `"`)
	var b strings.Builder
	for i := 0; i < len(body); i++ {
		if body[i] != '\\' || i+1 == len(body) {
			b.WriteByte(body[i])
			continue
		}
		i++
		switch body[i] {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case '0':
			b.WriteByte(0)
		default:
			b.WriteByte(body[i])
		}
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// Uma instrucao do TAC
// ---------------------------------------------------------------------------

var intOps = map[string]string{"+": "addu", "-": "subu", "*": "mul"}
var floatOps = map[string]string{"+": "add.s", "-": "sub.s", "*": "mul.s", "/": "div.s"}

// comparacao de inteiros com as pseudo-instrucoes set-on-condition.
var intCmp = map[string]string{"<": "slt", ">": "sgt", "<=": "sle", ">=": "sge", "==": "seq", "!=": "sne"}

func (g *mipsGen) instr(in Instr) {
	switch in.Kind {
	case ICopy:
		if in.Dst.Type == TypeFloat {
			g.loadF("$f0", in.A)
			g.storeF("$f0", in.Dst)
			return
		}
		g.load("$t0", in.A)
		g.store("$t0", in.Dst)

	case IConvert:
		g.load("$t0", in.A)
		g.emit("mtc1 $t0, $f0")
		g.emit("cvt.s.w $f0, $f0")
		g.storeF("$f0", in.Dst)

	case IUnary:
		if in.A.Type == TypeFloat {
			g.loadF("$f0", in.A)
			g.emit("neg.s $f0, $f0")
			g.storeF("$f0", in.Dst)
			return
		}
		g.load("$t0", in.A)
		g.emit("subu $t0, $zero, $t0")
		g.store("$t0", in.Dst)

	case IBinary:
		if isRelational(in.Op) {
			g.compare(in.Op, in.A, in.B)
			g.store("$t0", in.Dst)
			return
		}
		if in.A.Type == TypeFloat {
			g.loadF("$f0", in.A)
			g.loadF("$f2", in.B)
			g.emit("%s $f0, $f0, $f2", floatOps[in.Op])
			g.storeF("$f0", in.Dst)
			return
		}
		g.load("$t0", in.A)
		g.load("$t1", in.B)
		if in.Op == "/" {
			g.emit("div $t0, $t1")
			g.emit("mflo $t0")
		} else {
			g.emit("%s $t0, $t0, $t1", intOps[in.Op])
		}
		g.store("$t0", in.Dst)

	case IGoto:
		g.emit("j %s", g.tacLabel(in.Label))

	case IIfFalse:
		if in.Op != "" {
			g.compare(in.Op, in.A, in.B)
		} else {
			g.load("$t0", in.A)
		}
		g.emit("beqz $t0, %s", g.tacLabel(in.Label))

	case IParam:
		// empilha: o argumento fica no topo da pilha ate o call.
		g.emit("addiu $sp, $sp, -4")
		if in.A.Type == TypeFloat {
			g.loadF("$f0", in.A)
			g.emit("s.s $f0, 0($sp)")
			return
		}
		g.load("$t0", in.A)
		g.emit("sw $t0, 0($sp)")

	case ICall:
		g.emit("jal %s", funcLabel(in.Label))
		if in.N > 0 {
			g.emit("addiu $sp, $sp, %d", 4*in.N)
		}
		if !in.Dst.Valid() {
			return
		}
		if in.Dst.Type == TypeFloat {
			g.storeF("$f0", in.Dst)
			return
		}
		g.store("$v0", in.Dst)

	case IReturn:
		if in.A.Valid() {
			if in.A.Type == TypeFloat {
				g.loadF("$f0", in.A)
			} else {
				g.load("$v0", in.A)
			}
		}
		g.epilogue()

	case IPrint:
		g.print(in.A)

	case IPrintNL:
		g.emit("li $a0, 10")
		g.emit("li $v0, 11")
		g.emit("syscall")

	case IRead:
		g.needRead = true
		g.emit("jal _read")
		g.store("$v0", in.Dst)
	}
}

// compare deixa 1 ou 0 em $t0.
func (g *mipsGen) compare(op string, a, b Operand) {
	switch a.Type {
	case TypeFloat:
		g.loadF("$f0", a)
		g.loadF("$f2", b)
		// o coprocessador so tem eq, lt e le: o resto inverte a ordem ou o
		// resultado.
		cmp, x, y, negate := "c.eq.s", "$f0", "$f2", false
		switch op {
		case "<":
			cmp = "c.lt.s"
		case "<=":
			cmp = "c.le.s"
		case ">":
			cmp, x, y = "c.lt.s", "$f2", "$f0"
		case ">=":
			cmp, x, y = "c.le.s", "$f2", "$f0"
		case "!=":
			negate = true
		}
		done := g.newLocal()
		yes, no := 1, 0
		if negate {
			yes, no = 0, 1
		}
		g.emit("li $t0, %d", yes)
		g.emit("%s %s, %s", cmp, x, y)
		g.emit("bc1t %s", done)
		g.emit("li $t0, %d", no)
		g.labelHere(done)

	case TypeString:
		g.needStrEq = true
		g.load("$a0", a)
		g.load("$a1", b)
		g.emit("jal _streq")
		g.emit("move $t0, $v0")
		if op == "!=" {
			g.emit("xori $t0, $t0, 1")
		}

	default:
		g.load("$t0", a)
		g.load("$t1", b)
		g.emit("%s $t0, $t0, $t1", intCmp[op])
	}
}

func (g *mipsGen) print(o Operand) {
	switch o.Type {
	case TypeFloat:
		g.loadF("$f12", o)
		g.emit("li $v0, 2")
		g.emit("syscall")
	case TypeString:
		g.load("$a0", o)
		g.emit("li $v0, 4")
		g.emit("syscall")
	case TypeBool:
		g.needBool = true
		g.load("$t0", o)
		g.emit("la $a0, _s_true")
		label := g.newLocal()
		g.emit("bnez $t0, %s", label)
		g.emit("la $a0, _s_false")
		g.labelHere(label)
		g.emit("li $v0, 4")
		g.emit("syscall")
	default:
		g.load("$a0", o)
		g.emit("li $v0, 1")
		g.emit("syscall")
	}
}

// rotinas de apoio, so as que o programa usou.
func (g *mipsGen) runtime() string {
	var parts []string
	if g.needRead {
		parts = append(parts, `
# _read: le uma linha da entrada, sem o '\n', num buffer novo de 256 bytes.
# Devolve o endereco em $v0.
_read:
    li $a0, 256
    li $v0, 9            # sbrk
    syscall
    move $t0, $v0
    move $a0, $v0
    li $a1, 256
    li $v0, 8            # read_string
    syscall
    move $t1, $t0
_read_loop:
    lb $t2, 0($t1)
    beqz $t2, _read_done
    li $t3, 10
    beq $t2, $t3, _read_cut
    addiu $t1, $t1, 1
    j _read_loop
_read_cut:
    sb $zero, 0($t1)
_read_done:
    move $v0, $t0
    jr $ra
`)
	}
	if g.needStrEq {
		parts = append(parts, `
# _streq: compara as strings em $a0 e $a1. $v0 = 1 se iguais, 0 se nao.
_streq:
    lb $t2, 0($a0)
    lb $t3, 0($a1)
    bne $t2, $t3, _streq_no
    beqz $t2, _streq_yes
    addiu $a0, $a0, 1
    addiu $a1, $a1, 1
    j _streq
_streq_yes:
    li $v0, 1
    jr $ra
_streq_no:
    li $v0, 0
    jr $ra
`)
	}
	return strings.Join(parts, "")
}
