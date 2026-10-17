# Auditoria Diagnóstica de Game Design, Dedução e Ergonomia Lúdica
**Fase 1: Revisão Geral do Projeto Detiv**  
**Autor:** Oracle (Líder de Game Research, Game Design, Dedução e Ergonomia Lúdica)  
**Data:** 17 de Outubro de 2026  
**Status:** Concluído — Pronto para Revisão do Maestro / Orquestrador  
**Arquivos Inspecionados:**
- [`internal/game/detective_night.go`](file:///C:/Users/limae/Documents/Projects/Detiv/internal/game/detective_night.go)
- [`internal/game/game.go`](file:///C:/Users/limae/Documents/Projects/Detiv/internal/game/game.go)
- [`internal/game/weaver.go`](file:///C:/Users/limae/Documents/Projects/Detiv/internal/game/weaver.go)
- [`internal/server/handlers.go`](file:///C:/Users/limae/Documents/Projects/Detiv/internal/server/handlers.go)
- [`web/static/js/app.js`](file:///C:/Users/limae/Documents/Projects/Detiv/web/static/js/app.js)
- [`docs/RESEARCH_FEEDBACK_V2.md`](file:///C:/Users/limae/Documents/Projects/Detiv/docs/RESEARCH_FEEDBACK_V2.md) e [`docs/superpowers/specs/2026-10-14-tactile-desk-cartoonesque-noir-multitheme-design.md`](file:///C:/Users/limae/Documents/Projects/Detiv/docs/superpowers/specs/2026-10-14-tactile-desk-cartoonesque-noir-multitheme-design.md)

---

## 1. Diagnóstico das Mecânicas e Loop de Gameplay Atual

### 1.1 O Core Loop Atual
O fluxo de jogo implementado até a versão v0.5 segue o seguinte ciclo:
1. **Início do Caso:** O jogador escolhe o modo (Solo com tema/suspeitos configuráveis ou Sala Multiplayer). O caso é gerado proceduralmente pelo motor `Weaver` ou carregado a partir do catálogo *Detective Night*.
2. **Exploração de Dados:** Na mesa tátil, o jogador pode examinar:
   - Dossiê do Caso (Vítima, descrição do crime e fotos de cena).
   - Envelopes periciais (Pastas 1, 2 e 3).
   - Lista de Suspeitos (polaroids com papéis e álibis sumários).
   - Lista de Pistas (cards de pistas forenses, documentais e de contradição).
3. **Interrogatório Dinâmico:** O jogador abre a sala de interrogatório de um suspeito:
   - Pode enviar perguntas abertas ou clicar em fichas de sugestão (*chips*).
   - Pode anexar uma pista coletada (`confront_clue_id`).
   - O suspeito responde e seu medidor de estresse (0-100%) oscila entre 3 camadas narrativas (Tier 1: Álibi; Tier 2: Pequeno Delito; Tier 3: Verdade Íntima).
4. **Economia de Recursos:**
   - Perguntas regulares consom **15 minutos**.
   - Confrontos de pista consom **20 minutos**.
   - Acusações erradas consom **30 minutos** e **1 mandado**.
5. **Acusação e Veredito:**
   - O jogador seleciona: Suspeito + Arma + Motivo + 2 Pistas anexadas.
   - O backend valida a hipótese:
     - Se acertar: Veredito de Sucesso (Caso Resolvido).
     - Se errar: Perda de 1 Mandado e 30 minutos. Se zerar os 2 mandados ou o relógio (420 min / 04:00), *Game Over*.

---

## 2. Auditoria Crítica por Dimensão (Doubt-Driven Analysis)

### 2.1 Economia de Tempo (Relógio 21:00 às 04:00 / 420 Minutos)
* **Previsão no Design:** Gerar tensão claustrofóbica de prazo fatal (o trem do assassino foge às 04:00).
* **Realidade no Código e Experiência:**
  - O orçamento total é de **420 minutos**.
  - Como cada pergunta custa 15 min, o jogador pode realizar **28 perguntas livres** antes de esgotar o tempo.
  - Em um caso com 3 ou 4 suspeitos, 28 perguntas é uma margem excessivamente generosa. É praticamente impossível perder por tempo explorando normalmente.
  - O único jeito prático de perder é esgotando os **2 Mandados de Prisão**.
  - **Veredito:** O relógio atualmente funciona como um *contador cosmético*, sem exercer verdadeira pressão psicológica ou estratégica de escassez (ao contrário de *Shadows of Doubt* ou *Sherlock Holmes Consulting Detective*, onde cada pista visitada é uma escolha custosa).

### 2.2 Prevenção de Brute-Force e Limite de 2 Mandados
* **Previsão no Design:** Impedir que o jogador chute combinações aleatórias de culpados e armas até ganhar.
* **Realidade no Código e Experiência:**
  - **O Limite de 2 Mandados é Eficaz contra chutes repetidos de Suspeito:** Com 4 suspeitos, um jogador que chuta às cegas tem apenas 25% de chance na primeira tentativa e 33% na segunda.
  - **A Falha Crítica das Pistas Anexadas:**
    - Em `internal/server/handlers.go` (linha 527), o código apenas verifica:
      ```go
      if len(req.ClueIDs) != 2 {
          h.error(w, http.StatusBadRequest, "anexe duas pistas para fundamentar o mandado")
          return
      }
      ```
    - No entanto, a função `SubmitAccusation` em `internal/game/game.go` (linha 297) **sequer recebe o array `ClueIDs`**!
    - **Gravidade Lúdica:** O jogador pode selecionar *quaisquer duas pistas aleatórias* (ou até duas pistas que não têm nenhuma relação com o assassino ou a arma) que o backend não faz nenhuma validação semântica ou lógica sobre as pistas escolhidas. A exigência de provas é puramente formal (`len == 2`), sem valor dedutivo real.

### 2.3 Aplicação da Regra das Três Pistas (Three-Clue Rule)
* **Previsão no Design:** Conforme formulado por Justin Alexander, para qualquer dedução essencial, deve haver 3 vetores independentes de descoberta (Físico, Testemunhal, Documental).
* **Realidade no Código e Experiência:**
  - O motor `Weaver` gera:
    1. Pista Física (Arma).
    2. Pista de Contradição de Álibi.
    3. Pista Documental (Motivo).
    4. Pista Atmosférica/Circunstancial.
  - **O Problema da Revelação Prematura ("Spoiler Clues"):**
    - Em `internal/game/weaver.go` (linhas 450-462), a Pista 2 e a Pista 3 são geradas da seguinte forma:
      ```go
      contradictionFormat: "... O álibi alegado por %s contém contradição fatal e é falso."
      docDesc: "... diretamente conectado a %s."
      ```
    - O motor insere o **nome explícito do culpado** no texto da pista!
    - E pior: **todas as pistas são exibidas abertas na aba "Pistas" desde o primeiro segundo de jogo**.
    - **Impacto no Momento Eureka:** O jogador não precisa deduzir nada! Basta abrir a aba de Pistas, ler a Pista 2 (que literalmente diz quem mentiu) ou a Pista 3 (que diz de quem era o documento) e o caso está resolvido sem interrogar nenhum suspeito.

### 2.4 Teia Social e Segredos em Três Camadas
* **Previsão no Design:** Suspeitos com relacionamentos cruzados, pequenas infrações não letais (*red herrings*) e confissões sob estresse.
* **Realidade no Código e Experiência:**
  - O modelo de 3 Tiers no banco (`tier1_alibi`, `tier2_petty_crime`, `tier3_truth`, `cross_suspicion`) é brilhante em teoria.
  - Porém, no interrogatório (`internal/server/handlers.go`, linha 427), se o jogador não usar o LLM online ou a chave falhar, a lógica de estresse usa um cálculo rígido baseado em palavras-chave na pergunta.
  - Além disso, os suspeitos inocentes possuem `tier2_petty_crime`, mas essas confissões raramente são exploradas pelo jogador porque as pistas já entregam o culpado antecipadamente.

---

## 3. Tabela de Pontos Críticos de Fricção Lúdica e Balanceamento

| ID | Componente Lúdico | Comportamento Atual | Fricção / Falha de Design | Nível de Severidade |
| :---: | :--- | :--- | :--- | :---: |
| **FL-01** | **Exposição de Pistas (Spoiler Clue)** | As descrições das pistas no `weaver.go` contêm o nome explícito do assassino ("O álibi de X é falso"). | Destrói o mistério e a dedução; o jogador ganha lendo a lista de pistas na largada. | **CRÍTICO** |
| **FL-02** | **Descoberta Passiva de Pistas** | Todas as pistas já vêm desbloqueadas e visíveis desde o início da investigação. | Elimina o prazer da exploração forense e busca em locais (*scenes*). | **ALTO** |
| **FL-03** | **Validação de Evidências na Acusação** | O backend só checa `len(clue_ids) == 2`. Nenhuma validação semântica é feita no Go sobre a coerência das pistas. | O jogador pode anexar pistas desconexas e o juiz aceita o mandado normalmente. | **ALTO** |
| **FL-04** | **Economia de Tempo Excessivamente Ampla** | 420 minutos com custo de 15 min por pergunta (28 ações). Nenhum caso exige mais de 8 a 10 ações. | O relógio perde a sensação de perigo e urgência. | **MÉDIO** |
| **FL-05** | **Confronto de Pistas no Interrogatório** | Confrontar uma pista apenas compara substrings ("adaga", "alibi", "sangue"). | Não exige que o jogador aponte *qual contradição* existe entre a fala do suspeito e a evidência. | **MÉDIO** |
| **FL-06** | **Envelopes Detective Night Estáticos** | Clicar em "Examinar Documentos" abre um bloco de texto JSON cru na tela. | Quebra a metáfora da mesa noir diegética e desestimula a leitura imersiva. | **MÉDIO** |

---

## 4. Comparativo com Benchmarks de Investigação

```
+---------------------+-------------------------------+-----------------------------------+-------------------------------------+
| Jogo de Referência  | Mecânica Chave de Sucesso     | Como Resolve a Dedução            | Oportunidade para o Detiv           |
+---------------------+-------------------------------+-----------------------------------+-------------------------------------+
| Return of the       | Validação em Bloco (3 Fates)  | Impossibilita chute unitário;     | Exigir que as 2 pistas anexadas ao  |
| Obra Dinn           | e Dedução Visual              | exige que 3 conclusões casem      | mandado sejam validadas em conjunto |
|                     |                               | perfeitamente.                    | com a arma e o motivo do suspeito.  |
+---------------------+-------------------------------+-----------------------------------+-------------------------------------+
| Ace Attorney        | Confrontação de Depoimento    | O jogador seleciona a declaração  | Permitir "Objeção" sobre a frase    |
| (Gyakuten Saiban)   | ("Contradição!")              | específica da testemunha e bate   | exata do álibi do suspeito usando   |
|                     |                               | a prova contrária.                | a pista do relatório.               |
+---------------------+-------------------------------+-----------------------------------+-------------------------------------+
| Papers, Please      | Modo de Discrepância          | O jogador clica em 2 fatos que se | O jogador cruza o depoimento de A   |
|                     | ("Highlight Discrepancy")     | negam mutualmente na tela da mesa.| com o depoimento de B ou com o doc. |
+---------------------+-------------------------------+-----------------------------------+-------------------------------------+
| Her Story           | Busca Ativa & Desbloqueio     | Nenhuma informação vem de graça;  | Pistas começam "bloqueadas" ou      |
|                     | Progressivo                   | o jogador precisa pesquisar e     | "ocultas" nos cômodos e envelopes,  |
|                     |                               | interrogar para liberar nós.      | custando tempo para inspecionar.    |
+---------------------+-------------------------------+-----------------------------------+-------------------------------------+
```

---

## 5. Recomendações de Refinamento de Game Design & Regras

Para elevar o Detiv ao patamar dos clássicos de dedução investigativa, recomendamos as seguintes intervenções prioritárias para as próximas fases:

### R1. Anonimizar e Desacoplar as Pistas no Weaver (Correção Urgente de Spoiler)
- **Regra:** Nenhuma pista deve citar o nome do culpado como autor da mentira.
- **Implementação:**
  - A Pista 2 deve relatar o fato objetivo do mundo:  
    *Exemplo:* `"Registro de Manutenção da Portaria: O Salão de Música esteve lacrado para conserto de vazamento das 21h às 04h."`
  - O álibi do suspeito diz:  
    *Exemplo:* `"Eu estive a noite toda lendo no Salão de Música com uma taça de vinho."`
  - **O Eureka nasce na mente do jogador:** Ele lê o depoimento, lê o registro e conecta: *"Vincent disse que estava no salão, mas o salão estava trancado! Ele mentiu!"*.

### R2. Descoberta Ativa de Pistas com Economia de Exploração
- **Regra:** No início do caso, apenas a cena do crime imediata (Corpo + Arma aparente) está disponível.
- **Implementação:**
  - As demais pistas (Documentos no cofre, pegadas no jardim, registros na portaria) aparecem como locais com a opção: `[Examinar Local - Custa 30 min]`.
  - Isso faz com que os 420 minutos ganhem peso real: visitar todos os locais consome tempo precioso, forçando o jogador a priorizar pistas com base nos depoimentos dos suspeitos.

### R3. Validação Algorítmica das 2 Pistas na Acusação
- **Regra:** Para o juiz acatar o mandado de prisão, as duas pistas anexadas devem pertencer ao grafo lógico do caso:
  1. Uma pista que refuta o álibi do suspeito acusado (ou o conecta ao local).
  2. Uma pista que comprova a arma do crime ou o motivo do crime.
- Se o jogador anexar pistas irrelevantes, o veredito deve informar com precisão:  
  *"O Magistrado recusou o mandado: As pistas anexadas não comprovam a presença do suspeito ou a materialidade da arma no momento do crime!"*

### R4. Ajuste na Curva de Tensão do Relógio da Madrugada
- Reduzir o tempo total inicial de **420 min para 240 min** (das 23:00 às 03:00) ou aumentar o custo de ações investigativas:
  - Interrogar suspeito: **15 min**.
  - Inspecionar novo cômodo / recolher evidência: **30 min**.
  - Confrontar álibi: **20 min**.
  - Acusação incorreta: **45 min** de retratação com o delegado.
- Isso garante que uma partida média seja resolvida em cerca de 8 a 12 ações bem planejadas, criando suspense genuíno nos últimos 45 minutos.

### R5. Envelopes do Detective Night Formatados como Dossiês Táteis
- Substituir a visualização de JSON cru dos envelopes por mini-fichas datilografadas com selo policial, recortes de jornais e laudos do legista formatados em CSS, preservando a imersão da mesa de mogno 1940s.

---

## 6. Conclusão & Próximos Passos da Fase 1

O projeto Detiv tem uma fundação sólida de monolito e estética noir. Os maiores gargalos identificados nesta auditoria não são de código ou arquitetura, mas sim de **mecânica de dedução**: as pistas atualmente revelam o segredo antes de o jogador ter a chance de deduzi-lo.

Com o desacoplamento dos textos de pistas no `Weaver`, a validação lógica das 2 pistas na acusação e a descoberta ativa de evidências nos cômodos, o Detiv alcançará a excelência lúdica de mistérios aclamados como *Return of the Obra Dinn* e *Ace Attorney*.

*Auditoria concluída em estrita conformidade com as diretrizes da Fase 1 (sem alterações nos arquivos de produção).*
