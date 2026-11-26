package main

import (
	"fmt"
	"strconv"
	"strings"
)

// ---------------------------------------------------------------------------
// Codigo de tres enderecos (TAC)
// ---------------------------------------------------------------------------

// tipo do operando: variavel do fonte, temporario criado aqui ou constante.
type OperandKind int

const (
	OpNone OperandKind = iota
	OpVar
	OpTemp
	OpConst
)

// endereco do TAC. Type e o tipo da linguagem (int, float, string, bool),
// usado pela geracao de codigo para escolher instrucao e registrador.
type Operand struct {
	Kind OperandKind
	// nome unico (x, x.1 no sombreamento, t1) ou o texto da constante.
	Name string
	Type string
	// variavel declarada no topo do arquivo: mora na memoria global.
	Global bool
}

func (o Operand) String() string { return o.Name }

func (o Operand) Valid() bool { return o.Kind != OpNone }

type InstrKind int

const (
	IBinary  InstrKind = iota // Dst = A Op B
	IUnary                    // Dst = Op A
	ICopy                     // Dst = A
	IConvert                  // Dst = (float) A
	ILabel                    // Label:
	IGoto                     // goto Label
	IIfFalse                  // ifFalse A goto Label, ou ifFalse A Op B goto Label
	IParam                    // param A
	ICall                     // Dst = call Label, N (Dst vazio em funcao void)
	IReturn                   // return A (A vazio em funcao void)
	IPrint                    // print A
	IPrintNL                  // println
	IRead                     // Dst = read
)

type Instr struct {
	Kind  InstrKind
	Op    string
	Dst   Operand
	A     Operand
	B     Operand
	Label string
	N     int
}

func (in Instr) String() string {
	switch in.Kind {
	case IBinary:
		return fmt.Sprintf("%s = %s %s %s", in.Dst, in.A, in.Op, in.B)
	case IUnary:
		return fmt.Sprintf("%s = %s %s", in.Dst, in.Op, in.A)
	case ICopy:
		return fmt.Sprintf("%s = %s", in.Dst, in.A)
	case IConvert:
		return fmt.Sprintf("%s = (float) %s", in.Dst, in.A)
	case ILabel:
		return in.Label + ":"
	case IGoto:
		return "goto " + in.Label
	case IIfFalse:
		if in.Op != "" {
			return fmt.Sprintf("ifFalse %s %s %s goto %s", in.A, in.Op, in.B, in.Label)
		}
		return fmt.Sprintf("ifFalse %s goto %s", in.A, in.Label)
	case IParam:
		return "param " + in.A.String()
	case ICall:
		if in.Dst.Valid() {
			return fmt.Sprintf("%s = call %s, %d", in.Dst, in.Label, in.N)
		}
		return fmt.Sprintf("call %s, %d", in.Label, in.N)
	case IReturn:
		if in.A.Valid() {
			return "return " + in.A.String()
		}
		return "return"
	case IPrint:
		return "print " + in.A.String()
	case IPrintNL:
		return "println"
	case IRead:
		return fmt.Sprintf("%s = read", in.Dst)
	}
	return "?"
}

// uma funcao do fonte, ou o corpo do programa (Main), que sao os comandos
// soltos no topo do arquivo.
type TACFunc struct {
	Name   string
	Main   bool
	Return string
	Params []Operand
	Code   []Instr
}

type TACProgram struct {
	// variaveis do topo do arquivo, na ordem de declaracao.
	Globals []Operand
	// Funcs[0] e sempre o corpo do programa.
	Funcs []*TACFunc
}

func (p *TACProgram) String() string {
	var b strings.Builder
	if len(p.Globals) > 0 {
		names := make([]string, len(p.Globals))
		for i, g := range p.Globals {
			names[i] = g.Type + " " + g.Name
		}
		fmt.Fprintf(&b, "globais: %s\n\n", strings.Join(names, ", "))
	}
	for i, f := range p.Funcs {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(f.String())
	}
	return b.String()
}

