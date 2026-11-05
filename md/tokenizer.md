# Tokenizer

O tokenizer está em [tokenizer.go](tokenizer.go). Ele lê o fonte e devolve uma fatia de tokens, da esquerda para a direita. Não há token de fim de arquivo: o fim da fatia é o fim da entrada.

Cada padrão, caractere a caractere, está em [regex.md](regex.md). Este arquivo explica o que o lexer faz com o que casou.

A linguagem que esses tokens formam está em [linguagem.md](linguagem.md).

---

## O token

| Campo | Conteúdo |
|---|---|
| `Type` | classe: `IDENTIFIER`, `KEYWORD`, `TYPE`, `INT`, `FLOAT`, `STRING`, `BOOL_LITERAL`, `OPERATOR`, `DELIMITER`, `COMMENT`, `ERROR` |
| `Concept` | papel da palavra reservada (`if`, `else if`, `int`, `true`). Vazio no resto |
| `Value` | lexema, do jeito que está no fonte |
| `Line`, `Column` | posição, coluna contada em runes a partir de 1 |
| `Message` | texto do erro, só em `ERROR` |

`caso` e `if` chegam os dois como `KEYWORD` com `Concept` `if`. O parser decide pelo `Concept`, nunca pelo texto em português ou inglês. O `Value` continua sendo o que o programador escreveu, para a mensagem e para a árvore mostrarem `bater_ponto` quando foi isso que estava no fonte.

## A varredura

1. Espaço, tab e quebra de linha são pulados. Não viram token. A quebra de linha só avança o contador de linha.
2. A primeira regra que casa no texto restante vence. A ordem está na tabela `rules`.
3. `WORD` (`[a-zA-Z_][a-zA-Z0-9_]*`) é reclassificado: se está no mapa `reserved`, vira `KEYWORD`, `TYPE` ou `BOOL_LITERAL` e ganha `Concept`. Senão fica `IDENTIFIER`.
4. String com escape inválido, que o regex aceitou, é rebaixada para `ERROR` em `invalidEscape`.
5. `else` (ou `senao`) seguido de `if` (ou `caso`), sem outro token no meio, vira um token só com `Concept` `else if`. Espaço e quebra de linha não separam, porque não são token. Um comentário no meio separa, e a fusão não acontece. `recurso` já nasce como `else if`, numa palavra só.
6. O que não casa nenhuma regra vira um `ERROR` de um caractere, com a explicação de `unknownChar` quando ela existe.

Comentário é token. O parser é quem ignora `COMMENT`. Assim o lexer ainda conta a linha de um bloco `/* ... */` que atravessa várias linhas.

Não existe lookahead no regex. Onde "casa isto, mas não aquilo" seria preciso, a regra válida vem antes e a de erro fica com o que sobrou. `==` vem antes de `=`. `FLOAT` vem antes de `INT`. Os detalhes estão no [regex.md](regex.md).

## Palavras reservadas

O mapa `reserved` é uma palavra inteira. `casos` não vira `caso` mais `s`.

| Lexema | Type | Concept |
|---|---|---|
| `caso`, `if` | `KEYWORD` | `if` |
| `recurso` | `KEYWORD` | `else if` |
| `senao`, `else` | `KEYWORD` | `else` |
| `jornada`, `for` | `KEYWORD` | `for` |
| `plantao`, `while` | `KEYWORD` | `while` |
| `contratar`, `function` | `KEYWORD` | `function` |
| `rescindir`, `return` | `KEYWORD` | `return` |
| `bater_ponto`, `print` | `KEYWORD` | `print` |
| `reclamacao`, `input` | `KEYWORD` | `input` |
| `carteira`, `var` | `KEYWORD` | `var` |
| `falta`, `break` | `KEYWORD` | `break` |
| `registrado`, `true` | `BOOL_LITERAL` | `true` |
| `pj`, `false` | `BOOL_LITERAL` | `false` |
| `int`, `string`, `float`, `bool`, `void` | `TYPE` | o próprio nome |

## Operadores e delimitadores

Operadores, pelo `Value`: `==` `!=` `<=` `>=` `+` `-` `*` `/` `<` `>` `=`.

Delimitadores: `{` `}` `(` `)` `;` `,`.

`!` sozinho é erro, porque `!=` já foi consumido pela regra mais longa. `%`, `++`, `+=`, `&&`, `||`, `<<`, `>>` e `.` também são erro, cada um com a própria mensagem.

## Erros que o lexer aponta

O token `ERROR` já traz a frase. O parser copia essa frase para a lista de erros, no formato `linha N, col M: ...`.

| Situação | Mensagem |
|---|---|
| `/*` sem `*/` | comentário de bloco aberto e nunca fechado com `*/` |
| `0x`, `0b`, `0o` | notação hexadecimal/binária não existe |
| `1.5e-2` | notação científica não existe |
| `1.2.3` | float com mais de um ponto |
| `44horas` | número colado em identificador |
| `.5` | float sem parte inteira |
| `1.` | float sem parte decimal |
| aspas que não fecham na linha | string não terminada |
| `'A'` | aspas simples não existem |
| `\z` dentro da string | escape inválido |
| `&&`, `\|\|` | operadores lógicos não existem |
| `++`, `--` | incremento/decremento não existe |
| `+=`, `-=`, `*=`, `/=`, `%=` | atribuição composta não existe |
| `<<`, `>>` | deslocamento de bits não existe |
| `!` | negação `!` só é válida em `!=` |
| `%` | módulo não existe |
| `&` ou `\|` | não há operadores de bits |
| `.` | não há acesso a campo |
| `@`, `#`, `$`, `` ` ``, `[`, `]`, `?`, `:` | fora do alfabeto, cada um com a frase do mapa |

Os arquivos [comuns.clt](comuns.clt) e [errors.clt](errors.clt) exercitam esses casos.
