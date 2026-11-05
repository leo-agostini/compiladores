# Análise semântica

A análise semântica está em [semantic.go](semantic.go). Ela recebe a AST de [parser.go](parser.go) e responde se o significado fecha: nome declarado, tipo compatível, escopo respeitado, chamada com a assinatura certa.

`Analyze(root)` devolve todas as mensagens. Um erro não interrompe a varredura. O `main` só chama `Analyze` quando o parser não reportou nada. Numa árvore pela metade, um nome que o parser não leu pareceria "não declarado".

A linguagem está em [linguagem.md](linguagem.md). A forma de cada nó está em [parser.md](parser.md).

---

## Duas direções na árvore

Declaração é pré-ordem: o nome entra na tabela antes dos usos que vêm depois dele no fonte.

Expressão é pós-ordem: os filhos devolvem um tipo, e o operador calcula o tipo resultante. Esse tipo sobe como valor de retorno de `analyze`. A árvore não é reescrita.

O escopo atual e o tipo de retorno da função descem para os filhos. São o contexto. O tipo da expressão sobe. É o atributo sintetizado.

## Tabela de símbolos

Cada escopo é um `SymbolTable`: um mapa e o ponteiro `parent` para o escopo de fora. Entrar num bloco cria uma tabela nova. Sair volta para `parent`. A busca começa no mapa local e sobe até a raiz. Isso é a pilha de escopos da aula, com o elo guardado no `parent` em vez de num vetor separado.

A inserção olha só o mapa local. O mesmo nome lá dentro é duplicata. O mesmo nome num bloco de dentro sombreia o de fora, e isso é válido.

```clt
carteira int x = 1;

void f() {
    carteira int x = 2;   /* sombreia o x de fora */
    {
        carteira int y = x + 1;
    }
    /* y não existe mais aqui */
}
```

Uma entrada é um `Symbol`:

| Campo | Variável | Parâmetro | Função |
|---|---|---|---|
| `Name` | nome no fonte | nome no fonte | nome no fonte |
| `Kind` | `"var"` | `"param"` | `"func"` |
| `Type` | tipo da variável | tipo do parâmetro | tipo de retorno |
| `Params` | vazio | vazio | tipo de cada parâmetro, na ordem |
| `Scope` | rótulo do escopo | rótulo do escopo | rótulo do escopo |
| `Line`, `Column` | posição do nome | posição do nome | posição do nome |

Variável, parâmetro e função dividem o mesmo espaço de nomes. `x = 1` e `x` exigem que o símbolo não seja função. `x(...)` exige `Kind == "func"`.

O rótulo do escopo é `global` no arquivo, `local (f)` nos parâmetros e no corpo da função `f`, e `aninhado (f)` num `{ }` ou `jornada` dentro dela. Fora de função, um bloco é só `aninhado`.

## Tabela impressa

Cada símbolo inserido também vai para uma lista, que sobrevive ao fim do escopo. `Analyze` devolve essa lista na ordem do fonte, e o `main` a imprime depois da árvore, mesmo quando há erro semântico. Para o exemplo do slide 19:

```text
NOME  CATEGORIA  TIPO   ESCOPO           POSICAO
gg    variavel   int    global           5:14
main  funcao     int()  global           7:5
a     variavel   int    local (main)     8:18
b     variavel   int    local (main)     9:18
c     variavel   int    aninhado (main)  11:22
```

Função mostra a assinatura no tipo: `float(int, int)`.

## Testes

```text
go test ./...
```

[semantica.clt](../semantica.clt) é um programa válido: o teste exige zero erros e confere as entradas da tabela, incluindo a do slide 19.

[semantica_erros.clt](../semantica_erros.clt) tem um erro por verificação. Cada linha que deve falhar termina com `// espera: <mensagem>`; duas mensagens na mesma linha se separam por ` | `. O teste exige exatamente esses erros, sem faltar nem sobrar. Para cobrir um caso novo, basta acrescentar a linha e o comentário.

## Onde cada escopo abre

| Lugar | O que entra nele |
|---|---|
| arquivo | variáveis e funções globais. É a tabela inicial |
| `{ }` solto | o que está escrito dentro do bloco |
| função | os parâmetros e o corpo, no mesmo escopo. O `{ }` da função não abre outra tabela |
| `for` | o init, a condição, o passo e o corpo. A variável do `jornada (carteira int i = ...)` morre quando o `for` acaba |
| `if` / `while` | não abrem escopo. O bloco escrito no corpo é que abre |

O corpo da função fica no mesmo escopo dos parâmetros de propósito. `void f(int x) { carteira int x = 1; }` é duplicata, não sombreamento.

