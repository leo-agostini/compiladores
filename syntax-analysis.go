package main

import (
	"fmt"
	"slices"
)

// ---------------------------------------------------------------------------
// Arvore sintatica
// ---------------------------------------------------------------------------

// Node e a interface unica da AST: cada no diz como se descreve (Label) e quem
// sao seus filhos (Children). Assim a impressao da arvore e generica e um no
// novo nao obriga a mexer no printer.
type Node interface {
	Label() string
	Children() []Node
}

// group nao vem da gramatica: e so um rotulo ("condicao", "entao", "corpo")
// para dar nome ao papel de um grupo de filhos na saida.
type group struct {
	name  string
	nodes []Node
}

func (g *group) Label() string    { return g.name }
func (g *group) Children() []Node { return g.nodes }

// groupOf devolve nil quando nao ha nada dentro, para o rotulo vazio nao
// aparecer na arvore.
func groupOf(name string, nodes ...Node) Node {
	kept := make([]Node, 0, len(nodes))
	for _, node := range nodes {
		if node != nil {
			kept = append(kept, node)
		}
	}
	if len(kept) == 0 {
		return nil
	}
	return &group{name: name, nodes: kept}
}

func blockGroup(name string, block *Block) Node {
	if block == nil {
		return nil
	}
	return &group{name: name, nodes: block.Stmts}
}

// pos guarda de onde veio o no, para a mensagem de erro e para uma futura fase
// semantica poder apontar o lugar certo no codigo.
type pos struct {
	Line   int
	Column int
}

type Program struct {
	Decls []Node
}

func (n *Program) Label() string    { return "Programa" }
func (n *Program) Children() []Node { return n.Decls }

type VarDecl struct {
	pos
	Type  string
	Name  string
	Value Node // nil quando a variavel e declarada sem inicializacao
}

func (n *VarDecl) Label() string { return fmt.Sprintf("VarDecl %s %s", n.Type, n.Name) }
func (n *VarDecl) Children() []Node {
	if n.Value == nil {
		return nil
	}
	return []Node{n.Value}
}

type Param struct {
	pos
	Type string
	Name string
}

func (n *Param) Label() string    { return fmt.Sprintf("Param %s %s", n.Type, n.Name) }
func (n *Param) Children() []Node { return nil }

// FuncDecl vem das duas formas de declarar funcao ("float f(...) {" e
// "contratar f(...) float {"). As duas preenchem os mesmos campos; na forma com
// 'contratar' sem tipo depois do ')', Type fica "void".
type FuncDecl struct {
	pos
	Type   string
	Name   string
	Params []Node
	Body   *Block
}

func (n *FuncDecl) Label() string { return fmt.Sprintf("FuncDecl %s %s", n.Type, n.Name) }
func (n *FuncDecl) Children() []Node {
	return children(groupOf("parametros", n.Params...), blockGroup("corpo", n.Body))
}

type Block struct {
	pos
	Stmts []Node
}

func (n *Block) Label() string    { return "Bloco" }
func (n *Block) Children() []Node { return n.Stmts }

// Branch e um ramo "senao caso": condicao mais corpo.
type Branch struct {
	pos
	Cond Node
	Body *Block
}

type IfStmt struct {
	pos
	Cond    Node
	Then    *Block
	ElseIfs []*Branch
	Else    *Block
}

func (n *IfStmt) Label() string { return "Se (caso)" }
func (n *IfStmt) Children() []Node {
	nodes := children(groupOf("condicao", n.Cond), blockGroup("entao", n.Then))
	for _, branch := range n.ElseIfs {
		nodes = append(nodes, groupOf("senao caso",
			groupOf("condicao", branch.Cond), blockGroup("entao", branch.Body)))
	}
	return append(nodes, children(blockGroup("senao", n.Else))...)
}

type WhileStmt struct {
	pos
	Cond Node
	Body *Block
}

func (n *WhileStmt) Label() string { return "Enquanto (plantao)" }
func (n *WhileStmt) Children() []Node {
	return children(groupOf("condicao", n.Cond), blockGroup("corpo", n.Body))
}

