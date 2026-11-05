package main

import (
	"fmt"
	"strings"
)

type Node interface {
	Pretty(indent string) string
}

// linha e coluna do token, para o erro semantico apontar o lugar.
type Pos struct {
	Line   int
	Column int
}

func (p Pos) Position() (int, int) { return p.Line, p.Column }

// raiz do arquivo. Funcao, variavel e comando no mesmo nivel.
type Program struct {
	Decls []Node
}

func (n *Program) Pretty(indent string) string {
	return head(indent, "program", n.Decls...)
}

// as duas formas de funcao preenchem os mesmos campos.
type FuncDecl struct {
	Pos
	Name   string
	Params []*Param
	// concept do tipo. Vazio se contratar nao trouxe tipo.
	Return string
	Body   *Block
}

func (n *FuncDecl) Pretty(indent string) string {
	title := "func " + n.Name
	if n.Return != "" {
		title += " " + n.Return
	}
	children := make([]Node, 0, len(n.Params)+1)
	for _, p := range n.Params {
		children = append(children, p)
	}
	if n.Body != nil {
		children = append(children, n.Body)
	}
	return head(indent, title, children...)
}

type Param struct {
	Pos
	Type string
	Name string
}

func (n *Param) Pretty(indent string) string {
	return line(indent, "param "+n.Type+" "+n.Name)
}

type VarDecl struct {
	Pos
	Type string
	Name string
	// nil quando nao ha "=".
	Init Node
}

func (n *VarDecl) Pretty(indent string) string {
	title := "var " + n.Type + " " + n.Name
	if n.Init == nil {
		return line(indent, title)
	}
	return head(indent, title, n.Init)
}

// aceita funcao, variavel e comando, igual ao arquivo.
type Block struct {
	Decls []Node
}

func (n *Block) Pretty(indent string) string {
	return head(indent, "block", n.Decls...)
}

type IfStmt struct {
	Cond Node
	Then Node
	// else if ja chegou como um token so.
	ElseIfs []ElseIf
	Else    Node
}

type ElseIf struct {
	Cond Node
	Then Node
}

func (n *IfStmt) Pretty(indent string) string {
	var b strings.Builder
	b.WriteString(line(indent, "if"))
	b.WriteString(head(indent+"  ", "cond", n.Cond))
	b.WriteString(head(indent+"  ", "then", n.Then))
	for _, e := range n.ElseIfs {
		b.WriteString(line(indent+"  ", "else if"))
		b.WriteString(head(indent+"    ", "cond", e.Cond))
		b.WriteString(head(indent+"    ", "then", e.Then))
	}
	if n.Else != nil {
		b.WriteString(head(indent+"  ", "else", n.Else))
	}
	return b.String()
}

// init, cond e post podem faltar.
type ForStmt struct {
	Init Node
	Cond Node
	Post Node
	Body Node
}

func (n *ForStmt) Pretty(indent string) string {
	var b strings.Builder
	b.WriteString(line(indent, "for"))
	if n.Init != nil {
		b.WriteString(head(indent+"  ", "init", n.Init))
	}
	if n.Cond != nil {
		b.WriteString(head(indent+"  ", "cond", n.Cond))
	}
	if n.Post != nil {
		b.WriteString(head(indent+"  ", "post", n.Post))
	}
	b.WriteString(head(indent+"  ", "body", n.Body))
	return b.String()
}

type WhileStmt struct {
	Cond Node
	Body Node
}

func (n *WhileStmt) Pretty(indent string) string {
	var b strings.Builder
	b.WriteString(line(indent, "while"))
	b.WriteString(head(indent+"  ", "cond", n.Cond))
	b.WriteString(head(indent+"  ", "body", n.Body))
	return b.String()
}

// value nil e o retorno vazio.
type ReturnStmt struct {
	Pos
	Value Node
}

func (n *ReturnStmt) Pretty(indent string) string {
	if n.Value == nil {
		return line(indent, "return")
	}
	return head(indent, "return", n.Value)
}

// comando. No meio de uma expressao vira Call.
type PrintStmt struct {
	Args []Node
}

func (n *PrintStmt) Pretty(indent string) string {
	return head(indent, "print", n.Args...)
}

