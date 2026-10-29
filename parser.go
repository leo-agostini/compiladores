package main

import (
	"fmt"
	"strings"
)

// Node e a arvore. Pretty existe porque o main imprime o encaixe, nao a
// fila de tokens: sem isso, o programador veria KEYWORD/IDENTIFIER em
// sequencia e nao o "caso" virando if com cond/then/else. Cada tipo
// abaixo e um construto da gramatica; a interface so pede Pretty para
// o main nao fazer type switch na hora de imprimir.
type Node interface {
	Pretty(indent string) string
}

// Program e a raiz do arquivo. Nao ha "package" nem um bloco implicito:
// o fonte e uma lista de decls no topo, e o parser reusa essa lista
// dentro de Block para nao ter duas gramaticas.
type Program struct {
	// Decls mistura func, var e comando. Um "caso" no topo do arquivo
	// e valido; separar em Functs/Vars/Stmts quebraria parseDecl.
	Decls []Node
}

func (n *Program) Pretty(indent string) string {
	return head(indent, "program", n.Decls...)
}

// FuncDecl cobre as duas formas de funcao: tipo na frente
// ("void avisar(...)") e KEYWORD function ("contratar avisar(...) void").
// O parser preenche os mesmos campos nas duas; o que muda e so a ordem
// em que Return aparece no fonte.
type FuncDecl struct {
	// Name e o lexema do IDENTIFIER (avisar, calcular), nao um Concept:
	// funcao nao e palavra reservada.
	Name   string
	Params []*Param
	// Return guarda o Concept do TYPE (int, float, void), nao o lexema.
	// Vazio quando "contratar nome() {" nao trouxe tipo depois dos ")".
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

// Param e um par tipo+nome. Nao ha default nem varargs: a lista em
// FuncDecl.Params ja e a assinatura inteira.
type Param struct {
	// Type e o Concept do TYPE (int, string, ...), igual a FuncDecl.Return.
	Type string
	Name string
}

func (n *Param) Pretty(indent string) string {
	return line(indent, "param "+n.Type+" "+n.Name)
}

// VarDecl e "carteira T nome = expr;". A palavra no fonte e carteira
// (ou var); o no se chama VarDecl porque o tokenizer ja traduziu para
// Concept "var" e o parser trabalha com o papel, nao com o lexema.
type VarDecl struct {
	// Type e o Concept do TYPE. "carteira int horas" guarda "int", nao
	// o token TYPE inteiro.
	Type string
	Name string
	// Init e nil quando nao ha "=": "carteira int extras;" ainda e decl.
	Init Node
}

func (n *VarDecl) Pretty(indent string) string {
	title := "var " + n.Type + " " + n.Name
	if n.Init == nil {
		return line(indent, title)
	}
	return head(indent, title, n.Init)
}

// Block e "{ ... }". Decls passa pelo mesmo parseDecl do programa, entao
// um bloco aceita func, var e comando; nao e so uma lista de stmt.
type Block struct {
	Decls []Node
}

func (n *Block) Pretty(indent string) string {
	return head(indent, "block", n.Decls...)
}

// IfStmt e caso / if. ElseIfs e fatia propria, nao um IfStmt aninhado
// no Else: recurso / senao caso / else if ja chega como um token so, e
// o Pretty precisa imprimir "else if" no mesmo nivel do then, nao um
// if dentro do else.
type IfStmt struct {
	Cond Node
	Then Node
	// ElseIfs existe porque recurso / senao caso / else if ja e um token
	// so. Fundir de novo faria "recurso" virar else seguido de if solto.
	ElseIfs []ElseIf
	// Else e o ramo final (senao / else). Nil quando o caso nao tem.
	Else Node
}

// ElseIf e um ramo "else if (cond) stmt". Separado de IfStmt para o
// Pretty nao reentrar em "if" e para o parser nao ter que reescrever
// o token fundido como dois KEYWORD.
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

// ForStmt e jornada / for no estilo C: (init; cond; post). Qualquer um
// dos tres pode faltar (for(;;)). Init pode ser VarDecl: nesse caso
// parseVarDecl ja consumiu o ";", e o parser do for nao pede outro.
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

// WhileStmt e plantao / while. So condicao e corpo: nao tem init/post,
// por isso nao reusa ForStmt com campos vazios (o Pretty imprimiria
// "for" e o programador acharia que escreveu jornada).
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

// ReturnStmt e rescindir / return. Value nil e "rescindir;" (void);
// com expressao e "rescindir base + 1;".
type ReturnStmt struct {
	Value Node
}

func (n *ReturnStmt) Pretty(indent string) string {
	if n.Value == nil {
		return line(indent, "return")
	}
	return head(indent, "return", n.Value)
}

// PrintStmt e o comando bater_ponto / print seguido de ";". Quando
// print aparece no meio de uma expressao, vira Call, nao este no:
// os dois compartilham parseParenArgs, mas o comando e quem come o ";".
type PrintStmt struct {
	Args []Node
}

func (n *PrintStmt) Pretty(indent string) string {
	return head(indent, "print", n.Args...)
}

// InputStmt e o comando reclamacao / input seguido de ";". A chamada
// "nome = reclamacao()" e um Call (lexema no Name), porque ali input
// e expressao, nao comando.
type InputStmt struct {
	Args []Node
}

func (n *InputStmt) Pretty(indent string) string {
	return head(indent, "input", n.Args...)
}

// BreakStmt e falta / break. Sem campos: o comando nao carrega valor
// nem rotulo.
type BreakStmt struct{}

func (n *BreakStmt) Pretty(indent string) string {
	return line(indent, "break")
}

// ExprStmt e uma expressao usada como comando (atribuicao ou chamada
// no topo, sempre com ";"). Pretty imprime o filho direto, sem um
// nivel "expr" extra, para a saida acompanhar o que o programador
// escreveu; so mostra "expr" quando o filho e nil.
type ExprStmt struct {
	Expr Node
}

func (n *ExprStmt) Pretty(indent string) string {
	if n.Expr == nil {
		return line(indent, "expr")
	}
	return n.Expr.Pretty(indent)
}

// Assign e "nome = expr". So vale para IDENTIFIER a esquerda;
// "carteira" e KEYWORD var e vira VarDecl, nao passa por aqui. A
// associatividade e a direita (a = b = 1) porque parseAssign se chama
// de novo depois do "=".
type Assign struct {
	// Name e o lexema do identificador (taxa, horas), nao um Concept.
	Name  string
	Value Node
}

func (n *Assign) Pretty(indent string) string {
	return head(indent, "=", &Ident{Name: n.Name}, n.Value)
}

// Binary e operador infixo com dois filhos. Op e o Value do OPERATOR
// ("+", "=="), nao um enum PLUS: o tokenizer nao reclassifica, e o
// parser distingue pelo lexema. A arvore e esquerda-associativa
// (1+2+3 vira (+ (+ 1 2) 3)) porque o laco em parseTerm/parseFactor
// vai empilhando a esquerda.
type Binary struct {
	Op    string
	Left  Node
	Right Node
}

func (n *Binary) Pretty(indent string) string {
	return head(indent, n.Op, n.Left, n.Right)
}

// Unary e o menos prefixo. Mora em parseUnary, nao em parsePrimary:
// assim "-1 * 2" e (* (- 1) 2) e nao um literal negativo. So "-"
// chega aqui; "!" e erro lexico e nem vira no.
type Unary struct {
	Op   string
	Expr Node
}

func (n *Unary) Pretty(indent string) string {
	return head(indent, n.Op, n.Expr)
}

// Call e f(args). Serve tanto para IDENTIFIER (avisar, calcular)
// quanto para print/input usados como expressao. O comando
// bater_ponto(...); continua sendo PrintStmt.
type Call struct {
	// Name guarda o lexema (avisar, reclamacao, bater_ponto), nao o
	// Concept: o Pretty deve mostrar o que o programador escreveu.
	Name string
	Args []Node
}

func (n *Call) Pretty(indent string) string {
	return head(indent, "call "+n.Name, n.Args...)
}

// Ident e um nome no meio de uma expressao. Name e o lexema; palavras
// reservadas nao chegam aqui (o tokenizer ja as reclassificou para
// KEYWORD/TYPE/BOOL_LITERAL). A esquerda de "=" vira Assign, nao Ident.
type Ident struct {
	Name string
}

func (n *Ident) Pretty(indent string) string {
	return line(indent, "ident "+n.Name)
}

// Literal e um valor imediato. parsePrimary e o unico lugar que olha
// INT/FLOAT/STRING/BOOL_LITERAL: o resto da gramatica so ve Node.
type Literal struct {
	// Kind e int/float/string/bool. Sem isso, Pretty nao saberia se
	// "44" e numero ou texto.
	Kind string
	// Value do bool e o Concept (true/false), nao "registrado"/"pj".
	// Nos outros kinds e o lexema (44, 1.5, "Ana").
	Value string
}

func (n *Literal) Pretty(indent string) string {
	return line(indent, n.Kind+" "+n.Value)
}

// line e uma folha na impressao: indent + texto + newline, sem filhos.
func line(indent, text string) string {
	return indent + text + "\n"
}

// head e um no com filhos: o titulo numa linha e cada filho dois
// espacos mais fundo. Nil vira "?" para a arvore parcial do panicMode
// nao sumir na saida (um Cond que falhou ainda ocupa o lugar).
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

// Parser consome a fatia de tokenize da esquerda para a direita.
// Descida: program -> decl (func | var | stmt) -> expr. Sem EOF: o
// fim da entrada e Type == "". errors e []string no formato
// "linha %d, col %d: %s" porque o main imprime com %s.
type Parser struct {
	tokens []Token
	pos    int
	errors []string
	// panicMode evita uma cascata de erros no mesmo ponto: o primeiro
	// silencia os seguintes ate synchronize achar ";" ou "}". Sem isso, um
	// token ruim geraria uma mensagem por nao-terminal.
	panicMode bool
}

// Parse e a porta de entrada. Devolve a arvore mesmo com erros, para o
// main ainda imprimir o que deu para montar. Sem token EOF: o fim da
// entrada e Type == "" na fatia que tokenize ja produziu.
func Parse(tokens []Token) (*Program, []string) {
	p := &Parser{tokens: tokens}
	return p.parseProgram(), p.errors
}

// skipComments avanca pos sobre COMMENT. peek, peekNext e next chamam
// isto, entao o resto do parser nunca ve comentario: senao "senao /*meio*/
// caso" fundiria aqui, e o tokenizer ja recusou essa fusao.
func (p *Parser) skipComments() {
	for p.pos < len(p.tokens) && p.tokens[p.pos].Type == "COMMENT" {
		p.pos++
	}
}

// peek e o token atual, ja sem COMMENT. Type "" e fim da entrada: o
// tokenizer nao emite EOF.
func (p *Parser) peek() Token {
	p.skipComments()
	if p.pos >= len(p.tokens) {
		return Token{}
	}
	return p.tokens[p.pos]
}

// peekNext e o token logo depois do atual, sem consumir. COMMENT no meio
// nao conta: parseAssign precisa ver se o IDENTIFIER e seguido de "=".
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

// next consome o peek. No fim da entrada peek devolve Token{} e pos nao
// anda: sem isso, synchronize rodaria para sempre no Type "".
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

// expectDelim e match + erro. O no parcial continua e panicMode liga,
// para o proximo expect no mesmo ponto nao repetir a mensagem.
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

// checkConcept olha KEYWORD pelo Concept (if, var, else if), nunca pelo
// lexema: "caso" e "if" caem no mesmo ramo.
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

// lastPos e a linha/col do ultimo token visivel. errorAt usa quando o
// peek e Type "": nao ha token atual para apontar.
func (p *Parser) lastPos() (line, col int) {
	for i := len(p.tokens) - 1; i >= 0; i-- {
		if p.tokens[i].Type != "COMMENT" {
			return p.tokens[i].Line, p.tokens[i].Column
		}
	}
	return 1, 1
}

// recordLexical formata o Message do tokenizer. Nao liga panicMode: quem
// chama e quem decide se sincroniza.
func (p *Parser) recordLexical(t Token) {
	msg := t.Message
	if msg == "" {
		msg = "erro lexico"
	}
	p.errors = append(p.errors, fmt.Sprintf("linha %d, col %d: %s", t.Line, t.Column, msg))
}

// errorAt registra um erro e liga panicMode. Type ERROR reusa a
// mensagem lexica; Type "" aponta lastPos e acrescenta "(fim da
// entrada)". Com panicMode ligado, os expect seguintes no mesmo
// ponto ficam mudos ate synchronize.
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

// startsDecl e a fronteira de synchronize. TYPE inicia funcao C-like;
// KEYWORD cobre function/var e tambem stmt (if, for, ...): parar num
// "caso" evita engolir o proximo comando na recuperacao.
//
// IDENTIFIER so conta quando o token seguinte mostra comando novo
// (a = 2, foo(), a;). Parar em tod nome descartaria o argumento em
// "bater_ponto(a b);" e o synchronize geraria outro erro no ")".
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

// synchronize sai do ponto ruim ate um lugar seguro: consome ";" (fim
// do comando), para em "{" / "}" (o bloco ainda precisa deles) ou no
// inicio de uma decl. Desliga panicMode para o proximo erro voltar a
// ser reportado.
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

// parseProgram e o laco do arquivo: decls ate Type "". "}" solto e
// erro, nao fecha um bloco que nao abriu. Se parseDecl nao andar,
// consome um token para o laco nao travar.
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

// parseDecl e uma unidade de Program.Decls / Block.Decls. Tres formas:
// funcao (contratar ... ou TYPE nome), var (carteira) ou stmt. Stmt no
// topo e valido: um "caso" no arquivo nao precisa estar dentro de funcao.
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

// parseFuncDecl cobre as duas formas: tipo na frente ("void avisar(")
// e KEYWORD function ("contratar avisar( ... ) void"). returnAfterParams
// adia o Return para depois dos ")".
func (p *Parser) parseFuncDecl() Node {
	var ret string
	// contratar nome(...) tipo: o retorno vem depois dos parametros.
	// void nome(...): o tipo ja foi lido aqui.
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
	name := p.next().Value

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

// parseVarDecl e "carteira T nome = expr;". Sempre come o ";": por isso
// o for, quando o init e var, nao pede outro ponto-e-virgula.
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

// parseStmt e um comando. Nao e decl: carteira e funcao ficam em
// parseDecl. O que entra aqui:
//
//	bloco   { ... }           parseBlock, que volta a parseDecl
//	if      caso / if         parseIf
//	for     jornada / for     parseFor
//	while   plantao / while   parseWhile
//	return  rescindir         parseReturn
//	print   bater_ponto       parsePrint (comando, come o ";")
//	input   reclamacao        parseInput
//	break   falta             parseBreak
//	expr    nome = ...; f();  parseExprStmt
//
// else / else if soltos sao erro: so parseIf os consome depois de um if.
// Then/else de um if tambem passam por aqui, nao por parseBlock: assim
// "caso (x) falta;" e valido, e "caso (x) carteira int y = 1;" nao e
// (var so entra dentro de "{ }").
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

// parseBlock e "{ decls }". Reusa parseDecl, entao um bloco aceita
// func, var e comando: a mesma lista do programa.
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
	// else if / senao caso / recurso ja chegam como um token, Concept "else if".
	// Fundir de novo faria "recurso" virar else seguido de um if solto.
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

// parseParenArgs e "( expr, expr )". Compartilhado por print/input
// comando; a chamada em expressao (reclamacao()) monta os args em
// parsePrimary, porque la o nome ja foi consumido.
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

// parseExprStmt e expressao + ";". Sem o ponto-e-virgula, "taxa = 1
// carteira int x" colaria os dois. Nil quando a expressao falhou: nao
// vale a pena embrulhar um ExprStmt vazio.
func (p *Parser) parseExprStmt() Node {
	expr := p.parseExpr()
	p.expectDelim(";", "esperado ';' apos expressao")
	if expr == nil {
		return nil
	}
	return &ExprStmt{Expr: expr}
}

// parseType devolve o Concept (int, float, void), nao o lexema. E o
// valor que FuncDecl.Return / Param.Type / VarDecl.Type guardam.
func (p *Parser) parseType() string {
	t := p.peek()
	if t.Type != "TYPE" {
		p.errorAt(t, "esperado tipo")
		return ""
	}
	p.next()
	return t.Concept
}

// parseExpr e o topo da expressao. A precedencia desce por funcoes:
// assign (= direita) > equality (== !=) > comparison (<>) > term (+ -)
// > factor (* /) > unary (-) > primary. Cada infixo e um laco a
// esquerda; so assign chama a si mesmo a direita (a = b = 1).
func (p *Parser) parseExpr() Node {
	return p.parseAssign()
}

// parseAssign so casa IDENTIFIER "=". Sem olhar o proximo token, "taxa = 1"
// viraria Ident seguido de erro no "=". A chamada recursiva faz a = b = 1.
func (p *Parser) parseAssign() Node {
	if p.peek().Type != "IDENTIFIER" {
		return p.parseEquality()
	}
	next := p.peekNext()
	if next.Type != "OPERATOR" || next.Value != "=" {
		return p.parseEquality()
	}
	name := p.next().Value
	p.next()
	return &Assign{Name: name, Value: p.parseAssign()}
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

// parsePrimary e o unico lugar que olha literais. INT/FLOAT/STRING usam
// o lexema; BOOL usa o Concept (true/false), nao "registrado"/"pj".
// IDENTIFIER seguido de "(" e Call; print/input com "(" tambem, porque
// ali sao expressao (nome = reclamacao()), nao comando.
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
			return p.parseCall(t.Value)
		}
		return &Ident{Name: t.Value}
	case "KEYWORD":
		// print/input com "(" sao expressao (nome = reclamacao()), nao o
		// comando, que e quem consome o ";".
		next := p.peekNext()
		if (t.Concept == "print" || t.Concept == "input") && next.Type == "DELIMITER" && next.Value == "(" {
			p.next()
			return p.parseCall(t.Value)
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

// parseCall consome "(" args ")" depois que o nome ja foi lido.
func (p *Parser) parseCall(name string) *Call {
	p.next()
	var args []Node
	if !p.checkDelim(")") {
		args = p.parseArgs()
	}
	p.expectDelim(")", "esperado ')' apos os argumentos")
	return &Call{Name: name, Args: args}
}

func (p *Parser) parseArgs() []Node {
	args := []Node{p.parseExpr()}
	for p.matchDelim(",") {
		args = append(args, p.parseExpr())
	}
	return args
}