type ForStmt struct {
	pos
	Init Node // os tres cabecalhos sao opcionais: jornada (;;) e valido
	Cond Node
	Post Node
	Body *Block
}

func (n *ForStmt) Label() string { return "Para (jornada)" }
func (n *ForStmt) Children() []Node {
	return children(groupOf("inicio", n.Init), groupOf("condicao", n.Cond),
		groupOf("passo", n.Post), blockGroup("corpo", n.Body))
}

type ReturnStmt struct {
	pos
	Value Node // nil em "rescindir;", usado por funcoes void
}

func (n *ReturnStmt) Label() string    { return "Retorno (rescindir)" }
func (n *ReturnStmt) Children() []Node { return children(n.Value) }

type BreakStmt struct{ pos }

func (n *BreakStmt) Label() string    { return "Interrupcao (falta)" }
func (n *BreakStmt) Children() []Node { return nil }

// PrintStmt tem no proprio, e nao vira CallExpr, pelo mesmo motivo que
// InputExpr: "print" e "bater_ponto" sao o mesmo conceito e precisam gerar a
// mesma arvore. Como CallExpr guarda o lexema, os dois sinonimos virariam
// chamadas de funcoes com nomes diferentes.
type PrintStmt struct {
	pos
	Args []Node
}

func (n *PrintStmt) Label() string { return "Saida (bater_ponto)" }
func (n *PrintStmt) Children() []Node {
	return children(groupOf("argumentos", n.Args...))
}

type Assign struct {
	pos
	Name  string
	Value Node
}

func (n *Assign) Label() string    { return fmt.Sprintf("Atribuicao %s =", n.Name) }
func (n *Assign) Children() []Node { return children(n.Value) }

type ExprStmt struct {
	pos
	Expr Node
}

func (n *ExprStmt) Label() string    { return "ExprComando" }
func (n *ExprStmt) Children() []Node { return children(n.Expr) }

type BinaryExpr struct {
	pos
	Op    string
	Left  Node
	Right Node
}

func (n *BinaryExpr) Label() string    { return "Binario " + n.Op }
func (n *BinaryExpr) Children() []Node { return children(n.Left, n.Right) }

type UnaryExpr struct {
	pos
	Op      string
	Operand Node
}

func (n *UnaryExpr) Label() string    { return "Unario " + n.Op }
func (n *UnaryExpr) Children() []Node { return children(n.Operand) }

type CallExpr struct {
	pos
	Name string
	Args []Node
}

func (n *CallExpr) Label() string { return "Chamada " + n.Name }
func (n *CallExpr) Children() []Node {
	return children(groupOf("argumentos", n.Args...))
}

type InputExpr struct{ pos }

func (n *InputExpr) Label() string    { return "Entrada (reclamacao)" }
func (n *InputExpr) Children() []Node { return nil }

type Literal struct {
	pos
	Kind  string // INT, FLOAT, STRING ou BOOL_LITERAL
	Value string
}

func (n *Literal) Label() string    { return n.Kind + " " + display(n.Value) }
func (n *Literal) Children() []Node { return nil }

type Ident struct {
	pos
	Name string
}

func (n *Ident) Label() string    { return "Ident " + n.Name }
func (n *Ident) Children() []Node { return nil }

// children descarta os filhos nulos. Sem isso, um filho opcional ausente viraria
// uma interface nao-nula apontando para nada e o printer quebraria.
func children(nodes ...Node) []Node {
	kept := make([]Node, 0, len(nodes))
	for _, node := range nodes {
		if node != nil {
			kept = append(kept, node)
		}
	}
	return kept
}

// ---------------------------------------------------------------------------
// Parser
// ---------------------------------------------------------------------------

type SyntaxError struct {
	Line    int
	Column  int
	Message string
}

type parser struct {
	tokens []Token
	pos    int
	eof    Token
	errors []SyntaxError
}

