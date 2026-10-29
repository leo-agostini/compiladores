# Os regex do tokenizer, caractere a caractere

Documentacao de todos os 26 padroes de [lexical-analysis.go](lexical-analysis.go). Cada secao mostra
o padrao, decompoe cada caractere e da exemplos do que casa e do que nao casa.

---

## 1. Tres coisas para saber antes

### 1.1. Os padroes sao raw strings de Go

Todos os padroes estao entre crases (`` `...` ``), nao entre aspas. Numa raw string Go
nao processa escapes, entao o que esta escrito e exatamente o que o motor de regex
recebe:

| Escrito no codigo | O que o regex recebe | Se fosse com aspas duplas |
|---|---|---|
| `` `\d` `` | `\d` (digito) | `"\\d"` — precisaria dobrar a barra |
| `` `\n` `` | `\n` (dois caracteres: barra + n, que o regex le como "nova linha") | `"\n"` seria o byte 0x0A literal |
| `` `\\.` `` | `\\.` — barra escapada seguida de "qualquer caractere" | `"\\\\."` |

Por isso nenhuma barra aparece dobrada sem motivo: a dobra que existe e do **regex**,
nao do Go.

### 1.2. `anchored` envelopa todo padrao

```go
func anchored(pattern string) *regexp.Regexp {
	return regexp.MustCompile(`^(?:` + pattern + `)`)
}
```

| Trecho | Leitura |
|---|---|
| `^` | ancora: casa apenas no **inicio do texto**. Sem a flag `m`, e o inicio da string, nao o inicio da linha. |
| `(?:` | abre um grupo **nao capturante** |
| `pattern` | o padrao da regra |
| `)` | fecha o grupo |

O grupo nao e decoracao. Sem ele, `^` grudaria so na primeira alternativa:

- `^a|b` significa "`a` no inicio **ou** `b` em qualquer lugar" — errado.
- `^(?:a|b)` significa "`a` ou `b`, no inicio" — correto.

E o `^` so funciona porque o laco em `tokenize` fatia o texto restante a cada passo
(`rest := code[pos:]`), entao "inicio do texto" e sempre a posicao atual da varredura.

### 1.3. Go usa RE2

O pacote `regexp` de Go nao e um motor com backtracking; e RE2, que garante tempo
linear. Duas consequencias que aparecem o tempo todo neste tokenizer:

- **Nao existe lookahead/lookbehind** (`(?!...)`, `(?<=...)`). Onde seria preciso dizer
  "casa `.` mas nao se vier digito depois", a solucao no arquivo e **ordem**: a regra
  valida vem antes e a regra de erro fica so com o que sobrou.
- **A alternativa e leftmost-first**, nao leftmost-longest: em `==|=`, o ramo `==` vence
  porque esta escrito primeiro, nao porque e mais longo. Trocar a ordem quebraria o `==`.

Invariante importante do laco: `FindString` devolve `""` tanto para "nao casou" quanto
para "casou vazio", e o codigo trata `""` como nao-casou. Todos os 26 padroes exigem no
minimo um caractere, entao nunca casam vazio — uma regra nova precisa manter isso, ou o
laco entra em loop infinito.

---

## 2. Referencia rapida dos metacaracteres usados

