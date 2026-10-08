# Como o compilador CLT funciona

Este documento explica cada peca do front-end da linguagem CLT: o que cada arquivo faz,
como os dados passam de uma fase para a outra e por que as decisoes foram tomadas assim.
Os detalhes de cada regex do lexer estao em [regex.md](regex.md); aqui o foco e a
estrutura.

---

## 1. Visao geral

```
arquivo .clt ──► tokenize() ──► []Token ──► parse() ──► *Program (AST) ──► printTree()
                 lexico                      sintatico
```

| Arquivo | Papel |
|---|---|
| [entrypoint.go](entrypoint.go) | `main`: le o arquivo, roda as duas fases, imprime tokens, erros e a arvore |
| [lexical-analysis.go](lexical-analysis.go) | analise lexica: transforma o texto em uma lista de `Token` |
| [syntax-analysis.go](syntax-analysis.go) | analise sintatica: define a AST e o parser descendente recursivo |
| [regex.md](regex.md) | documentacao caractere a caractere dos padroes do lexer |
| [testes/](testes) | programas de exemplo, validos e com erros |

Para rodar:

```
go run . testes/teste.clt
```

Sem argumento, o programa usa `testes/teste.clt`. A saida de sucesso tem duas partes: a
tabela de tokens (`== analise lexica ==`) e a arvore sintatica (`== analise sintatica ==`).
O codigo de saida e `1` quando ha qualquer erro.

---

## 2. O ponto de entrada (`entrypoint.go`)

`main` executa as fases em ordem, e cada fase so roda se a anterior terminou limpa:

1. **Leitura.** `os.ReadFile` no caminho recebido.
2. **Analise lexica.** `tokenize` devolve todos os tokens, inclusive os de erro. Cada
   token e impresso numa linha (`linha, col | lexema -> TIPO (conceito)`).
3. **Barreira de erros lexicos.** Se algum token tem `Type == "ERROR"`, os erros sao
   listados e o programa para ali. Um token `ERROR` nao encaixa em nenhuma regra da
   gramatica, entao seguir so produziria erros sintaticos derivados do erro lexico.
4. **Analise sintatica.** `parse` devolve o programa e a lista de `SyntaxError`. Com
   erros, eles sao listados e o programa sai com `1`; sem erros, a arvore e impressa.

Duas funcoes auxiliares:

- `display` achata um lexema numa linha so e corta em 22 caracteres. Sem isso, um
  comentario de bloco de varias linhas quebraria a tabela. A AST tambem usa `display`
  para mostrar literais.
- `printTree` percorre a AST pela interface `Node` (`Label` + `Children`), dois
  espacos por nivel. Como ela so conhece a interface, um no novo aparece na saida sem
  mexer no printer.

---

## 3. Analise lexica (`lexical-analysis.go`)

### 3.1. O token

```go
type Token struct {
	Type    string // COMMENT, IDENTIFIER, KEYWORD, TYPE, BOOL_LITERAL, INT, FLOAT,
	               // STRING, OPERATOR, DELIMITER ou ERROR
	Concept string // papel da palavra reservada ("if", "while"...); vazio nos demais
	Value   string // o lexema exatamente como esta no fonte
	Line    int    // 1-based
	Column  int    // 1-based, contada em caracteres (runes), nao em bytes
	Message string // so em ERROR: explica o que esta errado
}
```

O campo que mais importa para o parser e `Concept`. A linguagem tem sinonimos (`caso` e
`if`, `bater_ponto` e `print`...), e o lexer resolve isso uma vez so: os dois viram
`KEYWORD` com o mesmo `Concept`. O parser compara conceitos, nunca lexemas, entao a
tabela de sinonimos existe em um lugar so.

### 3.2. A tabela de regras

`rules` e uma lista ordenada de pares (tipo, regex). Cada regex e envolvida por
`anchored`, que prende o padrao no inicio do texto restante. A cada passo, o laco tenta
as regras de cima para baixo e **a primeira que casar vence**. Por isso a ordem e parte
da especificacao:

- operadores de dois caracteres vem antes dos de um (`==` antes de `=`);
- `FLOAT` vem antes de `INT` (senao `3.14` viraria `3` seguido de lixo);
- as regras de erro moram na mesma tabela. Um lexema malformado e consumido inteiro
  por um padrao proprio, que traz a mensagem certa e evita a cascata (`1.2.3` vira um
  erro so, e nao `1.2` mais um ponto mais `3`).

