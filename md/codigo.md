# Geração de código

Depois da análise semântica, o compilador traduz a árvore em duas etapas:

```text
AST ──► GenerateTAC ──► código de três endereços ──► GenerateMIPS ──► assembly MIPS
        tac.go                                       mips.go
```

O `main` só chega aqui quando a análise semântica não reportou nada. O gerador confia nos nomes e nos tipos já conferidos.

```text
go run . codigo.clt            # imprime o TAC e a alocação, escreve codigo.asm
go run . codigo.clt saida.asm  # escolhe o arquivo de saída
spim -file codigo.asm          # roda (o MARS também abre o .asm)
```

A saída é `.asm`, e não `.s`: o Go tenta montar todo `.s` que encontra na pasta do pacote, e o build quebraria.

---

## Código de três endereços

Cada instrução tem no máximo um operador e três endereços: dois operandos e um destino. Um endereço é um `Operand`:

| Tipo | Exemplo | De onde vem |
|---|---|---|
| variável | `horas`, `x.1` | declaração do fonte |
| temporário | `t1`, `t2` | resultado intermediário criado pelo gerador |
| constante | `44`, `1.5`, `"Ana"`, `true` | literal do fonte |

Todo operando carrega o tipo da linguagem (`int`, `float`, `string`, `bool`). O TAC impresso não mostra o tipo, mas a geração de MIPS usa ele para escolher entre `add` e `add.s`, `lw` e `l.s`.

### Instruções

| Forma | Significado |
|---|---|
| `x = y op z` | binária: `+ - * /` e comparações `< > <= >= == !=` |
| `x = - y` | menos unário |
| `x = y` | cópia |
| `x = (float) y` | conversão de `int` para `float` |
| `L1:` | rótulo |
| `goto L1` | desvio incondicional |
| `ifFalse x goto L1` | desvia se `x` é falso |
| `ifFalse x < y goto L1` | desvia se a comparação é falsa, sem temporário |
| `param x` | empilha um argumento |
| `t = call f, n` | chama `f` com os `n` últimos `param` |
| `return x` | retorno, com ou sem valor |
| `print x`, `println` | escrita |
| `t = read` | leitura de uma linha (`reclamacao`) |

### O teste da aula

```clt
d = (a + b) * c;
```

```text
t1 = a + b
t2 = t1 * c
d = t2
```

Os parênteses obrigam a soma a vir antes. A tradução é pós-ordem: os dois filhos de um operador geram código primeiro, e o operador usa os endereços que eles devolveram. Cada operador ganha um temporário novo. A cópia para `d` vem por último. `TestTACSlide` confere exatamente essas linhas.

Sem parênteses, `d = a + b * c` dá `t1 = b * c`, `t2 = a + t1`, `d = t2`. A precedência já está na forma da árvore.

### Como cada nó vira TAC

`expr(n)` devolve o endereço que guarda o valor de `n`:

- literal e variável não geram instrução. Devolvem a própria constante ou o nome;
- binário e unário geram uma instrução com um temporário novo como destino e devolvem esse temporário;
- atribuição gera a cópia e devolve a variável. Por isso `a = b = 5` vira `b = 5` e depois `a = b`;
- chamada avalia todos os argumentos, depois emite os `param` em ordem, depois o `call`. Num `f(1, f(2, 3))`, os `param` da chamada de dentro não se misturam com os da de fora.

Comandos:

```text
if c1 {A} senao caso c2 {B} senao {C}     while (c) {A}        for (i; c; p) {A}

    ifFalse c1 goto L1                    L1:                      i
    A                                         ifFalse c goto L2    L1:
    goto L3                                   A                        ifFalse c goto L2
L1:                                           goto L1                  A
    ifFalse c2 goto L2                    L2:                          p
    B                                                                  goto L1
    goto L3                                                        L2:
L2:
    C
L3:
```

`falta` é `goto` para o rótulo de saída do laço mais interno. Uma condição que já é comparação vai direto no `ifFalse` (`ifFalse n > 0 goto L2`); qualquer outra (`ifFalse ativo goto L2`) usa o valor.

`bater_ponto(a, b)` vira `print a`, `print " "`, `print b`, `println`.

### Conversão de tipo

A semântica aceita `int` onde se espera `float`, sem mudar a árvore. O gerador torna essa conversão explícita:

```clt
carteira int i = 1;
carteira float f = 2.5;
f = f + i;
```

```text
t1 = (float) i
t2 = f + t1
f = t2
```

A conversão aparece em operação mista, atribuição, inicialização, argumento e `return`. Numa constante, ela é feita na hora: `carteira float f = 2;` vira `f = 2.0`.

### Nomes

O TAC é plano, sem escopos. Dois `x` em escopos diferentes precisam de nomes diferentes: o primeiro fica `x`, os seguintes viram `x.1`, `x.2`. O ponto não existe em identificador da linguagem, então não colide com um nome do fonte. Temporários (`t1`) e rótulos (`L1`) são numerados no programa inteiro.

Variável declarada sem valor começa com zero (`0`, `0.0`, `""`, `false`). Assim uma variável dentro de um laço não herda o valor da volta anterior.

### Funções

Os comandos soltos no topo do arquivo formam `programa`. Cada função do fonte vira um bloco `func nome(params) tipo: ... endfunc`. Uma função `void` sem `rescindir` no fim ganha um `return` implícito.