| Simbolo | Significado | Onde aparece |
|---|---|---|
| `^` | inicio do texto | `anchored` |
| `.` | qualquer caractere **exceto** nova linha | `\\.` nas strings |
| `\.` | um ponto literal | FLOAT, `\.\d+` |
| `\d` | um digito, o mesmo que `[0-9]` | numeros |
| `\s` | um espaco em branco (espaco, tab, nova linha...) | `[\s\S]` |
| `\S` | o contrario de `\s` | `[\s\S]` |
| `[abc]` | classe: **um** caractere entre os listados | quase todas |
| `[^abc]` | classe negada: um caractere que **nao** esta na lista | comentarios, strings |
| `a-z` | intervalo, dentro de classe | identificadores |
| `*` | zero ou mais repeticoes | quase todas |
| `+` | uma ou mais repeticoes | numeros, espacos |
| `?` | zero ou uma (opcional) | sinal do expoente |
| `*?` | zero ou mais, **preguicoso**: para no primeiro que der | comentario de bloco |
| `{2,}` | duas ou mais repeticoes | float com varios pontos |
| `\|` | alternativa ("ou") | operadores, strings |
| `(?:...)` | grupo nao capturante | numeros, strings |
| `(...)` | grupo capturante | STRING (unico caso) |
| `\` | escapa o proximo, tirando seu poder especial | `\*`, `\+`, `\\|`, `\.` |

Regra de ouro das classes: **dentro de `[...]` quase nada e especial**. `|`, `(`, `)`,
`{`, `}`, `*`, `+`, `.` sao literais ali dentro. Os unicos que ainda importam sao `^`
(quando e o primeiro), `]`, `\` e `-` (quando esta entre dois caracteres, formando
intervalo).

---

## 3. Comentarios

### `//[^\n]*` — COMMENT ([lexical-analysis.go:46](lexical-analysis.go#L46))

| Trecho | Leitura |
|---|---|
| `/` | barra literal |
| `/` | segunda barra literal |
| `[^\n]` | um caractere que nao seja nova linha |
| `*` | zero ou mais deles |

Vai da `//` ate o fim da linha. O `\n` final **nao** e consumido: fica para a regra de
espaco em branco, que e quem incrementa o contador de linha. O `*` (e nao `+`) permite
que um `//` sozinho no fim do arquivo tambem case.

- Casa: `// comentario`, `//`, `//comentario colado`
- Nao casa: `/ / com espaco no meio`

### `/\*[\s\S]*?\*/` — COMMENT ([lexical-analysis.go:47](lexical-analysis.go#L47))

| Trecho | Leitura |
|---|---|
| `/` | barra literal |
| `\*` | asterisco **literal** — sem a barra, `*` seria o quantificador "zero ou mais" |
| `[\s\S]` | um caractere qualquer, **incluindo nova linha** |
| `*?` | zero ou mais, preguicoso |
| `\*` | asterisco literal |
| `/` | barra literal |

Dois detalhes carregam esta regra:

**`[\s\S]` em vez de `.`** — o `.` nao casa nova linha (RE2 so mudaria isso com a flag
`(?s)`). Como um comentario de bloco atravessa linhas, o jeito e pedir a uniao de "e
espaco em branco" com "nao e espaco em branco", que por definicao e todo caractere.

**`*?` preguicoso em vez de `*` guloso** — o preguicoso para no **primeiro** `*/`. Com
o guloso, a linha

```
/* um */ codigo /* dois */
```

viraria **um** comentario so, engolindo `codigo` no meio.

### `/\*[\s\S]*` — ERROR, bloco nao fechado ([lexical-analysis.go:49](lexical-analysis.go#L49))

O mesmo padrao anterior sem o `\*/` final e com `*` guloso: consome do `/*` ate o fim do
arquivo.

So e alcancado quando a regra 48 falha, ou seja, quando nao existe nenhum `*/` a frente
— e por isso ela vem **depois** na tabela. Invertendo a ordem, todo comentario de bloco
valido do arquivo viraria erro.

---

## 4. Identificadores

### `[a-zA-Z_][a-zA-Z0-9_]*` — WORD ([lexical-analysis.go:52](lexical-analysis.go#L52))

| Trecho | Leitura |
|---|---|
| `[a-zA-Z_]` | primeiro caractere: letra minuscula, maiuscula ou `_` |
| `[a-zA-Z0-9_]` | demais caracteres: idem, mais digitos |
| `*` | zero ou mais deles |

A diferenca entre as duas classes e o `0-9`, ausente na primeira: **identificador nao
comeca com digito**. E o `*` (nao `+`) garante que um nome de uma letra so, como `a` ou
`e`, seja valido.