// parse roda a analise sintatica sobre os tokens do lexer. Sempre devolve um
// Program (possivelmente incompleto) e a lista de erros: com modo panico o
// parser nao para no primeiro problema, sincroniza e segue para reportar tudo
// numa passada so, igual ao lexer.
func parse(tokens []Token) (*Program, []SyntaxError) {
	p := newParser(tokens)
	program := &Program{}

	for !p.atEnd() {
		// No topo nao ha bloco aberto: um '}' aqui sempre sobra. Tratado a
		// parte porque a sincronizacao para em '}' sem consumi-lo.
		if p.check("DELIMITER", "}") {
			p.fail(p.next(), "'}' sem um '{' correspondente")
			continue
		}

		before := p.pos
		if decl := p.parseTopLevel(); decl != nil {
			program.Decls = append(program.Decls, decl)
			continue
		}
		p.synchronize()
		// Garantia de progresso: se a sincronizacao parou onde ja estava, o
		// laco ficaria preso no mesmo token para sempre.
		if p.pos == before {
			p.next()
		}
	}

	return program, p.errors
}

func newParser(tokens []Token) *parser {
	// Comentarios sao tokens validos mas nao fazem parte da gramatica; tirar
	// aqui evita ter que ignora-los em cada regra.
	kept := make([]Token, 0, len(tokens))
	for _, token := range tokens {
		if token.Type != "COMMENT" {
			kept = append(kept, token)
		}
	}

	// Sentinela de fim: peek() nunca precisa checar o limite do slice, e o erro
	// "esperava X, encontrei o fim do arquivo" ganha linha e coluna.
	eof := Token{Type: "EOF", Line: 1, Column: 1}
	if len(kept) > 0 {
		last := kept[len(kept)-1]
		eof.Line = last.Line
		eof.Column = last.Column + len([]rune(last.Value))
	}

	return &parser{tokens: kept, eof: eof}
}

func (p *parser) atEnd() bool { return p.pos >= len(p.tokens) }

func (p *parser) peek() Token { return p.peekAt(0) }

func (p *parser) peekAt(n int) Token {
	if p.pos+n >= len(p.tokens) {
		return p.eof
	}
	return p.tokens[p.pos+n]
}

func (p *parser) next() Token {
	token := p.peek()
	if !p.atEnd() {
		p.pos++
	}
	return token
}

// check compara tipo e lexema. Um value vazio casa qualquer lexema daquele
// tipo, o que serve para "um TYPE qualquer" ou "um IDENTIFIER qualquer".
func (p *parser) check(tokenType, value string) bool {
	return p.checkAt(0, tokenType, value)
}

func (p *parser) checkAt(n int, tokenType, value string) bool {
	token := p.peekAt(n)
	return token.Type == tokenType && (value == "" || token.Value == value)
}

// checkConcept olha o papel semantico da palavra reservada, nunca o lexema.
// E o que faz "caso" e "if" (ou "senao caso" e "recurso") caminharem pela mesma
// regra sem duplicar a tabela de sinonimos que ja mora no lexer.
func (p *parser) checkConcept(concept string) bool {
	token := p.peek()
	return token.Type == "KEYWORD" && token.Concept == concept
}

func (p *parser) accept(tokenType, value string) bool {
	if p.check(tokenType, value) {
		p.next()
		return true
	}
	return false
}

// expect consome o token esperado ou registra o erro e devolve false. Nao
// aborta: quem chamou decide se sincroniza ou se ainda da para continuar.
func (p *parser) expect(tokenType, value, what string) (Token, bool) {
	if p.check(tokenType, value) {
		return p.next(), true
	}

	// O que faltou pertence ao fim do token anterior. Se o encontrado ja esta
	// em outra linha (o caso tipico do ';' esquecido), apontar para ele levaria
	// o programador para a linha errada.
	found := p.peek()
	if p.pos > 0 && found.Type != "EOF" {
		if prev := p.tokens[p.pos-1]; found.Line > prev.Line {
			found.Line, found.Column = prev.Line, prev.Column+len([]rune(prev.Value))
		}
	}
	p.errorAt(found, what)
	return Token{}, false
}

