package main

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
)

const (
	TypeInt    = "int"
	TypeString = "string"
	TypeFloat  = "float"
	TypeBool   = "bool"
	TypeVoid   = "void"
	// expressao que ja falhou. Quem esta acima nao gera outro erro de tipo.
	TypeError = "error"
)

// variavel, parametro e funcao dividem o nome. Params so na funcao.
type Symbol struct {
	Name string
	// "var", "param" ou "func".
	Kind   string
	Type   string
	Params []string
	// rotulo do escopo onde foi declarado: global, local (f), aninhado (f).
	Scope  string
	Line   int
	Column int
}

// parent e o escopo de fora. Duplicata so no mapa local.
type SymbolTable struct {
	symbols map[string]Symbol
	parent  *SymbolTable
	scope   string
}

func newSymbolTable(parent *SymbolTable, scope string) *SymbolTable {
	return &SymbolTable{
		symbols: make(map[string]Symbol),
		parent:  parent,
		scope:   scope,
	}
}

func (st *SymbolTable) Define(name string, sym Symbol) {
	st.symbols[name] = sym
}

func (st *SymbolTable) Resolve(name string) (Symbol, bool) {
	if sym, ok := st.symbols[name]; ok {
		return sym, true
	}
	if st.parent != nil {
		return st.parent.Resolve(name)
	}
	return Symbol{}, false
}

type SemanticAnalyzer struct {
	current    *SymbolTable
	returnType string
	// funcao em analise, para rotular os escopos de dentro dela.
	funcName string
	// break so vale dentro de for/while.
	loopDepth int
	errs      []semErr
	// todo simbolo inserido, inclusive os de escopos ja fechados.
	declared []Symbol
}

// posicao fora do texto, para ordenar a saida por linha.
type semErr struct {
	line int
	col  int
	text string
}

func newSemanticAnalyzer() *SemanticAnalyzer {
	return &SemanticAnalyzer{current: newSymbolTable(nil, "global")}
}

// devolve a tabela (ordem do fonte) e os erros (ordem de linha).
func Analyze(root Node) ([]Symbol, []string) {
	sa := newSemanticAnalyzer()
	sa.analyze(root)
	// funcao entra antes do resto do escopo; a tabela volta para a ordem do fonte.
	sort.SliceStable(sa.declared, func(i, j int) bool {
		if sa.declared[i].Line != sa.declared[j].Line {
			return sa.declared[i].Line < sa.declared[j].Line
		}
		return sa.declared[i].Column < sa.declared[j].Column
	})
	sort.SliceStable(sa.errs, func(i, j int) bool {
		if sa.errs[i].line != sa.errs[j].line {
			return sa.errs[i].line < sa.errs[j].line
		}
		return sa.errs[i].col < sa.errs[j].col
	})
	out := make([]string, len(sa.errs))
	for i, e := range sa.errs {
		if e.line > 0 {
			out[i] = fmt.Sprintf("linha %d, col %d: %s", e.line, e.col, e.text)
			continue
		}
		out[i] = e.text
	}
	return sa.declared, out
}

// uma linha por simbolo: nome, categoria, tipo, escopo e posicao.
func FormatSymbols(symbols []Symbol) string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NOME\tCATEGORIA\tTIPO\tESCOPO\tPOSICAO")
	for _, s := range symbols {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d:%d\n", s.Name, kindLabel(s.Kind), typeLabel(s), s.Scope, s.Line, s.Column)
	}
	w.Flush()
	return b.String()
}

func kindLabel(kind string) string {
	switch kind {
	case "func":
		return "funcao"
	case "param":
		return "parametro"
	}
	return "variavel"
}

// funcao mostra a assinatura: int(int, float).
func typeLabel(s Symbol) string {
	if s.Kind != "func" {
		return s.Type
	}
	return s.Type + "(" + strings.Join(s.Params, ", ") + ")"
}

func (sa *SemanticAnalyzer) errorAt(line, col int, format string, args ...any) {
	sa.errs = append(sa.errs, semErr{line: line, col: col, text: fmt.Sprintf(format, args...)})
}

// insere no escopo atual e guarda uma copia para a tabela impressa.
func (sa *SemanticAnalyzer) define(sym Symbol) {
	sym.Scope = sa.current.scope
	sa.current.Define(sym.Name, sym)
	sa.declared = append(sa.declared, sym)
}