O Go usa RE2, que nao tem lookahead. Onde seria preciso dizer "casa X mas nao Y", a
regra valida vem primeiro e a de erro fica so com o que sobrou (`!=` e OPERATOR; o `!`
que sobra sozinho vira ERROR). Todas as 27 regras, uma por uma, estao em
[regex.md](regex.md).

### 3.3. Palavras reservadas

O lexer nao tem um regex por palavra-chave. Toda palavra casa primeiro como `WORD`
(`[a-zA-Z_][a-zA-Z0-9_]*`) e so depois e procurada no mapa `reserved`. Assim `casos`
continua sendo IDENTIFIER (e nao `caso` + `s`), e nenhuma reservada pode ser usada como
nome de variavel.

Todo conceito tem um nome em portugues e um em ingles:

| Conceito | Portugues | Ingles | Tipo do token |
|---|---|---|---|
| if | `caso` | `if` | KEYWORD |
| else if | `recurso`, `senao caso` | `else if` | KEYWORD |
| else | `senao` | `else` | KEYWORD |
| for | `jornada` | `for` | KEYWORD |
| while | `plantao` | `while` | KEYWORD |
| function | `contratar` | `function` | KEYWORD |
| return | `rescindir` | `return` | KEYWORD |
| print | `bater_ponto` | `print` | KEYWORD |
| input | `reclamacao` | `input` | KEYWORD |
| var | `carteira` | `var` | KEYWORD |
| break | `falta` | `break` | KEYWORD |
| true | `registrado` | `true` | BOOL_LITERAL |
| false | `pj` | `false` | BOOL_LITERAL |

Os tipos `int`, `float`, `string`, `bool` e `void` viram `TYPE`.

### 3.4. O laco de `tokenize`

A cada iteracao, `rest := code[pos:]` e o texto que ainda falta. Em ordem:

1. **Espaco em branco** e consumido sem gerar token. As quebras de linha dentro dele
   avancam o contador `line`.
2. **Regras.** A primeira que casar gera o token. Em seguida ha tres ajustes:
   - `WORD` e reclassificado como IDENTIFIER, KEYWORD, TYPE ou BOOL_LITERAL;
   - `STRING` passa por `invalidEscape`. O regex aceita qualquer caractere depois de
     `\`, entao so da para recusar `\z` olhando o lexema ja casado. Escapes validos:
     `\n \t \r \0 \" \\`;
   - **fusao do `else if`.** Um `caso`/`if` cujo token significativo anterior e um
     `senao`/`else` nao vira token novo: o anterior passa a ter conceito `"else if"`
     e lexema `"senao caso"`. Assim `recurso` (uma palavra) e `senao caso` (duas)
     chegam ao parser iguais. `lastMeaningful` pula comentarios, entao
     `senao /* x */ caso` tambem funde.
3. **Caractere desconhecido.** Se nenhuma regra casou, o caractere vira um ERROR com
   mensagem especifica quando ela existe em `unknownChar` (`@`, `#`, `[`, `?`...), ou
   generica. O laco consome uma **rune** inteira, para que `ç` gere um erro so, e nao
   dois bytes quebrados.

O laco nunca para no primeiro erro: todos os problemas lexicos do arquivo saem numa
passada so.

### 3.5. Linha e coluna

- `line` e um contador. Todo lexema que pode conter quebra de linha (comentario de bloco,
  a string quebrada em duas linhas) soma suas `\n` depois de consumido.
- A coluna e calculada sob demanda por `columnOf`: conta as runes entre o ultimo `\n` e
  a posicao. Contar runes, e nao bytes, mantem a coluna certa depois de um caractere
  acentuado dentro de string ou comentario.

---

## 4. Analise sintatica (`syntax-analysis.go`)

O parser e **descendente recursivo**: uma funcao por regra da gramatica, e cada funcao
consome os tokens da sua regra e devolve o no da AST correspondente.

### 4.1. A gramatica