// errorAt reporta o que faltava e o que apareceu no lugar. Para quando o
// proprio token e a explicacao ("'senao' sem um 'caso' antes"), use fail, senao
// a mensagem repete o lexema duas vezes.
func (p *parser) errorAt(token Token, what string) {
	p.fail(token, fmt.Sprintf("%s, encontrei %s", what, describe(token)))
}

func (p *parser) fail(token Token, message string) {
	p.errors = append(p.errors, SyntaxError{
		Line:    token.Line,
		Column:  token.Column,
		Message: message,
	})
}

func describe(token Token) string {
	if token.Type == "EOF" {
		return "o fim do arquivo"
	}
	return "'" + display(token.Value) + "'"
}

// startsStatement lista os conceitos que so podem aparecer no comeco de um
// comando. Sao os pontos seguros onde a sincronizacao para.
//
// "input" fica de fora de proposito: reclamacao() e expressao e costuma
// aparecer no meio de uma ("x = reclamacao();"). Parar nela recomecaria a
// analise no meio do comando quebrado e geraria um segundo erro derivado.
var startsStatement = map[string]bool{
	"function": true, "var": true, "if": true, "while": true, "for": true,
	"return": true, "break": true, "print": true,
}

// synchronize implementa o modo panico: depois de um erro, joga fora os tokens
// ate um ponto onde a analise plausivelmente recomeca (depois de um ';', ou em
// cima de uma chave, de um inicio de comando ou de um cabecalho de funcao). Sem isso, um ';'
// esquecido produziria uma cascata de erros derivados do mesmo engano.
func (p *parser) synchronize() {
	for !p.atEnd() {
		token := p.peek()
		if token.Type == "DELIMITER" && token.Value == ";" {
			p.next()
			return
		}
		if token.Type == "DELIMITER" && (token.Value == "{" || token.Value == "}") {
			return
		}
		// Um tipo so recomeca algo quando abre funcao: ele tambem aparece em
		// lista de parametros e depois de 'carteira', e parar ali reiniciaria
		// a analise no meio do comando quebrado.
		if p.startsFunction() || startsStatement[token.Concept] {
			return
		}
		p.next()
	}
}

// ---------------------------------------------------------------------------
// Declaracoes
// ---------------------------------------------------------------------------

// Topo -> FuncDecl | Comando
//
// A linguagem nao tem funcao main: o corpo do programa sao comandos soltos no
// topo do arquivo (ver testes/teste.clt), entao la vale tudo que vale dentro de um
// bloco, mais a declaracao de funcao, que so existe no topo.
func (p *parser) parseTopLevel() Node {
	if p.checkConcept("function") {
		return p.parseFunction()
	}
	if p.check("TYPE", "") {
		// Um tipo no topo so pode abrir uma funcao: variavel comeca com
		// 'carteira'. A mensagem diz isso em vez de reclamar generico do
		// token seguinte.
		if !p.checkAt(1, "IDENTIFIER", "") || !p.checkAt(2, "DELIMITER", "(") {
			p.errorAt(p.peek(), "esperava uma declaracao de funcao (tipo, nome e '('); variavel comeca com 'carteira'")
			return nil
		}
		return p.parseFunction()
	}
	return p.parseStatement()
}

// startsFunction reconhece as duas formas de cabecalho de funcao sem consumir
// nada. O "TYPE IDENT (" olha tres tokens a frente porque "int" sozinho nao
// distingue funcao de uma variavel declarada sem 'carteira'.
func (p *parser) startsFunction() bool {
	return p.checkConcept("function") ||
		p.check("TYPE", "") && p.checkAt(1, "IDENTIFIER", "") && p.checkAt(2, "DELIMITER", "(")
}

// VarDecl -> 'carteira' TYPE IDENT ('=' Expr)? ';'
// O ';' vira opcional dentro do cabecalho do 'jornada', onde quem separa e o for.
func (p *parser) parseVarDecl(semicolon bool) Node {
	start := p.next()

	varType, ok := p.expect("TYPE", "", "esperava um tipo (int, float, string, bool, void) depois de 'carteira'")
	if !ok {
		return nil
	}
	name, ok := p.expect("IDENTIFIER", "", "esperava o nome da variavel depois do tipo")
	if !ok {
		return nil
	}

	decl := &VarDecl{pos: at(start), Type: varType.Value, Name: name.Value}
	if p.accept("OPERATOR", "=") {
		if decl.Value = p.parseExpr(); decl.Value == nil {
			return nil
		}
	}
	if semicolon {
		if _, ok := p.expect("DELIMITER", ";", "esperava ';' no fim da declaracao"); !ok {
			return nil
		}
	}
	return decl
}

