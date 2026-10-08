package main

import "strings"

type Node interface {
	Pretty(indent string) string
}

type Program struct {
	Decls []Node
}

func (n *Program) Pretty(indent string) string {
	return head(indent, "program", n.Decls...)
}

type FuncDecl struct {
	Name   string
	Params []*Param
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
	Type string
	Name string
}

func (n *Param) Pretty(indent string) string {
	return line(indent, "param "+n.Type+" "+n.Name)
}

type VarDecl struct {
	Type string
	Name string
	Init Node
}

func (n *VarDecl) Pretty(indent string) string {
	title := "var " + n.Type + " " + n.Name
	if n.Init == nil {
		return line(indent, title)
	}
	return head(indent, title, n.Init)
}

type Block struct {
	Decls []Node
}

func (n *Block) Pretty(indent string) string {
	return head(indent, "block", n.Decls...)
}

type IfStmt struct {
	Cond    Node
	Then    Node
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

type ReturnStmt struct {
	Value Node
}

func (n *ReturnStmt) Pretty(indent string) string {
	if n.Value == nil {
		return line(indent, "return")
	}
	return head(indent, "return", n.Value)
}

type PrintStmt struct {
	Args []Node
}

func (n *PrintStmt) Pretty(indent string) string {
	return head(indent, "print", n.Args...)
}

type InputStmt struct {
	Args []Node
}

func (n *InputStmt) Pretty(indent string) string {
	return head(indent, "input", n.Args...)
}

type BreakStmt struct{}

func (n *BreakStmt) Pretty(indent string) string {
	return line(indent, "break")
}

type ExprStmt struct {
	Expr Node
}

func (n *ExprStmt) Pretty(indent string) string {
	if n.Expr == nil {
		return line(indent, "expr")
	}
	return n.Expr.Pretty(indent)
}

type Assign struct {
	Name  string
	Value Node
}

func (n *Assign) Pretty(indent string) string {
	return head(indent, "=", &Ident{Name: n.Name}, n.Value)
}

type Binary struct {
	Op    string
	Left  Node
	Right Node
}

func (n *Binary) Pretty(indent string) string {
	return head(indent, n.Op, n.Left, n.Right)
}

type Unary struct {
	Op   string
	Expr Node
}

func (n *Unary) Pretty(indent string) string {
	return head(indent, n.Op, n.Expr)
}

type Call struct {
	Name string
	Args []Node
}

func (n *Call) Pretty(indent string) string {
	return head(indent, "call "+n.Name, n.Args...)
}

type Ident struct {
	Name string
}

func (n *Ident) Pretty(indent string) string {
	return line(indent, "ident "+n.Name)
}

type Literal struct {
	Kind  string
	Value string
}

func (n *Literal) Pretty(indent string) string {
	return line(indent, n.Kind+" "+n.Value)
}

func line(indent, text string) string {
	return indent + text + "\n"
}

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