```
Programa       -> Topo*
Topo           -> FuncDecl | Comando

FuncDecl       -> TYPE IDENT '(' Params ')' Bloco
                | 'contratar' IDENT '(' Params ')' TYPE? Bloco      (sem TYPE = void)
Params         -> vazio | TYPE IDENT (',' TYPE IDENT)*

Bloco          -> '{' Comando* '}'

Comando        -> VarDecl | Se | Enquanto | Para | Retorno | Falta | Saida | Bloco
                | ComandoSimples ';'
ComandoSimples -> VarDecl-sem-';' | Atribuicao | Expr

VarDecl        -> 'carteira' TYPE IDENT ('=' Expr)? ';'
Atribuicao     -> IDENT '=' Expr
Se             -> 'caso' '(' Expr ')' Bloco
                  ('senao caso' '(' Expr ')' Bloco)*
                  ('senao' Bloco)?
Enquanto       -> 'plantao' '(' Expr ')' Bloco
Para           -> 'jornada' '(' ComandoSimples? ';' Expr? ';' (Atribuicao | Expr)? ')' Bloco
Retorno        -> 'rescindir' Expr? ';'
Falta          -> 'falta' ';'
Saida          -> 'bater_ponto' '(' Args ')' ';'

Expr           -> Igualdade
Igualdade      -> Comparacao (('==' | '!=') Comparacao)*
Comparacao     -> Soma (('<' | '>' | '<=' | '>=') Soma)*
Soma           -> Produto (('+' | '-') Produto)*
Produto        -> Unario (('*' | '/') Unario)*
Unario         -> '-' Unario | Primario
Primario       -> INT | FLOAT | STRING | BOOL_LITERAL
                | IDENT | IDENT '(' Args ')'
                | 'reclamacao' '(' ')'
                | '(' Expr ')'
Args           -> vazio | Expr (',' Expr)*
```

Onde a gramatica cita uma palavra em portugues, o sinonimo em ingles vale igual, porque
o parser compara conceitos.

Pontos que valem notar:

- **Nao existe `main`.** O corpo do programa sao comandos soltos no topo do arquivo.
  Funcao so pode ser declarada no topo, nunca dentro de um bloco.
- **Variavel sempre comeca com `carteira`.** Um `TYPE` no inicio de um comando so pode
  abrir funcao, e o parser decide olhando tres tokens: `TYPE IDENT '('`.
- **Corpos sempre tem chaves.** `caso (x) falta;` nao e valido; `caso (x) { falta; }` e.
- **`bater_ponto` e comando, `reclamacao` e expressao.** Imprimir nao devolve valor,
  entao `x = bater_ponto(1);` e erro. Ler devolve, entao `nome = reclamacao();` e
  valido, e `reclamacao();` sozinho tambem (como expressao usada como comando).
- **Atribuicao e comando, nao expressao.** `x = y = 1` e `caso (x = 1)` sao erros.
- **Nao ha operadores logicos nem `!`.** O lexer ja recusa `&&`, `||` e `!`.
- Todos os operadores binarios sao associativos a esquerda: `1 - 2 - 3` e
  `(1 - 2) - 3`.

### 4.2. O estado do parser

```go
type parser struct {
	tokens []Token        // tokens sem os COMMENT
	pos    int            // indice do proximo token a consumir
	eof    Token          // sentinela de fim de arquivo
	errors []SyntaxError
}
```

`newParser` faz dois preparos:

- **remove os comentarios.** Eles sao tokens validos, mas nao fazem parte da gramatica;
  tirar uma vez evita ter que pula-los em cada regra.
- **monta a sentinela `EOF`**, posicionada logo depois do ultimo token. `peek` devolve
  ela quando a lista acaba, entao nenhuma regra precisa checar limite de slice, e o erro
  "encontrei o fim do arquivo" ainda tem linha e coluna.

### 4.3. Primitivas

| Funcao | O que faz |
|---|---|
| `peek()`, `peekAt(n)` | olha o token atual (ou `n` a frente) sem consumir |
| `next()` | consome e devolve o token atual |
| `check(tipo, valor)` | o token atual e desse tipo? `valor` vazio aceita qualquer lexema |
| `checkConcept(c)` | o token atual e uma KEYWORD com esse conceito? |
| `accept(tipo, valor)` | `check` e, se casou, consome |
| `expect(tipo, valor, msg)` | consome o esperado ou registra o erro e devolve `false` |
| `errorAt(tok, msg)` | registra `"<msg>, encontrei '<lexema>'"` na posicao de `tok` |
| `fail(tok, msg)` | registra a mensagem como esta, para quando o proprio token ja e a explicacao |

`expect` tem um ajuste de posicao: se o token encontrado ja esta numa linha depois da do
token anterior, o erro aponta para o **fim do token anterior**. E o caso tipico do `;`
esquecido: o erro sai na linha onde ele faltou, e nao na linha seguinte.