// FuncDecl -> TYPE IDENT '(' Params ')' Bloco
// FuncDecl -> 'contratar' IDENT '(' Params ')' TYPE? Bloco
//
// A forma com 'contratar' poe o tipo de retorno depois dos parametros e
// permite omiti-lo, o que significa void.
func (p *parser) parseFunction() Node {
	var decl *FuncDecl

	if p.checkConcept("function") {
		keyword := p.next()
		name, ok := p.expect("IDENTIFIER", "", "esperava o nome da funcao depois de '"+keyword.Value+"'")
		if !ok {
			return nil
		}
		if _, ok := p.expect("DELIMITER", "(", "esperava '(' depois do nome da funcao"); !ok {
			return nil
		}
		decl = &FuncDecl{pos: at(keyword), Type: "void", Name: name.Value}
		if decl.Params, ok = p.parseParams(); !ok {
			return nil
		}
		if p.check("TYPE", "") {
			decl.Type = p.next().Value
		}
	} else {
		returnType := p.next()
		name := p.next()
		p.next() // '(', ja confirmado por startsFunction ou parseTopLevel

		decl = &FuncDecl{pos: at(returnType), Type: returnType.Value, Name: name.Value}
		var ok bool
		if decl.Params, ok = p.parseParams(); !ok {
			return nil
		}
	}

	if decl.Body = p.parseBlock(); decl.Body == nil {
		return nil
	}
	return decl
}

// Params -> vazio | TYPE IDENT (',' TYPE IDENT)*
// Chamado com o '(' ja consumido; consome ate o ')' inclusive.
func (p *parser) parseParams() ([]Node, bool) {
	var params []Node
	if !p.check("DELIMITER", ")") {
		for {
			paramType, ok := p.expect("TYPE", "", "esperava o tipo do parametro")
			if !ok {
				return nil, false
			}
			paramName, ok := p.expect("IDENTIFIER", "", "esperava o nome do parametro depois do tipo")
			if !ok {
				return nil, false
			}
			params = append(params, &Param{pos: at(paramType), Type: paramType.Value, Name: paramName.Value})
			if !p.accept("DELIMITER", ",") {
				break
			}
		}
	}
	if _, ok := p.expect("DELIMITER", ")", "esperava ')' fechando a lista de parametros"); !ok {
		return nil, false
	}
	return params, true
}

// Bloco -> '{' Comando* '}'
func (p *parser) parseBlock() *Block {
	open, ok := p.expect("DELIMITER", "{", "esperava '{' abrindo o bloco")
	if !ok {
		return nil
	}

	block := &Block{pos: at(open)}
	for !p.atEnd() && !p.check("DELIMITER", "}") {
		before := p.pos
		if stmt := p.parseStatement(); stmt != nil {
			block.Stmts = append(block.Stmts, stmt)
			continue
		}
		p.synchronize()
		if p.pos == before {
			p.next()
		}
	}

	if _, ok := p.expect("DELIMITER", "}", "esperava '}' fechando o bloco"); !ok {
		return nil
	}
	return block
}

// ---------------------------------------------------------------------------
// Comandos
// ---------------------------------------------------------------------------