O tipo `WORD` nao chega na saida: no laco de `tokenize` ele vira `IDENTIFIER`, ou
`KEYWORD`/`TYPE`/`BOOL_LITERAL` se o texto estiver no mapa `reserved`. Fazer assim, em
vez de um regex por palavra reservada, e o que garante que `casos` seja um identificador
inteiro, e nao a palavra `caso` seguida de um `s` solto.

- Casa: `horas`, `_tmp`, `bater_ponto`, `x1`, `casos`
- Nao casa: `44horas` (comeca com digito), `cafeé` (acento fora da classe)

---

## 5. Numeros

Sao oito padroes. Os cinco de erro vem **antes** de FLOAT e INT na tabela, senao o
numero malformado seria lido pela metade: `1.2.3` viraria FLOAT `1.2` mais lixo.

### `0[xXbBoO][0-9a-fA-F_]*` — ERROR, hexadecimal/binario ([lexical-analysis.go:56](lexical-analysis.go#L56))

| Trecho | Leitura |
|---|---|
| `0` | o digito zero, literal |
| `[xXbBoO]` | uma das letras de prefixo: hexa, binario ou octal, em maiuscula ou minuscula |
| `[0-9a-fA-F_]` | digito, letra de `a` a `f` (os digitos hexa) ou `_` |
| `*` | zero ou mais |

O `*` no fim deixa `0x` sozinho tambem casar — melhor dar erro de "notacao nao existe"
do que deixar passar.

Nao afeta o zero comum: `0[xXbBoO]` exige uma letra logo apos o `0`, entao em `0;` ou
`0.0` este padrao falha na segunda posicao.

- Casa: `0x1F`, `0b1010`, `0o777`, `0x`
- Nao casa: `0`, `0.0`, `10`

### `\d+(?:\.\d+)?[eE][+-]?\d+` — ERROR, notacao cientifica ([lexical-analysis.go:58](lexical-analysis.go#L58))

| Trecho | Leitura |
|---|---|
| `\d+` | um ou mais digitos (a mantissa) |
| `(?:` | abre grupo nao capturante |
| `\.` | ponto literal |
| `\d+` | um ou mais digitos |
| `)` | fecha o grupo |
| `?` | o grupo todo e opcional — e o que faz `1e10` e `1.5e-2` casarem no mesmo padrao |
| `[eE]` | a letra do expoente |
| `[+-]` | sinal; dentro da classe o `-` no fim e literal, nao forma intervalo |
| `?` | sinal opcional |
| `\d+` | um ou mais digitos do expoente, obrigatorios |

O `(?:` em vez de `(` e so eficiencia: o codigo usa `FindString` e nunca le grupos
capturados, entao nao ha motivo para o motor guardar o texto do grupo.

- Casa: `1e10`, `1.5e-2`, `3E+8`
- Nao casa: `1e` (falta expoente), `e10` (falta mantissa — vira identificador)

### `\d+(?:\.\d+){2,}` — ERROR, float com varios pontos ([lexical-analysis.go:60](lexical-analysis.go#L60))

| Trecho | Leitura |
|---|---|
| `\d+` | digitos iniciais |
| `(?:\.\d+)` | um bloco "ponto seguido de digitos" |
| `{2,}` | **duas ou mais** repeticoes desse bloco |

O `{2,}` e o coracao da regra. Com `+` (uma ou mais), `1.5` — que e um float valido —
tambem casaria e viraria erro. Exigindo dois blocos, so o que tem ponto sobrando cai
aqui.

- Casa: `1.2.3`, `1.5.2.3`
- Nao casa: `1.5`, `44`

### `\d+(?:\.\d+)?[a-zA-Z_][a-zA-Z0-9_]*` — ERROR, numero colado em identificador ([lexical-analysis.go:62](lexical-analysis.go#L62))

E a juncao do padrao de numero com o de identificador: `\d+`, uma parte decimal
opcional, e em seguida um identificador completo, sem nada entre eles.

Consumir o lexema inteiro e o que evita a cascata. Sem esta regra, `44horas` sairia como
INT `44` seguido de IDENTIFIER `horas` — dois tokens plausiveis e **nenhum erro**, que
era exatamente o bug de antes.