### 4.4. Como as regras se comunicam

Toda funcao `parseX` devolve o no construido, ou `nil` quando encontrou um erro, que ja
foi registrado. Quem chama ve o `nil` e devolve `nil` tambem, e o erro sobe ate o laco
mais proximo que sabe se recuperar: o de `parse` (topo do arquivo) ou o de `parseBlock`
(dentro de um bloco).

Nos opcionais ausentes ficam `nil` no campo (o `Else` de um `caso` sem `senao`, o `Value`
de `rescindir;`). A funcao `children` descarta esses `nil` na hora de listar os filhos,
para o printer nao receber uma interface que aponta para nada.

### 4.5. Recuperacao de erros (modo panico)

O parser nao para no primeiro erro. Quando uma regra devolve `nil`, o laco chama
`synchronize`, que descarta tokens ate um ponto onde a analise pode recomecar:

- **depois** de um `;` (o comando quebrado terminou);
- **em cima** de `{` ou `}` (o bloco ainda precisa deles para fechar certo);
- **em cima** do inicio de um comando: `carteira`, `caso`, `plantao`, `jornada`,
  `rescindir`, `falta`, `bater_ponto` ou `contratar`;
- **em cima** de um cabecalho de funcao `TYPE IDENT '('`.

Duas escolhas sao de proposito:

- `reclamacao` **nao** e ponto de parada. Ela costuma aparecer no meio de uma expressao,
  e parar nela recomecaria a analise no meio do comando quebrado, gerando um segundo
  erro derivado do primeiro.
- Um `TYPE` sozinho tambem nao e. Tipos aparecem em listas de parametros e depois de
  `carteira`; so o padrao completo de cabecalho de funcao reinicia a analise.

Os lacos de `parse` e `parseBlock` tem ainda uma **garantia de progresso**: se depois de
sincronizar o parser continua no mesmo token, ele consome um token a forca. E uma rede de
seguranca: se uma regra falhar sem consumir nada justamente num token em que
`synchronize` para, o laco tentaria a mesma regra no mesmo token para sempre.

Alguns erros sao tratados para nao virar cascata:

| Situacao | Tratamento |
|---|---|
| `}` sobrando no topo do arquivo | erro "`'}'` sem um `'{'` correspondente" e o token e consumido |
| `senao` / `senao caso` sem `caso` antes | erro dizendo que o bloco anterior provavelmente nao fechou |
| funcao declarada dentro de bloco | erro, mas a funcao e lida inteira para nao gerar um erro por pedaco |
| `carteira` no passo do `jornada` | erro, mas a declaracao e lida para a analise seguir no `)` |
| `bater_ponto` usado como valor | erro dizendo que ele e comando e nao devolve valor |

O resultado e um erro por engano. Em [testes/sintaxe.clt](testes/sintaxe.clt) ha dez
erros diferentes, e o parser reporta exatamente dez mensagens, cada uma na linha do seu
caso.

### 4.6. Expressoes e precedencia

A precedencia fica numa tabela, do nivel que amarra menos para o que amarra mais:

```go
var precedence = [][]string{
	{"==", "!="},
	{"<", ">", "<=", ">="},
	{"+", "-"},
	{"*", "/"},
}
```

`parseBinary(nivel)` e uma funcao so para todos os niveis: le o lado esquerdo no nivel
seguinte e, enquanto o operador atual pertencer ao seu nivel, consome o operador, le o
lado direito no nivel seguinte e encaixa um `BinaryExpr` a esquerda. Isso da a
associatividade a esquerda. Depois do ultimo nivel vem `parseUnary` (o `-` prefixo,
recursivo: `- -x` e `-(-x)`; escrito colado, `--` e recusado pelo lexer como
decremento) e entao `parsePrimary`.

Para adicionar um operador binario novo, basta incluir ele na linha certa da tabela (e
no regex de OPERATOR do lexer).

### 4.7. A AST

Todo no implementa:

```go
type Node interface {
	Label() string    // como o no aparece na arvore impressa
	Children() []Node // filhos, ja sem os nil
}
```

e quase todos embutem `pos` (linha e coluna de onde vieram), para a fase semantica poder
apontar erros no lugar certo.