// Comando -> VarDecl | Se | Enquanto | Para | Retorno | Falta | Saida | Bloco
// Comando -> ComandoSimples ';'
func (p *parser) parseStatement() Node {
	switch {
	case p.checkConcept("var"):
		return p.parseVarDecl(true)
	case p.checkConcept("if"):
		return p.parseIf()
	case p.checkConcept("while"):
		return p.parseWhile()
	case p.checkConcept("for"):
		return p.parseFor()
	case p.checkConcept("return"):
		return p.parseReturn()
	case p.checkConcept("break"):
		return p.parseBreak()
	case p.checkConcept("print"):
		return p.parsePrint()

	// Funcao so existe no topo do arquivo. Mesmo assim ela e lida inteira:
	// sem isso, a recuperacao tropecaria nos parametros e no corpo e cada
	// pedaco viraria um erro derivado.
	case p.startsFunction():
		p.fail(p.peek(), "funcao so pode ser declarada no topo do arquivo, fora de qualquer bloco")
		return p.parseFunction()
	case p.check("TYPE", ""):
		p.errorAt(p.peek(), "esperava um comando; variavel comeca com 'carteira'")
		return nil

	// 'senao' e 'senao caso' so existem grudados num 'caso' anterior, que ja
	// os teria consumido. Sozinhos aqui, a causa provavel e o bloco do 'caso'
	// nao ter fechado.
	case p.checkConcept("else"), p.checkConcept("else if"):
		p.fail(p.peek(), "'"+p.peek().Value+"' sem um 'caso' antes; verifique se o bloco anterior fechou")
		return nil

	case p.check("DELIMITER", "{"):
		block := p.parseBlock()
		if block == nil {
			return nil
		}
		return block
	}

	stmt := p.parseSimpleStatement()
	if stmt == nil {
		return nil
	}
	if _, ok := p.expect("DELIMITER", ";", "esperava ';' no fim do comando"); !ok {
		return nil
	}
	return stmt
}

// ComandoSimples -> VarDecl sem ';' | Atribuicao | Expr
//
// Sem o ';' para servir tambem ao cabecalho do 'jornada'. A declaracao so
// entra aqui pela inicializacao do 'jornada': num comando comum, parseStatement
// ja desviou o 'carteira' para parseVarDecl(true).
func (p *parser) parseSimpleStatement() Node {
	if p.checkConcept("var") {
		return p.parseVarDecl(false)
	}
	if p.check("IDENTIFIER", "") && p.checkAt(1, "OPERATOR", "=") {
		return p.parseAssign()
	}

	start := p.peek()
	expr := p.parseExpr()
	if expr == nil {
		return nil
	}
	return &ExprStmt{pos: at(start), Expr: expr}
}

// Atribuicao -> IDENT '=' Expr
func (p *parser) parseAssign() Node {
	name := p.next()
	p.next() // '='

	value := p.parseExpr()
	if value == nil {
		return nil
	}
	return &Assign{pos: at(name), Name: name.Value, Value: value}
}

// Se -> 'caso' '(' Expr ')' Bloco ('senao caso' '(' Expr ')' Bloco)* ('senao' Bloco)?
func (p *parser) parseIf() Node {
	keyword := p.next()

	cond, body, ok := p.parseCondBlock(keyword.Value)
	if !ok {
		return nil
	}
	stmt := &IfStmt{pos: at(keyword), Cond: cond, Then: body}

	// O lexer ja fundiu "senao caso" (e "recurso") num token so com o conceito
	// "else if", entao aqui basta olhar o conceito.
	for p.checkConcept("else if") {
		branch := p.next()
		cond, body, ok := p.parseCondBlock(branch.Value)
		if !ok {
			return nil
		}
		stmt.ElseIfs = append(stmt.ElseIfs, &Branch{pos: at(branch), Cond: cond, Body: body})
	}

	if p.checkConcept("else") {
		p.next()
		if stmt.Else = p.parseBlock(); stmt.Else == nil {
			return nil
		}
	}
	return stmt
}

// Enquanto -> 'plantao' '(' Expr ')' Bloco
func (p *parser) parseWhile() Node {
	keyword := p.next()

	cond, body, ok := p.parseCondBlock(keyword.Value)
	if !ok {
		return nil
	}
	return &WhileStmt{pos: at(keyword), Cond: cond, Body: body}
}