// comando. "nome = reclamacao()" e Call.
type InputStmt struct {
	Args []Node
}

func (n *InputStmt) Pretty(indent string) string {
	return head(indent, "input", n.Args...)
}

type BreakStmt struct {
	Pos
}

func (n *BreakStmt) Pretty(indent string) string {
	return line(indent, "break")
}

// pretty mostra o filho direto.
type ExprStmt struct {
	Expr Node
}

func (n *ExprStmt) Pretty(indent string) string {
	if n.Expr == nil {
		return line(indent, "expr")
	}
	return n.Expr.Pretty(indent)
}

// associativo a direita: a = b = 1.
type Assign struct {
	Pos
	Name  string
	Value Node
}

func (n *Assign) Pretty(indent string) string {
	return head(indent, "=", &Ident{Name: n.Name}, n.Value)
}

// op e o lexema ("+", "=="). Associativo a esquerda.
type Binary struct {
	Pos
	Op    string
	Left  Node
	Right Node
}

func (n *Binary) Pretty(indent string) string {
	return head(indent, n.Op, n.Left, n.Right)
}

// menos prefixo. "-1 * 2" nao e um literal negativo.
type Unary struct {
	Pos
	Op   string
	Expr Node
}

func (n *Unary) Pretty(indent string) string {
	return head(indent, n.Op, n.Expr)
}

type Call struct {
	Pos
	// lexema do fonte, nao o concept.
	Name string
	Args []Node
}

func (n *Call) Pretty(indent string) string {
	return head(indent, "call "+n.Name, n.Args...)
}

type Ident struct {
	Pos
	Name string
}

func (n *Ident) Pretty(indent string) string {
	return line(indent, "ident "+n.Name)
}

type Literal struct {
	Pos
	Kind string
	// no bool e true/false, nao registrado/pj.
	Value string
}

func (n *Literal) Pretty(indent string) string {
	return line(indent, n.Kind+" "+n.Value)
}

func line(indent, text string) string {
	return indent + text + "\n"
}

// filho nil vira "?", para o buraco do erro nao sumir.
func head(indent, title string, children ...Node) string {
	var b strings.Builder
	b.WriteString(line(indent, title))
	childIndent := indent + "  "
	for _, c := range children {
		if c == nil {
			b.WriteString(line(childIndent, "?"))
			continue
		}
		b.WriteString(c.Pretty(childIndent))
	}
	return b.String()
}

type Parser struct {
	tokens []Token
	pos    int
	errors []string
	// cala os erros seguintes no mesmo ponto.
	panicMode bool
}

// devolve a arvore mesmo com erro.
func Parse(tokens []Token) (*Program, []string) {
	p := &Parser{tokens: tokens}
	return p.parseProgram(), p.errors
}

// comentario fica fora da gramatica.
func (p *Parser) skipComments() {
	for p.pos < len(p.tokens) && p.tokens[p.pos].Type == "COMMENT" {
		p.pos++
	}
}

// type "" e fim da entrada. Nao ha token EOF.
func (p *Parser) peek() Token {
	p.skipComments()
	if p.pos >= len(p.tokens) {
		return Token{}
	}
	return p.tokens[p.pos]
}

// proximo token, sem consumir. Pula comentario.
func (p *Parser) peekNext() Token {
	p.skipComments()
	passedCurrent := false
	for i := p.pos; i < len(p.tokens); i++ {
		if p.tokens[i].Type == "COMMENT" {
			continue
		}
		if !passedCurrent {
			passedCurrent = true
			continue
		}
		return p.tokens[i]
	}
	return Token{}
}

// no fim da entrada pos nao anda.
func (p *Parser) next() Token {
	t := p.peek()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return t
}

func (p *Parser) checkDelim(value string) bool {
	t := p.peek()
	return t.Type == "DELIMITER" && t.Value == value
}

func (p *Parser) matchDelim(value string) bool {
	if p.checkDelim(value) {
		p.next()
		return true
	}
	return false
}

func (p *Parser) expectDelim(value, message string) bool {
	if p.matchDelim(value) {
		return true
	}
	p.errorAt(p.peek(), message)
	return false
}

