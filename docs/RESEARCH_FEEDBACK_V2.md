# Briefing de Pesquisa: Aprofundamento Narrativo, Tensão de Tempo e UI Imersiva Noir

## 1. Problemas Identificados pelo Jogador / Criador
Após os primeiros testes na versão v0.2, o criador destacou pontos críticos de melhoria na experiência:

1. **Problema do "Chute Infinito" (Brute Force de Acusação):**
   - *"Percebi que eu posso simplesmente sair chutando um monte de arma do crime e motivos até ganhar, senti que seria bom ter tempo pra investigação."*
   - O jogador precisa sentir urgência, pressão e risco real ao emitir uma acusação.

2. **Interface Muito Simples / Padronizada:**
   - *"Eu acho essa interface muito simples e padronizada."*
   - A interface precisa abandonar a aparência de cartões padrão e se transformar em uma **verdadeira mesa de investigação dos anos 1940**: dossiê de couro, máquina de escrever, fotos presas com clipes, carimbos, iluminação de persianas e sombras dramáticas.

3. **Falta de Profundidade e História dos Personagens:**
   - *"Algo que também senti falta foi a interação com os personagens, o fato de faltar história pra cada um deles e tudo mais."*
   - Os personagens precisam de laços dramáticos profundos com a vítima e entre si (ciúmes, chantagens mútuas, dívidas de submundo), com segredos em camadas que desabam durante o interrogatório.

---

## 2. Perguntas & Tópicos para o Oracle Pesquisar

### Tópico A: Mecânicas de Tensão, Tempo e Fim de Partida
- Como jogos de mistério de referência (Ace Attorney, Return of the Obra Dinn, Shadows of Doubt, Sherlock Holmes Consulting Detective, Darkside Detective, The Case of the Golden Idol) lidam com:
  - **Relógio da Noite / Limite de Turnos**: Tempo real vs tempo por ação (cada interrogatório ou busca consome 30 minutos na noite).
  - **Penalidade por Falsa Acusação**: Mandados limitados (ex: 2 mandados antes de ser afastado do caso pelo chefe de polícia), demissão ou perda de credibilidade.
  - **Requisito de Provas para Acusar**: Exigir vincular pelo menos 2 pistas que sustentem a dedução antes de o juiz aceitar emitir o mandado.

### Tópico B: UI Cinematográfica Noir Imersiva
- Referências estéticas em UI de jogos noir (L.A. Noire, Grim Fandango, Wolf Among Us, Disco Elysium, Papers Please):
  - Metáforas visuais táteis: Dossiê confidencial amarelado, fotos Polaroid/P&B com bordas desgastadas, máquina de escrever com cursor mecânico, lâmpada de mesa articulada, cinzeiro, efeitos de sombra de persiana veneziana em CSS puro.
  - Como manter o design ultraleve (<30MB RAM) sem sobrecarregar o PWA com bibliotecas pesadas.

### Tópico C: Rede Social e Segredos em Camadas dos Personagens
- Como estruturar personas com profundidade procedural:
  - **Teia de Relações**: Suspeito A chantageia Suspeito B; Suspeito C sabe um segredo vergonhoso de Suspeito A.
  - **Segredos em Camadas (Layered Secrets)**:
    - Camada 1 (Fachada pública e álibi falso superficial).
    - Camada 2 (Pequeno delito ou vício que ele confessa ao ser pressionado, tentando desviar da acusação de homicídio).
    - Camada 3 (A verdade irrefutável exposta pela evidência forense).
  - Como a LLM e o fallback podem modular as falas com dialetos e personalidades únicas.

---

## 3. Entrega do Oracle
- Elabore um relatório estruturado em `docs/RESEARCH_FEEDBACK_REPORT.md`.
- Apresente propostas práticas priorizadas com prós, contras e viabilidade técnica para Go + SQLite.
- Ao concluir, notifique o Orquestrador via:
  `maestri ask "Antigravity" "<resumo do relatorio>"`