// parseCondBlock le "(condicao) { corpo }", a forma comum a caso, senao caso
// e plantao.
func (p *parser) parseCondBlock(keyword string) (Node, *Block, bool) {
	if _, ok := p.expect("DELIMITER", "(", "esperava '(' depois de '"+keyword+"'"); !ok {
		return nil, nil, false
	}
	cond := p.parseExpr()
	if cond == nil {
		return nil, nil, false
	}
	if _, ok := p.expect("DELIMITER", ")", "esperava ')' fechando a condicao"); !ok {
		return nil, nil, false
	}

	body := p.parseBlock()
	if body == nil {
		return nil, nil, false
	}
	return cond, body, true
}

// Para -> 'jornada' '(' ComandoSimples? ';' Expr? ';' (Atribuicao | Expr)? ')' Bloco
func (p *parser) parseFor() Node {
	keyword := p.next()
	if _, ok := p.expect("DELIMITER", "(", "esperava '(' depois de '"+keyword.Value+"'"); !ok {
		return nil
	}

	stmt := &ForStmt{pos: at(keyword)}

	if !p.check("DELIMITER", ";") {
		if stmt.Init = p.parseSimpleStatement(); stmt.Init == nil {
			return nil
		}
	}
	if _, ok := p.expect("DELIMITER", ";", "esperava ';' depois da inicializacao do 'jornada'"); !ok {
		return nil
	}

	if !p.check("DELIMITER", ";") {
		if stmt.Cond = p.parseExpr(); stmt.Cond == nil {
			return nil
		}
	}
	if _, ok := p.expect("DELIMITER", ";", "esperava ';' depois da condicao do 'jornada'"); !ok {
		return nil
	}

	// O passo roda a cada volta: declarar variavel ali nao faz sentido, e
	// parseSimpleStatement aceitaria, porque serve tambem a inicializacao. A
	// declaracao e lida mesmo assim, para a analise seguir no ')' e nao
	// recomecar no meio do cabecalho.
	if p.checkConcept("var") {
		p.fail(p.peek(), "o passo do 'jornada' nao pode declarar variavel; use uma atribuicao")
		if p.parseVarDecl(false) == nil {
			return nil
		}
	} else if !p.check("DELIMITER", ")") {
		if stmt.Post = p.parseSimpleStatement(); stmt.Post == nil {
			return nil
		}
	}
	if _, ok := p.expect("DELIMITER", ")", "esperava ')' fechando o cabecalho do 'jornada'"); !ok {
		return nil
	}

	if stmt.Body = p.parseBlock(); stmt.Body == nil {
		return nil
	}
	return stmt
}

// Retorno -> 'rescindir' Expr? ';'
func (p *parser) parseReturn() Node {
	keyword := p.next()

	stmt := &ReturnStmt{pos: at(keyword)}
	if !p.check("DELIMITER", ";") {
		if stmt.Value = p.parseExpr(); stmt.Value == nil {
			return nil
		}
	}
	if _, ok := p.expect("DELIMITER", ";", "esperava ';' no fim do 'rescindir'"); !ok {
		return nil
	}
	return stmt
}

// Falta -> 'falta' ';'
func (p *parser) parseBreak() Node {
	keyword := p.next()
	if _, ok := p.expect("DELIMITER", ";", "esperava ';' depois de 'falta'"); !ok {
		return nil
	}
	return &BreakStmt{pos: at(keyword)}
}

// Saida -> 'bater_ponto' '(' Args ')' ';'
func (p *parser) parsePrint() Node {
	keyword := p.next()
	if _, ok := p.expect("DELIMITER", "(", "esperava '(' depois de '"+keyword.Value+"'"); !ok {
		return nil
	}
	args, ok := p.parseArgs(keyword.Value)
	if !ok {
		return nil
	}
	if _, ok := p.expect("DELIMITER", ";", "esperava ';' depois de '"+keyword.Value+"(...)'"); !ok {
		return nil
	}
	return &PrintStmt{pos: at(keyword), Args: args}
}

// ---------------------------------------------------------------------------
// Expressoes
// ---------------------------------------------------------------------------

// precedence e a cascata Igualdade -> Comparacao -> Soma -> Produto, do nivel
// que amarra menos para o que amarra mais. Cada nivel usa o mesmo laco, so
// mudam os operadores aceitos, e todos sao associativos a esquerda.
var precedence = [][]string{
	{"==", "!="},
	{"<", ">", "<=", ">="},
	{"+", "-"},
	{"*", "/"},
}