func (p *Parser) checkOp(value string) bool {
	t := p.peek()
	return t.Type == "OPERATOR" && t.Value == value
}

func (p *Parser) matchOp(values ...string) (op string, line, col int, ok bool) {
	t := p.peek()
	if t.Type != "OPERATOR" {
		return "", 0, 0, false
	}
	for _, v := range values {
		if t.Value == v {
			p.next()
			return v, t.Line, t.Column, true
		}
	}
	return "", 0, 0, false
}

// pelo concept (if, var), nao pelo lexema.
func (p *Parser) checkConcept(concept string) bool {
	t := p.peek()
	return t.Type == "KEYWORD" && t.Concept == concept
}

func (p *Parser) matchConcept(concept string) bool {
	if p.checkConcept(concept) {
		p.next()
		return true
	}
	return false
}

func (p *Parser) lastPos() (line, col int) {
	for i := len(p.tokens) - 1; i >= 0; i-- {
		if p.tokens[i].Type != "COMMENT" {
			return p.tokens[i].Line, p.tokens[i].Column
		}
	}
	return 1, 1
}

// copia a mensagem do lexer. Nao liga panicMode.
func (p *Parser) recordLexical(t Token) {
	msg := t.Message
	if msg == "" {
		msg = "erro lexico"
	}
	p.errors = append(p.errors, fmt.Sprintf("linha %d, col %d: %s", t.Line, t.Column, msg))
}

// liga panicMode. Token ERROR reusa a mensagem do lexer.
func (p *Parser) errorAt(t Token, message string) {
	if p.panicMode {
		return
	}
	p.panicMode = true
	if t.Type == "ERROR" {
		p.recordLexical(t)
		p.next()
		return
	}
	line, col := t.Line, t.Column
	if t.Type == "" {
		line, col = p.lastPos()
		message += " (fim da entrada)"
	} else {
		message = fmt.Sprintf("%s, encontrado %q", message, t.Value)
	}
	p.errors = append(p.errors, fmt.Sprintf("linha %d, col %d: %s", line, col, message))
}

// nome so conta se for comando novo (a =, foo(), a;).
func (p *Parser) startsDecl(t Token) bool {
	if t.Type == "TYPE" {
		return true
	}
	if t.Type == "KEYWORD" {
		switch t.Concept {
		case "function", "var", "if", "for", "while", "return", "print", "input", "break":
			return true
		}
		return false
	}
	if t.Type != "IDENTIFIER" {
		return false
	}
	next := p.peekNext()
	if next.Type == "OPERATOR" && next.Value == "=" {
		return true
	}
	if next.Type == "DELIMITER" && next.Value == "(" {
		return true
	}
	if next.Type == "DELIMITER" && next.Value == ";" {
		return true
	}
	return false
}

// anda ate ";", "{" ou "}" e solta o panicMode.
func (p *Parser) synchronize() {
	p.panicMode = false
	for {
		t := p.peek()
		if t.Type == "" {
			return
		}
		if t.Type == "ERROR" {
			p.recordLexical(t)
			p.next()
			continue
		}
		if p.checkDelim(";") {
			p.next()
			return
		}
		if p.checkDelim("}") || p.checkDelim("{") {
			return
		}
		if p.startsDecl(t) {
			return
		}
		p.next()
	}
}

// "}" solto e erro. Se nada consumir, anda um token.
func (p *Parser) parseProgram() *Program {
	var decls []Node
	for p.peek().Type != "" {
		if p.checkDelim("}") {
			p.errorAt(p.peek(), "'}' inesperado")
			p.next()
			p.panicMode = false
			continue
		}
		before := p.pos
		if d := p.parseDecl(); d != nil {
			decls = append(decls, d)
		}
		if p.pos == before {
			p.next()
		}
	}
	return &Program{Decls: decls}
}

func (p *Parser) parseDecl() Node {
	if p.peek().Type == "ERROR" {
		p.recordLexical(p.peek())
		p.next()
		p.synchronize()
		return nil
	}
	var n Node
	switch {
	case p.checkConcept("function") || p.peek().Type == "TYPE":
		n = p.parseFuncDecl()
	case p.checkConcept("var"):
		n = p.parseVarDecl()
	default:
		n = p.parseStmt()
	}
	if p.panicMode {
		p.synchronize()
	}
	return n
}

