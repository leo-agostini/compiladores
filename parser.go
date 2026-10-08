package main

import "fmt"

type Parser struct {
	tokens []Token
	pos    int
	errors []string
	// panicMode evita uma cascata de erros no mesmo ponto: o primeiro
	// silencia os seguintes ate synchronize achar ";" ou "}". Sem isso, um
	// token ruim geraria uma mensagem por nao-terminal.
	panicMode bool
}

func Parse(tokens []Token) (*Program, []string) {
	p := &Parser{tokens: tokens}
	return p.parseProgram(), p.errors
}

func (p *Parser) skipComments() {
	for p.pos < len(p.tokens) && p.tokens[p.pos].Type == "COMMENT" {
		p.pos++
	}
}

func (p *Parser) peek() Token {
	p.skipComments()
	if p.pos >= len(p.tokens) {
		return Token{}
	}
	return p.tokens[p.pos]
}

func (p *Parser) peekAt(offset int) Token {
	p.skipComments()
	seen := 0
	for i := p.pos; i < len(p.tokens); i++ {
		t := p.tokens[i]
		if t.Type == "COMMENT" {
			continue
		}
		if seen == offset {
			return t
		}
		seen++
	}
	return Token{}
}

func (p *Parser) next() Token {
	t := p.peek()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return t
}

// OPERATOR e DELIMITER se distinguem pelo Value ("+", "("), nao por um tipo
// PLUS/LPAREN: o tokenizer nao reclassifica, e um passo extra quebraria o
// contrato da fatia.
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

func (p *Parser) matchOp(values ...string) (string, bool) {
	t := p.peek()
	if t.Type != "OPERATOR" {
		return "", false
	}
	for _, v := range values {
		if t.Value == v {
			p.next()
			return v, true
		}
	}
	return "", false
}

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

func (p *Parser) recordLexical(t Token) {
	msg := t.Message
	if msg == "" {
		msg = "erro lexico"
	}
	p.errors = append(p.errors, fmt.Sprintf("linha %d, col %d: %s", t.Line, t.Column, msg))
}

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

func (p *Parser) startsDecl(t Token) bool {
	if t.Type == "TYPE" {
		return true
	}
	if t.Type != "KEYWORD" {
		return false
	}
	switch t.Concept {
	case "function", "var", "if", "for", "while", "return", "print", "input", "break":
		return true
	}
	return false
}

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
		if t.Type == "DELIMITER" && t.Value == ";" {
			p.next()
			return
		}
		if t.Type == "DELIMITER" && (t.Value == "}" || t.Value == "{") {
			return
		}
		if p.startsDecl(t) {
			return
		}
		p.next()
	}
}

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
	keywordForm := false
	if p.checkConcept("function") {
		p.next()
		keywordForm = true
	} else {
		ret = p.parseType()
	}

	if p.peek().Type != "IDENTIFIER" {
		p.errorAt(p.peek(), "esperado nome de funcao")
		return &FuncDecl{Return: ret}
	}
	name := p.next().Value

	p.expectDelim("(", "esperado '(' apos o nome da funcao")
	var params []*Param
	if !p.checkDelim(")") && p.peek().Type != "" && !p.checkDelim("{") {
		params = p.parseParams()
	}
	p.expectDelim(")", "esperado ')' apos os parametros")
	if keywordForm && p.peek().Type == "TYPE" {
		ret = p.parseType()
	}
	body := p.parseBlock()
	return &FuncDecl{Name: name, Params: params, Return: ret, Body: body}
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
	return &Param{Type: typ, Name: p.next().Value}
}

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
	name := p.next().Value
	var init Node
	if p.checkOp("=") {
		p.next()
		init = p.parseExpr()
	}
	p.expectDelim(";", "esperado ';' apos declaracao")
	return &VarDecl{Type: typ, Name: name, Init: init}
}