func (p *parser) parseExpr() Node { return p.parseBinary(0) }

func (p *parser) parseBinary(level int) Node {
	if level == len(precedence) {
		return p.parseUnary()
	}

	left := p.parseBinary(level + 1)
	if left == nil {
		return nil
	}
	for p.peek().Type == "OPERATOR" && slices.Contains(precedence[level], p.peek().Value) {
		operator := p.next()
		right := p.parseBinary(level + 1)
		if right == nil {
			return nil
		}
		left = &BinaryExpr{pos: at(operator), Op: operator.Value, Left: left, Right: right}
	}
	return left
}

// Unario -> '-' Unario | Primario
func (p *parser) parseUnary() Node {
	if p.check("OPERATOR", "-") {
		operator := p.next()
		operand := p.parseUnary()
		if operand == nil {
			return nil
		}
		return &UnaryExpr{pos: at(operator), Op: operator.Value, Operand: operand}
	}
	return p.parsePrimary()
}

// Primario -> literal | IDENT | Chamada | 'reclamacao' '(' ')' | '(' Expr ')'
func (p *parser) parsePrimary() Node {
	token := p.peek()

	switch token.Type {
	case "INT", "FLOAT", "STRING", "BOOL_LITERAL":
		p.next()
		return &Literal{pos: at(token), Kind: token.Type, Value: token.Value}

	case "IDENTIFIER":
		p.next()
		if p.check("DELIMITER", "(") {
			return p.parseCall(token)
		}
		return &Ident{pos: at(token), Name: token.Value}

	case "DELIMITER":
		if token.Value == "(" {
			p.next()
			inner := p.parseExpr()
			if inner == nil {
				return nil
			}
			if _, ok := p.expect("DELIMITER", ")", "esperava ')' fechando a expressao"); !ok {
				return nil
			}
			return inner
		}

	case "KEYWORD":
		// reclamacao e palavra reservada, nao identificador, entao a chamada
		// dela nao cai no caso de IDENTIFIER acima.
		switch token.Concept {
		case "print":
			// bater_ponto e comando (ver parsePrint): nao devolve valor, entao
			// nao pode aparecer onde se espera uma expressao.
			p.fail(token, "'"+token.Value+"' e um comando e nao devolve valor; use-o sozinho, terminado em ';'")
			return nil
		case "input":
			p.next()
			if _, ok := p.expect("DELIMITER", "(", "esperava '(' depois de '"+token.Value+"'"); !ok {
				return nil
			}
			if _, ok := p.expect("DELIMITER", ")", "'"+token.Value+"' nao recebe argumentos; esperava ')'"); !ok {
				return nil
			}
			return &InputExpr{pos: at(token)}
		}
	}

	p.errorAt(token, "esperava uma expressao (numero, texto, identificador ou '(')")
	return nil
}

// Chamada -> nome '(' Args ')', com o '(' ja confirmado por quem chamou.
func (p *parser) parseCall(name Token) Node {
	p.next() // '('

	args, ok := p.parseArgs(name.Value)
	if !ok {
		return nil
	}
	return &CallExpr{pos: at(name), Name: name.Value, Args: args}
}

// Args -> vazio | Expr (',' Expr)*
// Chamado com o '(' ja consumido; consome ate o ')' inclusive. Compartilhado
// por chamada de funcao e bater_ponto.
func (p *parser) parseArgs(name string) ([]Node, bool) {
	var args []Node
	if !p.check("DELIMITER", ")") {
		for {
			arg := p.parseExpr()
			if arg == nil {
				return nil, false
			}
			args = append(args, arg)
			if !p.accept("DELIMITER", ",") {
				break
			}
		}
	}
	if _, ok := p.expect("DELIMITER", ")", "esperava ')' fechando a chamada de '"+name+"'"); !ok {
		return nil, false
	}
	return args, true
}

func at(token Token) pos { return pos{Line: token.Line, Column: token.Column} }