- Casa: `44horas`, `1.5x`, `10_total`
- Nao casa: `44 horas` (o espaco separa), `horas44` (comeca com letra, vira WORD)

### `\.\d+` — ERROR, float sem parte inteira ([lexical-analysis.go:64](lexical-analysis.go#L64))

| Trecho | Leitura |
|---|---|
| `\.` | ponto literal — sem a barra, `.` casaria qualquer caractere |
| `\d+` | um ou mais digitos |

- Casa: `.5`, `.25`
- Nao casa: `.` sozinho (cai na regra do ponto isolado, mais abaixo)

### `\d+\.\d+` — FLOAT ([lexical-analysis.go:67](lexical-analysis.go#L67))

Digitos, ponto literal, digitos. Exige numero dos dois lados do ponto — e essa exigencia
que deixa `1.` e `.5` sobrarem para as regras de erro.

### `\d+\.` — ERROR, float sem parte decimal ([lexical-analysis.go:69](lexical-analysis.go#L69))

Digitos seguidos de ponto, sem exigir nada depois.

Sozinho, este padrao tambem casaria o comeco de `1.5`. Ele nao estraga o FLOAT porque
esta escrito **depois** dele na tabela: em `1.5` a regra 68 ja venceu, e so um `1.` de
verdade chega ate aqui. Este e o caso mais claro de "ordem substituindo o lookahead que
o RE2 nao tem" — com lookahead seria `\d+\.(?!\d)`.

### `\d+` — INT ([lexical-analysis.go:71](lexical-analysis.go#L71))

Um ou mais digitos. Vem por ultimo entre os numeros: se viesse antes de FLOAT, `3.14`
seria lido como INT `3` e o resto viraria lixo.

---

## 6. Strings e caracteres

### `"(\\.|[^"\\\n])*"` — STRING ([lexical-analysis.go:73](lexical-analysis.go#L73))

O padrao mais denso do arquivo. Contando caractere a caractere:

| Trecho | Leitura |
|---|---|
| `"` | aspas de abertura, literal |
| `(` | abre grupo (capturante — o unico do arquivo; `(?:` seria equivalente e um pouco mais barato) |
| `\\` | **uma** barra invertida literal: a primeira escapa a segunda |
| `.` | qualquer caractere exceto nova linha |
| `\|` | ou |
| `[^` | abre classe negada |
| `"` | ...que nao seja aspas |
| `\\` | ...nem barra invertida |
| `\n` | ...nem nova linha |
| `]` | fecha a classe |
| `)` | fecha o grupo |
| `*` | zero ou mais repeticoes do grupo — o `*` deixa `""` valida |
| `"` | aspas de fechamento, literal |

O conteudo da string e, portanto, uma sequencia de duas coisas possiveis: **um par de
escape** (`\\.`) ou **um caractere comum** (`[^"\\\n]`).

A ordem dentro da alternativa e essencial. `\\.` vem primeiro para que `\"` seja
consumido como um par unico; se a alternativa comum viesse antes, o `"` de `\"` fecharia
a string cedo demais.

O `\n` na classe negada e o que impede uma string de atravessar linhas: assim, um
`"` sem par nao engole o arquivo inteiro, so o resto da linha.

O que este padrao **nao** consegue fazer e distinguir `\n` de `\z`, porque `\\.` aceita
qualquer caractere depois da barra. Negar isso exigiria lookahead. Por isso a validacao
de escape acontece em Go, na funcao `invalidEscape`, sobre o lexema ja casado.

- Casa: `"Ana"`, `""`, `"diz \"oi\""`, `"barra dupla \\z"`
- Nao casa: `"quebra` + nova linha, `'A'`

### `"(?:\\.|[^"\\\n])*` — ERROR, string nao terminada ([lexical-analysis.go:76](lexical-analysis.go#L76))

Identico ao anterior, sem as aspas de fechamento (e com `(?:`). Consome da aspa aberta
ate o fim da linha.