func (p *Parser) parseStmt() Node {
	if p.peek().Type == "ERROR" {
		p.recordLexical(p.peek())
		p.next()
		p.synchronize()
		return nil
	}
	t := p.peek()
	if t.Type == "DELIMITER" && t.Value == "{" {
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
	var elifs []ElseIf
	// else if / senao caso / recurso ja chegam como um token, Concept "else if".
	// Fundir de novo faria "recurso" virar else seguido de um if solto.
	for p.checkConcept("else if") {
		p.next()
		p.expectDelim("(", "esperado '(' apos else if")
		c := p.parseExpr()
		p.expectDelim(")", "esperado ')' apos a condicao")
		elifs = append(elifs, ElseIf{Cond: c, Then: p.parseStmt()})
	}
	var els Node
	if p.matchConcept("else") {
		els = p.parseStmt()
	}
	return &IfStmt{Cond: cond, Then: then, ElseIfs: elifs, Else: els}
}

func (p *Parser) parseFor() Node {
	p.next()
	p.expectDelim("(", "esperado '(' apos for")
	var init Node
	if p.checkConcept("var") {
		// varDecl ja consome o ";". Sem este ramo, o parser esperaria outro
		// ";" e "jornada (carteira int i = 0; ...)" leria o ponto-e-virgula duas vezes.
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
	p.next()
	var value Node
	if !p.checkDelim(";") && p.peek().Type != "" && !p.checkDelim("}") {
		value = p.parseExpr()
	}
	p.expectDelim(";", "esperado ';' apos return")
	return &ReturnStmt{Value: value}
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
	p.next()
	p.expectDelim(";", "esperado ';' apos break")
	return &BreakStmt{}
}

func (p *Parser) parseExprStmt() Node {
	expr := p.parseExpr()
	p.expectDelim(";", "esperado ';' apos expressao")
	if expr == nil {
		return nil
	}
	return &ExprStmt{Expr: expr}
}

func (p *Parser) parseType() string {
	t := p.peek()
	if t.Type != "TYPE" {
		p.errorAt(t, "esperado tipo")
		return ""
	}
	p.next()
	return t.Concept
}

func (p *Parser) parseExpr() Node {
	return p.parseAssign()
}

func (p *Parser) parseAssign() Node {
	if p.peek().Type == "IDENTIFIER" && p.peekAt(1).Type == "OPERATOR" && p.peekAt(1).Value == "=" {
		name := p.next().Value
		p.next()
		return &Assign{Name: name, Value: p.parseAssign()}
	}
	return p.parseEquality()
}

func (p *Parser) parseEquality() Node {
	left := p.parseComparison()
	for {
		op, ok := p.matchOp("==", "!=")
		if !ok {
			return left
		}
		left = &Binary{Op: op, Left: left, Right: p.parseComparison()}
	}
}

func (p *Parser) parseComparison() Node {
	left := p.parseTerm()
	for {
		op, ok := p.matchOp("<", ">", "<=", ">=")
		if !ok {
			return left
		}
		left = &Binary{Op: op, Left: left, Right: p.parseTerm()}
	}
}

func (p *Parser) parseTerm() Node {
	left := p.parseFactor()
	for {
		op, ok := p.matchOp("+", "-")
		if !ok {
			return left
		}
		left = &Binary{Op: op, Left: left, Right: p.parseFactor()}
	}
}

func (p *Parser) parseFactor() Node {
	left := p.parseUnary()
	for {
		op, ok := p.matchOp("*", "/")
		if !ok {
			return left
		}
		left = &Binary{Op: op, Left: left, Right: p.parseUnary()}
	}
}

func (p *Parser) parseUnary() Node {
	if p.checkOp("-") {
		op := p.next().Value
		return &Unary{Op: op, Expr: p.parseUnary()}
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
		return &Literal{Kind: "int", Value: t.Value}
	case "FLOAT":
		p.next()
		return &Literal{Kind: "float", Value: t.Value}
	case "STRING":
		p.next()
		return &Literal{Kind: "string", Value: t.Value}
	case "BOOL_LITERAL":
		p.next()
		return &Literal{Kind: "bool", Value: t.Concept}
	case "IDENTIFIER":
		p.next()
		if p.checkDelim("(") {
			p.next()
			var args []Node
			if !p.checkDelim(")") {
				args = p.parseArgs()
			}
			p.expectDelim(")", "esperado ')' apos os argumentos")
			return &Call{Name: t.Value, Args: args}
		}
		return &Ident{Name: t.Value}
	case "KEYWORD":
		if (t.Concept == "print" || t.Concept == "input") &&
			p.peekAt(1).Type == "DELIMITER" && p.peekAt(1).Value == "(" {
			p.next()
			p.next()
			var args []Node
			if !p.checkDelim(")") {
				args = p.parseArgs()
			}
			p.expectDelim(")", "esperado ')' apos os argumentos")
			return &Call{Name: t.Value, Args: args}
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

func (p *Parser) parseArgs() []Node {
	args := []Node{p.parseExpr()}
	for p.matchDelim(",") {
		args = append(args, p.parseExpr())
	}
	return args
}