func (p *Parser) parseFuncDecl() Node {
	var ret string
	// contratar: o tipo vem depois do ")".
	returnAfterParams := false
	if p.checkConcept("function") {
		p.next()
		returnAfterParams = true
	} else {
		ret = p.parseType()
	}

	if p.peek().Type != "IDENTIFIER" {
		p.errorAt(p.peek(), "esperado nome de funcao")
		return &FuncDecl{Return: ret}
	}
	nameTok := p.next()
	name := nameTok.Value

	p.expectDelim("(", "esperado '(' apos o nome da funcao")
	var params []*Param
	if !p.checkDelim(")") && p.peek().Type != "" && !p.checkDelim("{") {
		params = p.parseParams()
	}
	p.expectDelim(")", "esperado ')' apos os parametros")
	if returnAfterParams && p.peek().Type == "TYPE" {
		ret = p.parseType()
	}
	body := p.parseBlock()
	return &FuncDecl{Pos: Pos{Line: nameTok.Line, Column: nameTok.Column}, Name: name, Params: params, Return: ret, Body: body}
}

func (p *Parser) parseParams() []*Param {
	params := []*Param{p.parseParam()}
	for p.matchDelim(",") {
		params = append(params, p.parseParam())
	}
	return params
}

func (p *Parser) parseParam() *Param {
	if p.peek().Type != "TYPE" {
		p.errorAt(p.peek(), "esperado tipo de parametro")
		return &Param{}
	}
	typ := p.parseType()
	if p.peek().Type != "IDENTIFIER" {
		p.errorAt(p.peek(), "esperado nome de parametro")
		return &Param{Type: typ}
	}
	name := p.next()
	return &Param{Pos: Pos{Line: name.Line, Column: name.Column}, Type: typ, Name: name.Value}
}

// consome o ";". O for nao pede outro.
func (p *Parser) parseVarDecl() Node {
	p.next()
	if p.peek().Type != "TYPE" {
		p.errorAt(p.peek(), "esperado tipo apos var")
		return &VarDecl{}
	}
	typ := p.parseType()
	if p.peek().Type != "IDENTIFIER" {
		p.errorAt(p.peek(), "esperado nome de variavel")
		return &VarDecl{Type: typ}
	}
	name := p.next()
	var init Node
	if p.checkOp("=") {
		p.next()
		init = p.parseExpr()
	}
	p.expectDelim(";", "esperado ';' apos declaracao")
	return &VarDecl{Pos: Pos{Line: name.Line, Column: name.Column}, Type: typ, Name: name.Value, Init: init}
}

// var nao entra aqui. "caso (x) falta;" vale; "caso (x) carteira" nao.
func (p *Parser) parseStmt() Node {
	if p.peek().Type == "ERROR" {
		p.recordLexical(p.peek())
		p.next()
		p.synchronize()
		return nil
	}
	t := p.peek()
	if p.checkDelim("{") {
		return p.parseBlock()
	}
	if t.Type == "KEYWORD" {
		switch t.Concept {
		case "if":
			return p.parseIf()
		case "for":
			return p.parseFor()
		case "while":
			return p.parseWhile()
		case "return":
			return p.parseReturn()
		case "print":
			return p.parsePrint()
		case "input":
			return p.parseInput()
		case "break":
			return p.parseBreak()
		case "else", "else if":
			p.errorAt(t, "else/else if sem if correspondente")
			p.next()
			return nil
		default:
			p.errorAt(t, "inicio de comando invalido")
			p.next()
			return nil
		}
	}
	return p.parseExprStmt()
}

func (p *Parser) parseBlock() *Block {
	if !p.expectDelim("{", "esperado '{'") {
		return &Block{}
	}
	var decls []Node
	for !p.checkDelim("}") && p.peek().Type != "" {
		before := p.pos
		if d := p.parseDecl(); d != nil {
			decls = append(decls, d)
		}
		if p.pos == before {
			p.next()
		}
	}
	p.expectDelim("}", "esperado '}'")
	return &Block{Decls: decls}
}