Vem **depois** de STRING, entao so recebe o que nao tinha par. E consumir tudo ate o fim
da linha e o que mata a cascata: antes, `"nao fecha;` gerava um ERROR na aspa e depois
tokenizava `nao` e `fecha` como identificadores.

### `'(?:\\.|[^'\\\n])*'?` — ERROR, aspas simples ([lexical-analysis.go:78](lexical-analysis.go#L78))

Mesma estrutura, com `'` no lugar de `"`, e o fechamento marcado com `?`:

| Trecho | Leitura |
|---|---|
| `'` | aspa simples de abertura |
| `(?:\\.\|[^'\\\n])*` | conteudo: pares de escape ou caracteres que nao sejam `'`, `\` ou nova linha |
| `'` | aspa de fechamento |
| `?` | ...opcional |

O `?` final cobre os dois casos com uma regra so: `'A'` (fechada) e `'a` no fim do
arquivo (sem par). A linguagem nao tem tipo caractere, entao ambos sao erro — a mensagem
manda usar aspas duplas.

---

## 7. Operadores

Os quatro padroes de erro abaixo vem **antes** de OPERATOR. Sem isso, `++` seria lido
como dois `+` validos e nenhum erro apareceria.

### `&&|\|\|` — ERROR, operadores logicos ([lexical-analysis.go:84](lexical-analysis.go#L84))