Uma função declarada dentro de outra vira uma função separada. Ela pode usar globais e chamar outras funções, mas não pode ler variável local da função de fora: isso exigiria um elo de acesso entre os quadros. O gerador reporta `funcao aninhada nao pode usar 'k', variavel local de 'f'`.

---

## MIPS com alocação ingênua

A saída roda no SPIM e no MARS. Cada instrução do TAC aparece como comentário antes do código dela.

### A estratégia ingênua

Todo nome do TAC tem uma casa fixa na memória. Os registradores só guardam valores durante uma instrução do TAC. Para `x = y op z`:

```text
# t1 = a + b
lw   $t0, v_a          carrega os operandos
lw   $t1, v_b
addu $t0, $t0, $t1     opera em registrador
sw   $t0, -4($fp)      guarda na casa de t1
```

Nenhum valor passa de uma instrução para a seguinte num registrador. Por isso não há registrador para salvar numa chamada nem valor para derramar (spill). O código é o mais lento possível: `t2 = t1 * c` carrega de volta o `t1` que acabou de ser guardado. Esse é o ponto de partida que as alocações melhores otimizam.

| Registrador | Uso |
|---|---|
| `$t0`, `$t1` | operandos `int`, `bool` e `string` (endereço) |
| `$f0`, `$f2` | operandos `float` (precisão simples) |
| `$v0` / `$f0` | valor de retorno |
| `$a0`, `$f12` | argumento das chamadas de sistema |
| `$fp`, `$sp`, `$ra` | quadro e retorno |

### Onde mora cada nome

| Nome | Casa | Exemplo |
|---|---|---|
| variável do topo do arquivo | rótulo no `.data` | `v_horas`, `v1_x` para `x.1` |
| parâmetro | acima do `$fp`, onde quem chamou empilhou | `8($fp)`, `12($fp)` |
| variável local, temporário | abaixo do `$fp`, no quadro | `-4($fp)`, `-8($fp)` |

`Allocate` percorre o TAC da função e dá os próximos 4 bytes do quadro a cada nome novo, na ordem em que aparece. Nenhum espaço é reaproveitado, nem o de um temporário que já não será lido. O corpo do programa também tem quadro, para os seus temporários e para as variáveis de bloco no topo.

O `main` imprime a tabela:

```text
FUNCAO    NOME   TIPO    CASA
fatorial  n      int     8($fp) (parametro)
fatorial  t3     int     -4($fp) (temporario)
fatorial  t4     int     -8($fp) (temporario)
fatorial  t5     int     -12($fp) (temporario)
media     total  int     12($fp) (parametro)
media     qtd    int     8($fp) (parametro)
media     t7     float   -4($fp) (temporario)
media     m      float   -8($fp) (local)
```

### Chamada de função

```text
         ...             argumentos empilhados por quem chamou
         12($fp)         penúltimo argumento
         8($fp)          último argumento
         4($fp)          $ra salvo
$fp ──►  0($fp)          $fp de quem chamou
         -4($fp)         primeira local ou temporário
         ...
$sp ──►
```

- `param x`: `addiu $sp, $sp, -4` e guarda `x` no topo da pilha.
- `t = call f, n`: `jal f_f`, desempilha `4*n` bytes e guarda `$v0` (ou `$f0`) na casa de `t`.
- Prólogo: salva `$ra` e `$fp`, aponta `$fp` para o quadro e reserva o espaço das locais.
- `return x`: põe `x` em `$v0` ou `$f0`, restaura `$sp`, `$fp` e `$ra` e volta com `jr $ra`.

Cada chamada tem quadro próprio, então a recursão funciona: o `n` de `fatorial(5)` e o de `fatorial(4)` moram em quadros diferentes.

Funções do fonte ganham o prefixo `f_` (`f_fatorial`), e as globais `v_`. Uma função chamada `main` não colide com o ponto de entrada.

### Tipos no MIPS

| Tipo | Representação | Escrita |
|---|---|---|
| `int` | palavra | syscall 1 |
| `float` | precisão simples, coprocessador 1 | syscall 2 |
| `bool` | palavra, 1 ou 0 | `true` / `false` |
| `string` | endereço de texto terminado em zero | syscall 4 |

Comparação de inteiros usa `slt`, `sgt`, `sle`, `sge`, `seq`, `sne`. De float, `c.lt.s`, `c.le.s`, `c.eq.s` com `bc1t`; `>` e `>=` trocam a ordem dos operandos. `==` e `!=` entre strings chamam a rotina `_streq`. `reclamacao()` chama `_read`, que pega 256 bytes com `sbrk`, lê a linha e tira o `\n`. As rotinas só entram no arquivo quando o programa usa.

---

## Testes

```text
go test ./...
```

[codegen_test.go](../codegen_test.go):

- `TestTACSlide`: o TAC de `d = (a + b) * c`, linha por linha;
- `TestTACFormas`: precedência, conversão, atribuição encadeada, `while`, `if/else`, `for` com `break`, ordem dos `param`, sombreamento;
- `TestTACFuncao`: o TAC de `fatorial`;
- `TestAlocacao`: a casa de cada nome e o tamanho do quadro;
- `TestCodigoNoSpim`: compila [codigo.clt](../codigo.clt), roda no SPIM e compara com o bloco `/* saida esperada: ... */` no fim do arquivo. Sem `spim` instalado, o teste é pulado.

## O que esta fase não faz

Não otimiza: não reaproveita temporário, não propaga constante, não elimina a cópia `d = t2`. Não há concatenação de string. Divisão por zero não é tratada. Função aninhada não lê local da função de fora.