func (p *Parser) parseIf() Node {
	p.next()
	p.expectDelim("(", "esperado '(' apos if")
	cond := p.parseExpr()
	p.expectDelim(")", "esperado ')' apos a condicao")
	then := p.parseStmt()
	var elseIfs []ElseIf
	for p.checkConcept("else if") {
		p.next()
		p.expectDelim("(", "esperado '(' apos else if")
		cond := p.parseExpr()
		p.expectDelim(")", "esperado ')' apos a condicao")
		elseIfs = append(elseIfs, ElseIf{Cond: cond, Then: p.parseStmt()})
	}
	var elseBranch Node
	if p.matchConcept("else") {
		elseBranch = p.parseStmt()
	}
	return &IfStmt{Cond: cond, Then: then, ElseIfs: elseIfs, Else: elseBranch}
}

func (p *Parser) parseFor() Node {
	p.next()
	p.expectDelim("(", "esperado '(' apos for")
	var init Node
	if p.checkConcept("var") {
		// a declaracao ja consumiu o ";".
		init = p.parseVarDecl()
	} else {
		if !p.checkDelim(";") && !p.checkDelim(")") {
			init = p.parseExpr()
		}
		p.expectDelim(";", "esperado ';' no for")
	}
	var cond Node
	if !p.checkDelim(";") && !p.checkDelim(")") {
		cond = p.parseExpr()
	}
	p.expectDelim(";", "esperado ';' no for")
	var post Node
	if !p.checkDelim(")") {
		post = p.parseExpr()
	}
	p.expectDelim(")", "esperado ')' apos for")
	return &ForStmt{Init: init, Cond: cond, Post: post, Body: p.parseStmt()}
}

func (p *Parser) parseWhile() Node {
	p.next()
	p.expectDelim("(", "esperado '(' apos while")
	cond := p.parseExpr()
	p.expectDelim(")", "esperado ')' apos a condicao")
	return &WhileStmt{Cond: cond, Body: p.parseStmt()}
}

func (p *Parser) parseReturn() Node {
	kw := p.next()
	var value Node
	if !p.checkDelim(";") && p.peek().Type != "" && !p.checkDelim("}") {
		value = p.parseExpr()
	}
	p.expectDelim(";", "esperado ';' apos return")
	return &ReturnStmt{Pos: Pos{Line: kw.Line, Column: kw.Column}, Value: value}
}

func (p *Parser) parsePrint() Node {
	p.next()
	args := p.parseParenArgs()
	p.expectDelim(";", "esperado ';' apos print")
	return &PrintStmt{Args: args}
}

func (p *Parser) parseInput() Node {
	p.next()
	args := p.parseParenArgs()
	p.expectDelim(";", "esperado ';' apos input")
	return &InputStmt{Args: args}
}

func (p *Parser) parseParenArgs() []Node {
	p.expectDelim("(", "esperado '('")
	var args []Node
	if !p.checkDelim(")") {
		args = p.parseArgs()
	}
	p.expectDelim(")", "esperado ')'")
	return args
}

func (p *Parser) parseBreak() Node {
	kw := p.next()
	p.expectDelim(";", "esperado ';' apos break")
	return &BreakStmt{Pos: Pos{Line: kw.Line, Column: kw.Column}}
}

// sem expressao valida, nao cria no vazio.
func (p *Parser) parseExprStmt() Node {
	expr := p.parseExpr()
	p.expectDelim(";", "esperado ';' apos expressao")
	if expr == nil {
		return nil
	}
	return &ExprStmt{Expr: expr}
}

// devolve o concept (int, void), nao o lexema.
func (p *Parser) parseType() string {
	t := p.peek()
	if t.Type != "TYPE" {
		p.errorAt(t, "esperado tipo")
		return ""
	}
	p.next()
	return t.Concept
}

// =, depois == !=, < >, + -, * /, unario e primario.
func (p *Parser) parseExpr() Node {
	return p.parseAssign()
}