// mensagem de duplicata aponta onde o nome ja estava.
func (sa *SemanticAnalyzer) duplicate(line, col int, prev Symbol, format string, name string) {
	sa.errorAt(line, col, format+" (linha %d)", name, prev.Line)
}

func (sa *SemanticAnalyzer) enter(scope string) {
	sa.current = newSymbolTable(sa.current, scope)
}

// bloco, for: aninhado dentro da funcao atual, ou do global.
func (sa *SemanticAnalyzer) nestedScope() string {
	if sa.funcName == "" {
		return "aninhado"
	}
	return "aninhado (" + sa.funcName + ")"
}

func (sa *SemanticAnalyzer) leave() {
	if sa.current != nil && sa.current.parent != nil {
		sa.current = sa.current.parent
	}
}

// assinatura antes do corpo, para recursao e chamada adiantada.
func (sa *SemanticAnalyzer) declareFuncs(decls []Node) {
	for _, d := range decls {
		fn, ok := d.(*FuncDecl)
		if !ok || fn.Name == "" {
			continue
		}
		if prev, exists := sa.current.symbols[fn.Name]; exists {
			sa.duplicate(fn.Line, fn.Column, prev, "'%s' ja foi declarado neste escopo", fn.Name)
			continue
		}
		ret := fn.Return
		if ret == "" {
			ret = TypeVoid
		}
		params := make([]string, len(fn.Params))
		for i, p := range fn.Params {
			params[i] = TypeError
			if p != nil && p.Type != "" && p.Type != TypeVoid {
				params[i] = p.Type
			}
		}
		sa.define(Symbol{Name: fn.Name, Kind: "func", Type: ret, Params: params, Line: fn.Line, Column: fn.Column})
	}
}

func (sa *SemanticAnalyzer) analyze(n Node) string {
	if n == nil {
		return TypeVoid
	}
	switch n := n.(type) {
	case *Program:
		sa.declareFuncs(n.Decls)
		for _, d := range n.Decls {
			sa.analyze(d)
		}
	case *Block:
		sa.enter(sa.nestedScope())
		sa.declareFuncs(n.Decls)
		for _, d := range n.Decls {
			sa.analyze(d)
		}
		sa.leave()
	case *FuncDecl:
		sa.analyzeFunc(n)
	case *VarDecl:
		sa.analyzeVar(n)
	case *IfStmt:
		sa.expectBool(n.Cond, "if")
		sa.analyze(n.Then)
		for _, e := range n.ElseIfs {
			sa.expectBool(e.Cond, "else if")
			sa.analyze(e.Then)
		}
		sa.analyze(n.Else)
	case *ForStmt:
		// a variavel do init morre no fim do for.
		sa.enter(sa.nestedScope())
		sa.analyze(n.Init)
		if n.Cond != nil {
			sa.expectBool(n.Cond, "for")
		}
		sa.analyze(n.Post)
		sa.loopDepth++
		sa.analyze(n.Body)
		sa.loopDepth--
		sa.leave()
	case *WhileStmt:
		sa.expectBool(n.Cond, "while")
		sa.loopDepth++
		sa.analyze(n.Body)
		sa.loopDepth--
	case *ReturnStmt:
		sa.analyzeReturn(n)
	case *BreakStmt:
		if sa.loopDepth == 0 {
			sa.errorAt(n.Line, n.Column, "break fora de laco")
		}
	case *PrintStmt:
		sa.valueList(n.Args)
	case *InputStmt:
		sa.valueList(n.Args)
	case *ExprStmt:
		sa.analyze(n.Expr)
	case *Assign:
		return sa.analyzeAssign(n)
	case *Binary:
		return sa.analyzeBinary(n)
	case *Unary:
		return sa.analyzeUnary(n)
	case *Call:
		return sa.analyzeCall(n)
	case *Ident:
		return sa.analyzeIdent(n)
	case *Literal:
		if n.Kind == "" {
			return TypeError
		}
		return n.Kind
	}
	return TypeVoid
}