| No | Vem de | Rotulo impresso | Filhos |
|---|---|---|---|
| `Program` | arquivo inteiro | `Programa` | declaracoes e comandos do topo |
| `FuncDecl` | funcao | `FuncDecl <tipo> <nome>` | `parametros`, `corpo` |
| `Param` | parametro | `Param <tipo> <nome>` | — |
| `VarDecl` | `carteira` | `VarDecl <tipo> <nome>` | valor inicial, se houver |
| `Block` | `{ ... }` solto | `Bloco` | comandos |
| `IfStmt` | `caso` | `Se (caso)` | `condicao`, `entao`, um `senao caso` por ramo, `senao` |
| `WhileStmt` | `plantao` | `Enquanto (plantao)` | `condicao`, `corpo` |
| `ForStmt` | `jornada` | `Para (jornada)` | `inicio`, `condicao`, `passo`, `corpo` (os presentes) |
| `ReturnStmt` | `rescindir` | `Retorno (rescindir)` | valor, se houver |
| `BreakStmt` | `falta` | `Interrupcao (falta)` | — |
| `PrintStmt` | `bater_ponto` | `Saida (bater_ponto)` | `argumentos` |
| `Assign` | `x = ...` | `Atribuicao <nome> =` | valor |
| `ExprStmt` | expressao usada como comando | `ExprComando` | a expressao |
| `BinaryExpr` | `a + b` etc. | `Binario <op>` | esquerda, direita |
| `UnaryExpr` | `-a` | `Unario -` | operando |
| `CallExpr` | `f(...)` | `Chamada <nome>` | `argumentos` |
| `InputExpr` | `reclamacao()` | `Entrada (reclamacao)` | — |
| `Literal` | numero, texto, booleano | `<TIPO> <lexema>` | — |
| `Ident` | nome numa expressao | `Ident <nome>` | — |

`PrintStmt` e `InputExpr` tem no proprio, em vez de virarem `CallExpr`, porque
`CallExpr` guarda o lexema: `print(x)` e `bater_ponto(x)` virariam chamadas a funcoes de
nomes diferentes, e a fase seguinte teria que conhecer os sinonimos de novo.

Os rotulos `condicao`, `entao`, `corpo`, `parametros`... nao sao nos da gramatica: sao
`group`, um no auxiliar que so da nome ao papel de um conjunto de filhos na impressao.
`groupOf` devolve `nil` quando o grupo ficaria vazio (uma funcao sem parametros nao
imprime `parametros`); `blockGroup` sempre imprime o rotulo de um bloco existente, mesmo
vazio, para que um `senao { }` continue visivel.

Exemplo, para

```
caso (horas >= 44) {
    avisar("hora extra");
} senao {
    falta;
}
```

a saida e

```
Se (caso)
  condicao
    Binario >=
      Ident horas
      INT 44
  entao
    ExprComando
      Chamada avisar
        argumentos
          STRING "hora extra"
  senao
    Interrupcao (falta)
```

---

## 5. O que fica para a analise semantica

O parser valida so a **forma** do programa. Tudo abaixo passa pela analise sintatica e
precisa ser checado numa fase seguinte, que pode usar o `pos` de cada no para apontar o
lugar:

- tipos: `carteira int x = "texto";`, operandos incompativeis, condicao que nao e `bool`;
- `carteira void x;` e parametro do tipo `void`;
- variavel ou funcao usada sem declarar, ou declarada duas vezes;
- numero de argumentos de uma chamada diferente do numero de parametros;
- `rescindir` fora de funcao, ou com/sem valor em desacordo com o tipo de retorno;
- `falta` fora de `plantao`/`jornada`;
- expressao sem efeito usada como comando (`1 + 2;`).

---

## 6. Os arquivos de teste

| Arquivo | O que exercita | Resultado esperado |
|---|---|---|
| [testes/teste.clt](testes/teste.clt) | programa completo tipico: variaveis, funcoes, `caso`/`senao`, `plantao`, chamadas | lexico e sintatico limpos; imprime a arvore |
| [testes/comuns.clt](testes/comuns.clt) | casos de canto validos: escapes, divisao colada a comentario, comentario no meio da declaracao, as tres grafias de `else if` | lexico e sintatico limpos |
| [testes/errors.clt](testes/errors.clt) | um exemplo de cada erro lexico | 26 erros lexicos; o parser nao roda |
| [testes/sintaxe.clt](testes/sintaxe.clt) | dez erros sintaticos com o lexico limpo | exatamente 10 erros sintaticos, um por caso |
