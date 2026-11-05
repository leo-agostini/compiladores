# A linguagem CLT

CLT é a linguagem do compilador. O fonte é um arquivo `.clt`. Cada palavra em português tem um equivalente em inglês, e os dois produzem o mesmo papel no compilador.

Os outros documentos:

- [regex.md](regex.md) — cada padrão do tokenizer, caractere a caractere
- [tokenizer.md](tokenizer.md) — tokens, palavras reservadas e o que o lexer entrega
- [parser.md](parser.md) — gramática e a árvore sintática
- [semantica.md](semantica.md) — escopos, tipos e mensagens de erro

Para compilar um arquivo:

```text
go run . teste.clt
```

Sem argumento, o programa lê `teste.clt`. A saída padrão mostra a árvore e, quando ela está inteira, a tabela de símbolos. Erro léxico, sintático ou semântico vai para a saída de erro e o processo termina com código 1. A análise semântica só roda quando a árvore está inteira.

---

## Tipos

| Tipo | O que guarda | Exemplo |
|---|---|---|
| `int` | inteiro | `44` |
| `float` | número com ponto | `1.5` |
| `string` | texto entre aspas duplas | `"Ana"` |
| `bool` | verdadeiro ou falso | `registrado`, `pj` |
| `void` | função sem valor | `void avisar(...)` |

`int` pode ser usado onde se espera `float`. O caminho contrário é erro. `void` não é valor: não declara variável e não serve de argumento.

## Palavras

| Português | Inglês | Papel |
|---|---|---|
| `carteira` | `var` | declaração de variável |
| `caso` | `if` | condicional |
| `senao` | `else` | ramo final |
| `recurso` | `else if` | ramo intermediário, uma palavra só |
| `senao caso` | `else if` | o mesmo ramo, duas palavras fundidas |
| `jornada` | `for` | laço com init, condição e passo |
| `plantao` | `while` | laço com condição |
| `contratar` | `function` | função com o tipo depois dos parâmetros |
| `rescindir` | `return` | retorno |
| `bater_ponto` | `print` | escrita |
| `reclamacao` | `input` | leitura, devolve `string` |
| `falta` | `break` | sai do `for` ou do `while` |
| `registrado` | `true` | bool verdadeiro |
| `pj` | `false` | bool falso |

`int`, `float`, `string`, `bool` e `void` são tipos, não nomes. Palavra reservada não pode ser nome de variável ou função. `casos` já é identificador: a reserva vale a palavra inteira.

## Formas que o fonte aceita

Variável. O `=` é opcional. O `;` é obrigatório.

```clt
carteira int horas = 44;
carteira string nome = "Ana";
carteira float taxa = 1.5;
carteira bool ativo = registrado;
carteira int extras;
```

Função, nas duas ordens. Sem tipo depois de `contratar nome()`, o retorno é `void`.

```clt
float calcular(int base, float adicional) {
    rescindir base + adicional * 2;
}

contratar avisar(string mensagem) void {
    bater_ponto(mensagem);
}
```

Condicional, laço e saída.

```clt
caso (horas >= 44) {
    avisar("hora extra");
} recurso (horas > 0) {
    horas = horas - 1;
} senao {
    falta;
}

plantao (horas != 0) {
    horas = horas - 1;
}

jornada (carteira int i = 0; i < 3; i = i + 1) {
    bater_ponto(i);
}
```

`falta` só dentro de `jornada` ou `plantao`. `rescindir` só dentro de função.

Chamada e atribuição. `a = b = 1` vale, da direita para a esquerda.

```clt
taxa = calcular(horas, taxa);
nome = reclamacao();
bater_ponto(nome);
```

Comentário de linha (`//`) e de bloco (`/* ... */`). Divisão é `taxa / 2`. `//` colado num nome começa comentário.

## O que não existe

Não há `&&`, `||`, `!` sozinho, `++`, `--`, `+=`, `%`, deslocamento de bits, acesso a campo (`.`), vetor, notação `0x` nem notação científica. String usa aspas duplas e fica na mesma linha. Os escapes válidos são `\n`, `\t`, `\r`, `\0`, `\"` e `\\`.

`+` entre strings não concatena. Os operadores aritméticos só aceitam número.