func (sa *SemanticAnalyzer) analyzeFunc(n *FuncDecl) {
	ret := n.Return
	if ret == "" {
		ret = TypeVoid
	}
	prevRet, prevName := sa.returnType, sa.funcName
	sa.returnType, sa.funcName = ret, n.Name
	sa.enter("local (" + n.Name + ")")
	for _, p := range n.Params {
		sa.defineParam(p)
	}
	// mesmo escopo dos parametros, senao o nome repetido sombreia.
	if n.Body != nil {
		sa.declareFuncs(n.Body.Decls)
		for _, d := range n.Body.Decls {
			sa.analyze(d)
		}
	}
	sa.leave()
	sa.returnType, sa.funcName = prevRet, prevName
}

func (sa *SemanticAnalyzer) defineParam(p *Param) {
	if p == nil || p.Name == "" {
		return
	}
	if prev, exists := sa.current.symbols[p.Name]; exists {
		sa.duplicate(p.Line, p.Column, prev, "parametro '%s' repetido", p.Name)
		return
	}
	typ := p.Type
	if typ == "" || typ == TypeVoid {
		sa.errorAt(p.Line, p.Column, "parametro '%s' nao pode ser void", p.Name)
		typ = TypeError
	}
	sa.define(Symbol{Name: p.Name, Kind: "param", Type: typ, Line: p.Line, Column: p.Column})
}

func (sa *SemanticAnalyzer) analyzeVar(n *VarDecl) {
	var initType string
	if n.Init != nil {
		initType = sa.asValue(n.Init)
	}
	if n.Name == "" {
		return
	}
	if prev, exists := sa.current.symbols[n.Name]; exists {
		sa.duplicate(n.Line, n.Column, prev, "'%s' ja foi declarado neste escopo", n.Name)
		return
	}
	sym := Symbol{Name: n.Name, Kind: "var", Type: n.Type, Line: n.Line, Column: n.Column}
	if sym.Type == "" || sym.Type == TypeVoid {
		sa.errorAt(n.Line, n.Column, "variavel '%s' nao pode ser void", n.Name)
		sym.Type = TypeError
		sa.define(sym)
		return
	}
	if n.Init != nil && !assignable(sym.Type, initType) {
		sa.errorAt(n.Line, n.Column, "tipos incompativeis na declaracao de '%s': esperado %s, encontrado %s", n.Name, sym.Type, initType)
	}
	sa.define(sym)
}

func (sa *SemanticAnalyzer) analyzeReturn(n *ReturnStmt) {
	if sa.returnType == "" {
		sa.errorAt(n.Line, n.Column, "return fora de funcao")
		if n.Value != nil {
			sa.asValue(n.Value)
		}
		return
	}
	if sa.returnType == TypeVoid {
		if n.Value != nil {
			sa.errorAt(n.Line, n.Column, "return com valor em funcao void")
			sa.asValue(n.Value)
		}
		return
	}
	if n.Value == nil {
		sa.errorAt(n.Line, n.Column, "return sem valor em funcao %s", sa.returnType)
		return
	}
	got := sa.asValue(n.Value)
	if !assignable(sa.returnType, got) {
		sa.errorAt(n.Line, n.Column, "return incompativel: esperado %s, encontrado %s", sa.returnType, got)
	}
}

func (sa *SemanticAnalyzer) analyzeAssign(n *Assign) string {
	valType := TypeError
	if n.Value != nil {
		valType = sa.asValue(n.Value)
	}
	sym, ok := sa.current.Resolve(n.Name)
	if !ok {
		sa.errorAt(n.Line, n.Column, "variavel '%s' nao declarada", n.Name)
		return TypeError
	}
	if sym.Kind == "func" {
		sa.errorAt(n.Line, n.Column, "'%s' e uma funcao; chame com '%s(...)'", n.Name, n.Name)
		return TypeError
	}
	if !assignable(sym.Type, valType) {
		sa.errorAt(n.Line, n.Column, "tipos incompativeis na atribuicao de '%s': esperado %s, encontrado %s", n.Name, sym.Type, valType)
	}
	return sym.Type
}

func (sa *SemanticAnalyzer) analyzeIdent(n *Ident) string {
	sym, ok := sa.current.Resolve(n.Name)
	if !ok {
		sa.errorAt(n.Line, n.Column, "variavel '%s' nao declarada", n.Name)
		return TypeError
	}
	if sym.Kind == "func" {
		sa.errorAt(n.Line, n.Column, "'%s' e uma funcao; chame com '%s(...)'", n.Name, n.Name)
		return TypeError
	}
	return sym.Type
}

