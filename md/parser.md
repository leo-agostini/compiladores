# Parser e AST

O parser está em [parser.go](parser.go). Ele consome a fatia de [tokenizer.go](tokenizer.go) e devolve um `*Program` mais a lista de erros. A árvore sai mesmo com erro, para ainda dar para imprimir o que foi montado.

`Parse(tokens)` é a entrada. Não há token de fim: `Type == ""` encerra o arquivo.

A linguagem está em [linguagem.md](linguagem.md). O que a árvore significa está em [semantica.md](semantica.md).

---

## Como a descida funciona

```text
program → decl*
decl    → func | var | stmt
stmt    → block | if | for | while | return | print | input | break | expr ;
expr    → assign → equality → comparison → term → factor → unary → primary
```

Um comando no topo do arquivo é válido. `caso` não precisa estar dentro de função.

`var` e função só entram por `parseDecl`. O `then` de um `if` passa por `parseStmt`, então isto é erro de sintaxe:

```clt
caso (x) carteira int y = 1;
```

A variável precisa do bloco:

```clt
caso (x) { carteira int y = 1; }
```

Comentário some em `skipComments`, chamado por `peek`, `peekNext` e `next`. O resto do parser não vê `COMMENT`.

## As duas funções

`void avisar(string mensagem) { ... }` — o tipo vem antes do nome.

`contratar avisar(string mensagem) void { ... }` — a palavra `function` vem antes, e o tipo vem depois do `)`. Sem esse tipo, `Return` fica vazio. A análise semântica trata vazio como `void`.

O nó é o mesmo `FuncDecl`. Muda só a ordem em que o tipo aparece no fonte.

## Precedência

Da mais frouxa para a mais apertada:

| Nível | Operadores | Associação |
|---|---|---|
| atribuição | `=` | direita: `a = b = 1` |
| igualdade | `==` `!=` | esquerda |
| comparação | `<` `>` `<=` `>=` | esquerda |
| termo | `+` `-` | esquerda |
| fator | `*` `/` | esquerda |
| unário | `-` prefixo | — |
| primário | literal, nome, chamada, `(expr)` | — |

`1+2+3` vira `(+ (+ 1 2) 3)`. `-1 * 2` vira `(* (- 1) 2)`, porque o menos mora em `parseUnary`, não no literal.

`=` só casa quando o token atual é `IDENTIFIER` e o próximo é `=`. Sem olhar adiante, `taxa = 1` viraria um nome solto e um erro no `=`.

## Nós da árvore

Todo nó implementa `Node` com `Pretty`. `Pos` (`Line`, `Column`) marca o token que originou o nó, para a análise semântica citar o lugar. Nó montado no meio de um erro pode ficar com posição zero.

`Pretty("")` é o que o `main` imprime. Filho nulo aparece como `?`, para um pedaço que falhou não sumir da árvore.

| Nó | Fonte | Campos |
|---|---|---|
| `Program` | o arquivo | `Decls` — função, variável e comando misturados |
| `FuncDecl` | as duas formas de função | `Name`, `Params`, `Return` (o `Concept` do tipo), `Body` |
| `Param` | `int base` | `Type`, `Name` |
| `VarDecl` | `carteira T nome = expr;` | `Type`, `Name`, `Init` (nulo se não houver `=`) |
| `Block` | `{ ... }` | `Decls`, de novo por `parseDecl` |
| `IfStmt` | `caso` / `if` | `Cond`, `Then`, `ElseIfs`, `Else` |
| `ElseIf` | `recurso`, `senao caso`, `else if` | `Cond`, `Then`. Não é um `IfStmt` dentro do `else` |
| `ForStmt` | `jornada` / `for` | `Init`, `Cond`, `Post`, `Body`. Qualquer um dos três primeiros pode faltar |
| `WhileStmt` | `plantao` / `while` | `Cond`, `Body` |
| `ReturnStmt` | `rescindir` / `return` | `Value`, nulo no retorno vazio |
| `PrintStmt` | `bater_ponto(...);` | `Args` |
| `InputStmt` | `reclamacao(...);` | `Args` |
| `BreakStmt` | `falta;` | só a posição |
| `ExprStmt` | expressão seguida de `;` | `Expr`. O `Pretty` mostra o filho direto |
| `Assign` | `nome = expr` | `Name`, `Value` |
| `Binary` | operador infixo | `Op` (o lexema, `"+"` ou `"=="`), `Left`, `Right` |
| `Unary` | `-` prefixo | `Op`, `Expr` |
| `Call` | `nome(args)` | `Name` (o lexema), `Args` |
| `Ident` | nome numa expressão | `Name` |
| `Literal` | número, string, bool | `Kind` (`int`, `float`, `string`, `bool`), `Value` |

`print` e `input` no início de um comando viram `PrintStmt` e `InputStmt`, e esse comando consome o `;`. No meio de uma expressão (`nome = reclamacao()`) viram `Call`, com o lexema em `Name`.

O bool guarda o `Concept` (`true` ou `false`), não `registrado` nem `pj`. Os outros literais guardam o lexema (`44`, `1.5`, `"Ana"`).

O `for` com `carteira` no init não pede outro `;`: `parseVarDecl` já consumiu o da declaração. `jornada (;;)` deixa `Init`, `Cond` e `Post` nulos.

`else` ou `else if` sem `if` é erro. O parser não tenta encaixar esse ramo em outro comando.

## Erro e recuperação

`panicMode` silencia os `expect` seguintes no mesmo ponto. Sem isso, um token ruim geraria uma frase por não-terminal.

`synchronize` anda até um lugar seguro e desliga o `panicMode`:

- consome um `;`
- para em `{` ou `}`, que o bloco ainda precisa ler
- para no começo de uma declaração ou comando (`TYPE`, `function`, `var`, `if`, `for`, `while`, `return`, `print`, `input`, `break`, ou um nome seguido de `=`, `(` ou `;`)

Um nome no meio de `bater_ponto(a b)` não conta como declaração nova. Parar ali jogaria fora o argumento e criaria outro erro no `)`.

Token `ERROR` reaproveita a `Message` do lexer. Fim da entrada usa a posição do último token visível e acrescenta `(fim da entrada)`.

O formato é o mesmo das outras fases:

```text
linha 12, col 5: esperado ';', encontrado "carteira"
```