Um `{ }` dentro da função abre escopo novo, e aí o sombreamento vale.

## Funções antes do corpo

Em cada lista de declarações (o arquivo, um bloco, o corpo de uma função), as assinaturas entram na tabela antes de qualquer uso ser analisado. Sem isso, `fat` não estaria na tabela na chamada recursiva, nem `bar` nesta ordem:

```clt
void foo() { bar(); }
void bar() { foo(); }
```

Parâmetro `void`, ou sem tipo, entra na assinatura como `error`. A chamada não gera um segundo erro de tipo por causa disso. O erro do parâmetro sai uma vez, na definição.

## Tipos

| Nome | Papel |
|---|---|
| `int`, `float`, `string`, `bool`, `void` | os tipos da linguagem |
| `error` | recuperação. A expressão que já falhou devolve `error`, e quem está acima aceita esse tipo sem nova mensagem |

`int` cabe onde se espera `float`. `float` não cabe onde se espera `int`. A aula trata a primeira conversão como segura. A árvore não ganha um nó de conversão: o tipo que sobe já é o resultado.

`+`, `-`, `*` e `/` exigem número dos dois lados. Se um lado é `float`, o resultado é `float`. Senão é `int`. `int / int` continua `int`.

`-` unário exige número e devolve o mesmo tipo.

`<`, `>`, `<=`, `>=` exigem número e devolvem `bool`.

`==` e `!=` devolvem `bool`. Aceitam os dois lados iguais (`bool` com `bool`, `string` com `string`) ou os dois numéricos (`int` com `float`).

`+` entre strings é erro. Não há concatenação.

A condição de `if`, `else if`, `while` e `for` tem de ser `bool`. `jornada (;;)` não tem condição, e isso é o laço sem teste, não um erro de tipo.

## Cada verificação

| Situação | Mensagem |
|---|---|
| nome repetido no mesmo escopo | `'x' ja foi declarado neste escopo (linha 3)`, com a linha da primeira declaração |
| variável `void` | `variavel 'x' nao pode ser void` |
| inicialização com outro tipo | `tipos incompativeis na declaracao de 'w': esperado int, encontrado bool` |
| uso de nome inexistente | `variavel 'z' nao declarada` |
| nome de função usado como valor | `'foo' e uma funcao; chame com 'foo(...)'` |
| atribuição com outro tipo | `tipos incompativeis na atribuicao de 'x': esperado int, encontrado float` |
| operador no tipo errado | `operacao '+' invalida entre string e string` |
| menos num não-número | `operador '-' invalido para bool` |
| condição que não é `bool` | `condicao do if deve ser bool, encontrado int` |
| chamada de nome inexistente | `funcao 'bar' nao declarada` |
| chamada de variável | `'horas' nao e uma funcao` |
| argumento com tipo errado | `chamada de 'soma': argumento 2 esperado int, encontrado string` |
| quantidade de argumentos | `chamada de 'soma': esperado 2 argumento(s), encontrado 1` |
| parâmetro repetido | `parametro 'x' repetido (linha 1)` |
| parâmetro `void` | `parametro 'v' nao pode ser void` |
| `return` fora de função | `return fora de funcao` |
| valor numa função `void` | `return com valor em funcao void` |
| `return` vazio numa função com tipo | `return sem valor em funcao int` |
| valor de outro tipo | `return incompativel: esperado int, encontrado string` |
| `void` lido como valor | `expressao void nao pode ser usada como valor` |
| `break` fora de laço | `break fora de laco` |

Toda mensagem com posição sai como `linha N, col M: ...`. No fim, as mensagens são ordenadas por linha e coluna. As funções são registradas antes do resto do escopo; sem essa ordenação, a duplicata de uma função no fim do arquivo apareceria antes do erro da primeira linha.

`avisar();` como comando pode ser `void`. `x = avisar()` não pode: alguém leu o resultado.

Depois de "não declarado", a expressão vira `error`. A atribuição que usa essa expressão não ganha um segundo "tipos incompatíveis".

## Prontas da linguagem

`bater_ponto` e `print` não entram na tabela: são palavra reservada. Aceitam qualquer quantidade de argumentos, cada um um valor, e o resultado é `void`.

`reclamacao` e `input` também. Devolvem `string`, que é o uso `nome = reclamacao()`. Os argumentos, se houver, são valores de qualquer tipo.

## O que esta fase não faz

Não exige `return` em todo caminho da função. Não avalia constante, então `1 / 0` não é erro daqui. Não reescreve a AST com conversão `int` para `float`. Não olha o fonte de novo: a linha e a coluna vêm do `Pos` gravado no nó.