func (sa *SemanticAnalyzer) analyzeUnary(n *Unary) string {
	t := sa.asValue(n.Expr)
	if t == TypeError {
		return TypeError
	}
	if !isNumeric(t) {
		sa.errorAt(n.Line, n.Column, "operador '%s' invalido para %s", n.Op, t)
		return TypeError
	}
	return t
}

func (sa *SemanticAnalyzer) analyzeBinary(n *Binary) string {
	left := sa.asValue(n.Left)
	right := sa.asValue(n.Right)
	if left == TypeError || right == TypeError {
		return TypeError
	}
	switch n.Op {
	case "+", "-", "*", "/":
		if bothNumeric(left, right) {
			return numericResult(left, right)
		}
	case "==", "!=":
		if left == right || bothNumeric(left, right) {
			return TypeBool
		}
	case "<", ">", "<=", ">=":
		if bothNumeric(left, right) {
			return TypeBool
		}
	default:
		sa.errorAt(n.Line, n.Column, "operador '%s' desconhecido", n.Op)
		return TypeError
	}
	sa.errorAt(n.Line, n.Column, "operacao '%s' invalida entre %s e %s", n.Op, left, right)
	return TypeError
}

func (sa *SemanticAnalyzer) analyzeCall(n *Call) string {
	// reservada, nao entra na tabela. input devolve string.
	if isPrintName(n.Name) {
		sa.valueList(n.Args)
		return TypeVoid
	}
	if isInputName(n.Name) {
		sa.valueList(n.Args)
		return TypeString
	}
	sym, ok := sa.current.Resolve(n.Name)
	if !ok {
		sa.errorAt(n.Line, n.Column, "funcao '%s' nao declarada", n.Name)
		sa.valueList(n.Args)
		return TypeError
	}
	if sym.Kind != "func" {
		sa.errorAt(n.Line, n.Column, "'%s' nao e uma funcao", n.Name)
		sa.valueList(n.Args)
		return TypeError
	}
	for i, arg := range n.Args {
		got := sa.asValue(arg)
		if i < len(sym.Params) && !assignable(sym.Params[i], got) {
			sa.errorAt(n.Line, n.Column, "chamada de '%s': argumento %d esperado %s, encontrado %s", n.Name, i+1, sym.Params[i], got)
		}
	}
	if len(n.Args) != len(sym.Params) {
		sa.errorAt(n.Line, n.Column, "chamada de '%s': esperado %d argumento(s), encontrado %d", n.Name, len(sym.Params), len(n.Args))
	}
	return sym.Type
}

// void lido como valor. O comando "avisar();" nao passa aqui.
func (sa *SemanticAnalyzer) asValue(n Node) string {
	if n == nil {
		return TypeError
	}
	t := sa.analyze(n)
	if t == TypeVoid {
		line, col := posOf(n)
		sa.errorAt(line, col, "expressao void nao pode ser usada como valor")
		return TypeError
	}
	return t
}

func (sa *SemanticAnalyzer) valueList(args []Node) {
	for _, a := range args {
		sa.asValue(a)
	}
}

func (sa *SemanticAnalyzer) expectBool(n Node, where string) {
	if n == nil {
		return
	}
	t := sa.asValue(n)
	if t == TypeError || t == TypeBool {
		return
	}
	line, col := posOf(n)
	sa.errorAt(line, col, "condicao do %s deve ser bool, encontrado %s", where, t)
}

func posOf(n Node) (int, int) {
	if n == nil {
		return 0, 0
	}
	if p, ok := n.(interface{ Position() (int, int) }); ok {
		return p.Position()
	}
	return 0, 0
}

// int cabe em float. Float em int continua erro.
func assignable(dst, src string) bool {
	if dst == TypeError || src == TypeError {
		return true
	}
	if dst == src {
		return true
	}
	return dst == TypeFloat && src == TypeInt
}

func isNumeric(t string) bool {
	return t == TypeInt || t == TypeFloat
}

func bothNumeric(a, b string) bool {
	return isNumeric(a) && isNumeric(b)
}

func numericResult(a, b string) string {
	if a == TypeFloat || b == TypeFloat {
		return TypeFloat
	}
	return TypeInt
}

func isPrintName(name string) bool {
	return name == "print" || name == "bater_ponto"
}

func isInputName(name string) bool {
	return name == "input" || name == "reclamacao"
}