- `&` `&` — dois "e comercial" literais; `&` nao tem poder especial em regex
- `|` — a alternativa
- `\|` `\|` — duas barras verticais **literais**; fora de uma classe, `|` precisa de
  escape, senao seria mais uma alternativa (e `&&|||` seria lido como "`&&` ou vazio ou
  vazio")

### `\+\+|--` — ERROR, incremento/decremento ([lexical-analysis.go:86](lexical-analysis.go#L86))

- `\+` `\+` — dois `+` literais; sem a barra, `+` seria o quantificador "um ou mais"
- `|` — ou
- `-` `-` — dois hifens; **fora** de uma classe o `-` nao e especial e dispensa escape

O contraste `\+` versus `-` na mesma linha resume a regra: escapa-se so o que tem poder
naquela posicao.

### `\+=|-=|\*=|/=|%=` — ERROR, atribuicao composta ([lexical-analysis.go:88](lexical-analysis.go#L88))

Cinco alternativas de dois caracteres. Os escapes: `\+` (quantificador) e `\*`
(quantificador). Ja `/` e `%` sao literais comuns — em Go o padrao e uma string, nao
tem delimitadores `/.../` como em JavaScript, entao a barra nao precisa de escape.

### `<<|>>` — ERROR, deslocamento de bits ([lexical-analysis.go:90](lexical-analysis.go#L90))

Quatro caracteres literais em duas alternativas. `<` e `>` nao sao especiais em RE2.

Nao conflita com `<=` e `>=`, que tem o segundo caractere diferente.

### `==|!=|<=|>=|[+\-*/<>=]` — OPERATOR ([lexical-analysis.go:93](lexical-analysis.go#L93))

| Trecho | Leitura |
|---|---|
| `==` | igualdade |
| `!=` | diferenca |
| `<=` | menor ou igual |
| `>=` | maior ou igual |
| `[` | abre classe: um unico caractere entre os seguintes |
| `+` | mais — literal **dentro** da classe, sem escape |
| `\-` | hifen **escapado**: entre `+` e `*` um `-` cru viraria intervalo `+` ate `*` |
| `*` `/` `<` `>` `=` | literais dentro da classe |
| `]` | fecha a classe |

As quatro alternativas de dois caracteres vem antes da classe de um caractere porque Go
casa a **primeira** alternativa que funciona, nao a mais longa. Se `[+\-*/<>=]` viesse
primeiro, `==` seria lido como dois `=` separados.

O `\-` e o detalhe facil de errar: dentro de `[...]`, `-` so e literal se estiver no
comeco, no fim, ou escapado. Aqui ele esta no meio, entao leva barra. Compare com
`[+-]?` do expoente, onde o `-` esta no fim e por isso fica cru.

### `!` — ERROR, negacao isolada ([lexical-analysis.go:96](lexical-analysis.go#L96))

Um caractere literal. Toda a inteligencia esta na **posicao**: vem depois de OPERATOR,
entao `!=` ja foi consumido la em cima e so o `!` sozinho chega aqui. Com lookahead
seria `!(?!=)`; sem ele, a ordem faz o trabalho.

### `%` — ERROR, modulo ([lexical-analysis.go:98](lexical-analysis.go#L98))

Um caractere literal. `%` nao e especial em regex (isso e formatacao de string, outra
coisa).

### `[&|]` — ERROR, `&` ou `|` sozinhos ([lexical-analysis.go:100](lexical-analysis.go#L100))

| Trecho | Leitura |
|---|---|
| `[` | abre classe |
| `&` | literal |
| `\|` | literal — **dentro da classe a barra vertical perde o sentido de alternativa** |
| `]` | fecha |

Vale comparar com a regra 85: `\|\|` fora de classe precisa de escape, `[&|]` dentro de
classe nao. Pega o que sobrou de `&&`/`||`, ou seja, o simbolo solto.

### `\.` — ERROR, ponto isolado ([lexical-analysis.go:102](lexical-analysis.go#L102))

Um ponto literal. Vem depois de todas as regras de numero, entao `1.5`, `1.`, `.5` e
`1.2.3` ja foram tratados; aqui chega o ponto de `horas.campo`, que na linguagem nao
tem significado.

---

## 8. Delimitadores e espaco

### `[{}();,]` — DELIMITER ([lexical-analysis.go:105](lexical-analysis.go#L105))

| Trecho | Leitura |
|---|---|
| `[` | abre classe |
| `{` `}` | chaves literais — fora da classe iniciariam contagem de repeticao, tipo `{2,}` |
| `(` `)` | parenteses literais — fora da classe abririam grupo |
| `;` `,` | literais |
| `]` | fecha |

Quatro caracteres que seriam metacaracteres soltos ficam inofensivos so por estarem
dentro de `[...]`.

### `[ \t\r\n]+` — espaco em branco ([lexical-analysis.go:169](lexical-analysis.go#L169))

| Trecho | Leitura |
|---|---|
| `[` | abre classe |
| (espaco) | um espaco literal |
| `\t` | tabulacao |
| `\r` | retorno de carro (arquivos salvos no Windows usam `\r\n`) |
| `\n` | nova linha |
| `]` | fecha |
| `+` | uma ou mais |

Nao gera token: e consumido no topo do laco. O `+` importa para o desempenho e para a
contagem de linhas — a indentacao inteira sai de uma vez, e `strings.Count(ws, "\n")`
soma todas as quebras do bloco de uma so vez.

---

## 9. Resumo: por que a ordem da tabela e o que ela e

A tabela `rules` e lida de cima para baixo e o primeiro padrao que casar vence. Cinco
pares dependem disso:

| Antes | Depois | Se invertesse |
|---|---|---|
| `/\*[\s\S]*?\*/` (fechado) | `/\*[\s\S]*` (erro) | todo comentario de bloco valido viraria erro |
| erros de numero | `\d+\.\d+`, `\d+` | `1.2.3` seria FLOAT `1.2` + lixo, sem erro |
| `\d+\.\d+` (FLOAT) | `\d+\.` (erro) | `1.5` seria erro `1.` seguido de INT `5` |
| `"..."` (STRING) | `"...` (erro) | toda string valida viraria "nao terminada" |
| `\+\+`, `\+=`, `&&`... | `==\|!=\|...` (OPERATOR) | `++` seria dois `+` validos, sem erro |

E dois pares dependem do contrario — a regra de erro vem **depois** da valida, porque e
assim que se emula o lookahead que o RE2 nao tem:

| Antes | Depois | Efeito |
|---|---|---|
| OPERATOR (que tem `!=`) | `!` | so o `!` sem `=` vira erro |
| `\d+\.\d+` (FLOAT) | `\d+\.` | so o `1.` sem decimal vira erro |