// so nome seguido de "=". A recursao faz a = b = 1.
func (p *Parser) parseAssign() Node {
	if p.peek().Type != "IDENTIFIER" {
		return p.parseEquality()
	}
	next := p.peekNext()
	if next.Type != "OPERATOR" || next.Value != "=" {
		return p.parseEquality()
	}
	name := p.next()
	p.next()
	return &Assign{Pos: Pos{Line: name.Line, Column: name.Column}, Name: name.Value, Value: p.parseAssign()}
}

func (p *Parser) parseEquality() Node {
	left := p.parseComparison()
	for {
		op, line, col, ok := p.matchOp("==", "!=")
		if !ok {
			return left
		}
		left = &Binary{Pos: Pos{Line: line, Column: col}, Op: op, Left: left, Right: p.parseComparison()}
	}
}

func (p *Parser) parseComparison() Node {
	left := p.parseTerm()
	for {
		op, line, col, ok := p.matchOp("<", ">", "<=", ">=")
		if !ok {
			return left
		}
		left = &Binary{Pos: Pos{Line: line, Column: col}, Op: op, Left: left, Right: p.parseTerm()}
	}
}

func (p *Parser) parseTerm() Node {
	left := p.parseFactor()
	for {
		op, line, col, ok := p.matchOp("+", "-")
		if !ok {
			return left
		}
		left = &Binary{Pos: Pos{Line: line, Column: col}, Op: op, Left: left, Right: p.parseFactor()}
	}
}

func (p *Parser) parseFactor() Node {
	left := p.parseUnary()
	for {
		op, line, col, ok := p.matchOp("*", "/")
		if !ok {
			return left
		}
		left = &Binary{Pos: Pos{Line: line, Column: col}, Op: op, Left: left, Right: p.parseUnary()}
	}
}

func (p *Parser) parseUnary() Node {
	if p.checkOp("-") {
		tok := p.next()
		return &Unary{Pos: Pos{Line: tok.Line, Column: tok.Column}, Op: tok.Value, Expr: p.parseUnary()}
	}
	return p.parsePrimary()
}

func (p *Parser) parsePrimary() Node {
	t := p.peek()
	if t.Type == "ERROR" {
		p.recordLexical(t)
		p.next()
		p.panicMode = true
		return nil
	}
	switch t.Type {
	case "INT":
		p.next()
		return &Literal{Pos: Pos{Line: t.Line, Column: t.Column}, Kind: "int", Value: t.Value}
	case "FLOAT":
		p.next()
		return &Literal{Pos: Pos{Line: t.Line, Column: t.Column}, Kind: "float", Value: t.Value}
	case "STRING":
		p.next()
		return &Literal{Pos: Pos{Line: t.Line, Column: t.Column}, Kind: "string", Value: t.Value}
	case "BOOL_LITERAL":
		p.next()
		return &Literal{Pos: Pos{Line: t.Line, Column: t.Column}, Kind: "bool", Value: t.Concept}
	case "IDENTIFIER":
		p.next()
		if p.checkDelim("(") {
			return p.parseCall(t.Value, t.Line, t.Column)
		}
		return &Ident{Pos: Pos{Line: t.Line, Column: t.Column}, Name: t.Value}
	case "KEYWORD":
		// com "(" e expressao, nao o comando.
		next := p.peekNext()
		if (t.Concept == "print" || t.Concept == "input") && next.Type == "DELIMITER" && next.Value == "(" {
			p.next()
			return p.parseCall(t.Value, t.Line, t.Column)
		}
	}
	if t.Type == "DELIMITER" && t.Value == "(" {
		p.next()
		expr := p.parseExpr()
		p.expectDelim(")", "esperado ')' apos a expressao")
		return expr
	}
	p.errorAt(t, "esperado expressao")
	return nil
}

func (p *Parser) parseCall(name string, line, col int) *Call {
	p.next()
	var args []Node
	if !p.checkDelim(")") {
		args = p.parseArgs()
	}
	p.expectDelim(")", "esperado ')' apos os argumentos")
	return &Call{Pos: Pos{Line: line, Column: col}, Name: name, Args: args}
}

func (p *Parser) parseArgs() []Node {
	args := []Node{p.parseExpr()}
	for p.matchDelim(",") {
		args = append(args, p.parseExpr())
	}
	return args
}