func (f *TACFunc) String() string {
	var b strings.Builder
	if f.Main {
		b.WriteString("programa:\n")
	} else {
		params := make([]string, len(f.Params))
		for i, p := range f.Params {
			params[i] = p.Type + " " + p.Name
		}
		fmt.Fprintf(&b, "func %s(%s) %s:\n", f.Name, strings.Join(params, ", "), f.Return)
	}
	for _, in := range f.Code {
		if in.Kind == ILabel {
			b.WriteString(in.String() + "\n")
			continue
		}
		b.WriteString("    " + in.String() + "\n")
	}
	if !f.Main {
		b.WriteString("endfunc\n")
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// Gerador: AST -> TAC
// ---------------------------------------------------------------------------

// o que um nome significa num escopo: variavel (com o nome unico do TAC) ou
// funcao (com a assinatura).
type tacSym struct {
	operand Operand
	// funcao dona da variavel. Global quando nil.
	owner *TACFunc
	fn    *tacFuncSig
}

type tacFuncSig struct {
	name   string // nome unico no TAC
	ret    string
	params []string
	decl   *FuncDecl
}

type tacScope struct {
	syms   map[string]tacSym
	parent *tacScope
}

func (s *tacScope) resolve(name string) (tacSym, bool) {
	for sc := s; sc != nil; sc = sc.parent {
		if sym, ok := sc.syms[name]; ok {
			return sym, true
		}
	}
	return tacSym{}, false
}

type tacGen struct {
	prog  *TACProgram
	fn    *TACFunc
	scope *tacScope
	// contadores globais: t1, t2... e L1, L2... nao se repetem no programa.
	temps  int
	labels int
	// quantas vezes cada nome ja foi usado, para x, x.1, x.2.
	versions map[string]int
	// rotulo de saida do laco mais interno, para o break.
	breaks []string
	errs   []string
}

// GenerateTAC traduz uma AST ja validada pela analise semantica. Os erros
// devolvidos sao so do que a geracao nao suporta (funcao aninhada lendo
// variavel local da funcao de fora).
func GenerateTAC(root *Program) (*TACProgram, []string) {
	g := &tacGen{
		prog:     &TACProgram{},
		scope:    &tacScope{syms: map[string]tacSym{}},
		versions: map[string]int{},
	}
	main := &TACFunc{Name: "programa", Main: true}
	g.prog.Funcs = append(g.prog.Funcs, main)
	g.fn = main

	g.declareFuncs(root.Decls)
	for _, d := range root.Decls {
		g.stmt(d)
	}
	return g.prog, g.errs
}

func (g *tacGen) errorAt(n Node, format string, args ...any) {
	line, col := posOf(n)
	g.errs = append(g.errs, fmt.Sprintf("linha %d, col %d: %s", line, col, fmt.Sprintf(format, args...)))
}

// nome unico: o primeiro x fica x, os seguintes viram x.1, x.2. O ponto nao
// existe em identificador da linguagem, entao nao colide.
func (g *tacGen) unique(name string) string {
	n := g.versions[name]
	g.versions[name] = n + 1
	if n == 0 {
		return name
	}
	return fmt.Sprintf("%s.%d", name, n)
}

func (g *tacGen) newTemp(typ string) Operand {
	g.temps++
	return Operand{Kind: OpTemp, Name: fmt.Sprintf("t%d", g.temps), Type: typ}
}

func (g *tacGen) newLabel() string {
	g.labels++
	return fmt.Sprintf("L%d", g.labels)
}

func (g *tacGen) emit(in Instr) { g.fn.Code = append(g.fn.Code, in) }

func (g *tacGen) label(l string) { g.emit(Instr{Kind: ILabel, Label: l}) }

func (g *tacGen) enter() { g.scope = &tacScope{syms: map[string]tacSym{}, parent: g.scope} }

func (g *tacGen) leave() { g.scope = g.scope.parent }

// assinaturas antes dos corpos, igual a analise semantica: recursao e
// chamada adiantada.
func (g *tacGen) declareFuncs(decls []Node) {
	for _, d := range decls {
		fn, ok := d.(*FuncDecl)
		if !ok {
			continue
		}
		ret := fn.Return
		if ret == "" {
			ret = TypeVoid
		}
		sig := &tacFuncSig{name: g.unique(fn.Name), ret: ret, decl: fn}
		for _, p := range fn.Params {
			sig.params = append(sig.params, p.Type)
		}
		g.scope.syms[fn.Name] = tacSym{fn: sig}
	}
}

// ---------------------------------------------------------------------------
// Funcoes
// ---------------------------------------------------------------------------

func (g *tacGen) function(sig *tacFuncSig) {
	fn := &TACFunc{Name: sig.name, Return: sig.ret}
	g.prog.Funcs = append(g.prog.Funcs, fn)

	prevFn, prevBreaks := g.fn, g.breaks
	g.fn, g.breaks = fn, nil
	g.enter()
	for _, p := range sig.decl.Params {
		op := Operand{Kind: OpVar, Name: g.unique(p.Name), Type: p.Type}
		fn.Params = append(fn.Params, op)
		g.scope.syms[p.Name] = tacSym{operand: op, owner: fn}
	}
	// corpo no mesmo escopo dos parametros, como na semantica.
	if sig.decl.Body != nil {
		g.declareFuncs(sig.decl.Body.Decls)
		for _, d := range sig.decl.Body.Decls {
			g.stmt(d)
		}
	}
	// funcao sem return no fim: o return implicito fecha o corpo.
	if len(fn.Code) == 0 || fn.Code[len(fn.Code)-1].Kind != IReturn {
		g.emit(Instr{Kind: IReturn})
	}
	g.leave()
	g.fn, g.breaks = prevFn, prevBreaks
}

// ---------------------------------------------------------------------------
// Comandos
// ---------------------------------------------------------------------------

func (g *tacGen) stmt(n Node) {
	switch n := n.(type) {
	case nil:
	case *FuncDecl:
		sym, _ := g.scope.resolve(n.Name)
		if sym.fn == nil || sym.fn.decl != n {
			return
		}
		// gerada agora, com o escopo atual (uma funcao aninhada enxerga os
		// nomes de fora), mas numa TACFunc propria: o codigo dela nao se
		// mistura com o de quem a contem.
		g.function(sym.fn)
	case *VarDecl:
		g.varDecl(n)
	case *Block:
		g.enter()
		g.declareFuncs(n.Decls)
		for _, d := range n.Decls {
			g.stmt(d)
		}
		g.leave()
	case *IfStmt:
		g.ifStmt(n)
	case *WhileStmt:
		start, end := g.newLabel(), g.newLabel()
		g.label(start)
		g.condJump(n.Cond, end)
		g.loopBody(n.Body, end)
		g.emit(Instr{Kind: IGoto, Label: start})
		g.label(end)
	case *ForStmt:
		g.forStmt(n)
	case *ReturnStmt:
		in := Instr{Kind: IReturn}
		if n.Value != nil {
			in.A = g.coerce(g.expr(n.Value), g.fn.Return)
		}
		g.emit(in)
	case *BreakStmt:
		if len(g.breaks) > 0 {
			g.emit(Instr{Kind: IGoto, Label: g.breaks[len(g.breaks)-1]})
		}
	case *PrintStmt:
		g.print(n.Args)
	case *InputStmt:
		g.read(n.Args)
	case *ExprStmt:
		g.expr(n.Expr)
	}
}

func (g *tacGen) varDecl(n *VarDecl) {
	op := Operand{Kind: OpVar, Name: g.unique(n.Name), Type: n.Type}
	// so a declaracao direta no topo do arquivo e global. Um bloco ou um for
	// no topo ainda pertence ao corpo do programa.
	if g.fn.Main && g.scope.parent == nil {
		op.Global = true
		g.prog.Globals = append(g.prog.Globals, op)
	}

	var value Operand
	if n.Init != nil {
		value = g.coerce(g.expr(n.Init), n.Type)
	} else {
		// sem inicializacao, comeca em zero: assim a variavel de um bloco
		// dentro de um laco nao herda o valor da volta anterior.
		value = zeroOf(n.Type)
	}
	// o nome so entra depois do valor: em "carteira int x = x + 1" o x da
	// direita e o de fora.
	g.scope.syms[n.Name] = tacSym{operand: op, owner: g.ownerFor(op)}
	g.emit(Instr{Kind: ICopy, Dst: op, A: value})
}

func (g *tacGen) ownerFor(op Operand) *TACFunc {
	if op.Global {
		return nil
	}
	return g.fn
}

func zeroOf(typ string) Operand {
	switch typ {
	case TypeFloat:
		return Operand{Kind: OpConst, Name: "0.0", Type: typ}
	case TypeString:
		return Operand{Kind: OpConst, Name: `""`, Type: typ}
	case TypeBool:
		return Operand{Kind: OpConst, Name: "false", Type: typ}
	}
	return Operand{Kind: OpConst, Name: "0", Type: TypeInt}
}

// if c1 {A} else if c2 {B} else {C}
//
//	    ifFalse c1 goto L1
//	    A
//	    goto Lfim
//	L1: ifFalse c2 goto L2
//	    B
//	    goto Lfim
//	L2: C
//	Lfim:
func (g *tacGen) ifStmt(n *IfStmt) {
	branches := append([]ElseIf{{Cond: n.Cond, Then: n.Then}}, n.ElseIfs...)
	end := ""
	for i, br := range branches {
		next := g.newLabel()
		g.condJump(br.Cond, next)
		g.stmt(br.Then)
		last := i == len(branches)-1 && n.Else == nil
		if !last {
			if end == "" {
				end = g.newLabel()
			}
			g.emit(Instr{Kind: IGoto, Label: end})
		}
		g.label(next)
	}
	if n.Else != nil {
		g.stmt(n.Else)
	}
	if end != "" {
		g.label(end)
	}
}

// for (init; cond; post) body
//
//	    init
//	L1: ifFalse cond goto L2
//	    body
//	    post
//	    goto L1
//	L2:
func (g *tacGen) forStmt(n *ForStmt) {
	g.enter()
	g.stmt(n.Init)
	start, end := g.newLabel(), g.newLabel()
	g.label(start)
	if n.Cond != nil {
		g.condJump(n.Cond, end)
	}
	g.loopBody(n.Body, end)
	if n.Post != nil {
		g.expr(n.Post)
	}
	g.emit(Instr{Kind: IGoto, Label: start})
	g.label(end)
	g.leave()
}

func (g *tacGen) loopBody(body Node, end string) {
	g.breaks = append(g.breaks, end)
	g.stmt(body)
	g.breaks = g.breaks[:len(g.breaks)-1]
}

// desvia para label quando cond e falsa. Uma comparacao entra direto no
// ifFalse ("ifFalse a < b goto L"), sem passar por um temporario bool.
func (g *tacGen) condJump(cond Node, label string) {
	if bin, ok := cond.(*Binary); ok && isRelational(bin.Op) {
		a, b := g.operands(bin)
		g.emit(Instr{Kind: IIfFalse, Op: bin.Op, A: a, B: b, Label: label})
		return
	}
	g.emit(Instr{Kind: IIfFalse, A: g.expr(cond), Label: label})
}

// cada argumento e impresso seguido de um espaco; o ultimo, de quebra de linha.
func (g *tacGen) print(args []Node) {
	values := make([]Operand, len(args))
	for i, a := range args {
		values[i] = g.expr(a)
	}
	for i, v := range values {
		if i > 0 {
			g.emit(Instr{Kind: IPrint, A: Operand{Kind: OpConst, Name: `" "`, Type: TypeString}})
		}
		g.emit(Instr{Kind: IPrint, A: v})
	}
	g.emit(Instr{Kind: IPrintNL})
}

// os argumentos de reclamacao(...) sao o prompt, impresso antes da leitura.
func (g *tacGen) read(args []Node) Operand {
	for _, a := range args {
		g.emit(Instr{Kind: IPrint, A: g.expr(a)})
	}
	t := g.newTemp(TypeString)
	g.emit(Instr{Kind: IRead, Dst: t})
	return t
}

// ---------------------------------------------------------------------------
// Expressoes
// ---------------------------------------------------------------------------

// expr devolve o endereco que guarda o valor da expressao. Constante e
// variavel nao geram codigo; operador gera uma instrucao com destino novo.
func (g *tacGen) expr(n Node) Operand {
	switch n := n.(type) {
	case *Literal:
		return literal(n)
	case *Ident:
		return g.variable(n, n.Name)
	case *Assign:
		target := g.variable(n, n.Name)
		value := g.coerce(g.expr(n.Value), target.Type)
		g.emit(Instr{Kind: ICopy, Dst: target, A: value})
		return target
	case *Unary:
		a := g.expr(n.Expr)
		t := g.newTemp(a.Type)
		g.emit(Instr{Kind: IUnary, Op: n.Op, Dst: t, A: a})
		return t
	case *Binary:
		a, b := g.operands(n)
		typ := a.Type
		if isRelational(n.Op) {
			typ = TypeBool
		}
		t := g.newTemp(typ)
		g.emit(Instr{Kind: IBinary, Op: n.Op, Dst: t, A: a, B: b})
		return t
	case *Call:
		return g.call(n)
	}
	return Operand{}
}

func literal(n *Literal) Operand {
	return Operand{Kind: OpConst, Name: n.Value, Type: n.Kind}
}

func (g *tacGen) variable(n Node, name string) Operand {
	sym, ok := g.scope.resolve(name)
	if !ok || sym.fn != nil {
		// a semantica ja barrou; so por seguranca.
		g.errorAt(n, "'%s' nao e uma variavel", name)
		return Operand{Kind: OpVar, Name: name, Type: TypeInt}
	}
	if sym.owner != nil && sym.owner != g.fn {
		g.errorAt(n, "funcao aninhada nao pode usar '%s', variavel local de '%s'", name, sym.owner.Name)
	}
	return sym.operand
}

// os dois lados de um operador binario, ja no mesmo tipo: num int com float,
// o int ganha uma conversao explicita antes da operacao.
func (g *tacGen) operands(n *Binary) (Operand, Operand) {
	a := g.expr(n.Left)
	b := g.expr(n.Right)
	if a.Type == TypeFloat || b.Type == TypeFloat {
		a = g.coerce(a, TypeFloat)
		b = g.coerce(b, TypeFloat)
	}
	return a, b
}

// int onde se espera float vira float. Constante converte na hora;
// variavel e temporario ganham a instrucao "(float)".
func (g *tacGen) coerce(v Operand, want string) Operand {
	if want != TypeFloat || v.Type != TypeInt {
		return v
	}
	if v.Kind == OpConst {
		f, _ := strconv.ParseFloat(v.Name, 64)
		return Operand{Kind: OpConst, Name: strconv.FormatFloat(f, 'f', 1, 64), Type: TypeFloat}
	}
	t := g.newTemp(TypeFloat)
	g.emit(Instr{Kind: IConvert, Dst: t, A: v})
	return t
}

// Os argumentos sao todos avaliados antes do primeiro param: uma chamada
// dentro de outro argumento nao pode intercalar os seus param com os de fora.
func (g *tacGen) call(n *Call) Operand {
	if isPrintName(n.Name) {
		g.print(n.Args)
		return Operand{}
	}
	if isInputName(n.Name) {
		return g.read(n.Args)
	}

	sym, ok := g.scope.resolve(n.Name)
	if !ok || sym.fn == nil {
		g.errorAt(n, "'%s' nao e uma funcao", n.Name)
		return Operand{}
	}
	sig := sym.fn
	args := make([]Operand, len(n.Args))
	for i, a := range n.Args {
		args[i] = g.expr(a)
		if i < len(sig.params) {
			args[i] = g.coerce(args[i], sig.params[i])
		}
	}
	for _, a := range args {
		g.emit(Instr{Kind: IParam, A: a})
	}

	in := Instr{Kind: ICall, Label: sig.name, N: len(args)}
	if sig.ret != TypeVoid {
		in.Dst = g.newTemp(sig.ret)
	}
	g.emit(in)
	return in.Dst
}

func isRelational(op string) bool {
	switch op {
	case "<", ">", "<=", ">=", "==", "!=":
		return true
	}
	return false
}
