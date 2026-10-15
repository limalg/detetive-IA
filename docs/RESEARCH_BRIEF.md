# Briefing de Pesquisa & Game Design: Detetive IA

## 1. Visão Geral e Objetivo
Detetive IA é um jogo de mistério investigativo procedural em estilo Cinema Noir.
Atualmente, o projeto é um monolito em Go puro (<15MB RAM em execução, SQLite com WAL embutido, web responsiva PWA, 100% testado).

### O Momento "Eureka" Desejado pelo Criador:
1. **Entrada Personalizada do Jogador:** O jogador descreve a si mesmo (verdadeira ou ficticiamente: profissão, segredos, hábitos, relacionamentos).
2. **Geração Dinâmica do Caso em Torno dos Jogadores:** O motor do jogo tece uma trama de assassinato original em que ELE, seus amigos e bots com personalidades ricas são suspeitos. Todos recebem conexões plausíveis com a vítima, álibis com contradições sutis, segredos ocultos e pistas materiais.
3. **Se Solo ou Poucos Jogadores:** A IA preenche os papéis restantes com Bots que possuem histórias ricas, álibis e motivações verossímeis e coerentes com o caso.
4. **Sistema de Partidas por Código de Sala:** Entrada simples estilo Jackbox/Among Us com código de 4 a 6 letras (ex: `NOIR-87`), tela de lobby para aguardar amigos e sincronização de início.
5. **Ativos Audiovisuais (Higgsfield AI):** Temos um portal conectado no Maestri com 10 tokens na plataforma Higgsfield AI para gerar imagens/animações (retratos de personagens falando/expressões, armas do crime, locais do crime).

---

## 2. Perguntas & Tópicos de Pesquisa (Fóruns e Referências na Web)
Pesquise referências em Reddit (r/gamedev, r/boardgames), itch.io, Steam, Jackbox Games, Town of Salem, Blood on the Clocktower, Shadows of Doubt, Return of the Obra Dinn, Her Story, Clue/Detetive:

### Tópico 1: Geração Procedural de Mistérios (Graph-Based Deduction)
- Como transformar descrições livres de jogadores + bots em uma rede lógica consistente de dedução (Information Dependency Graph).
- A aplicação da "Three-Clue Rule" (cada fato vital deduzível por 3 caminhos: físico, testemunhal, documental).
- Como gerar contradições lógicas em álibis sem alucinação da LLM.

### Tópico 2: Lobbies e Códigos de Sala (Jackbox / Among Us Style)
- Melhores práticas de UX para mobile/desktop em jogos de dedução social.
- Como lidar com a entrada de dados (auto-descrição dos jogadores) durante a fase de lobby.
- Sincronização em tempo real (SSE vs WebSockets vs Polling leve) dentro do limite de memória de 1GB RAM.

### Tópico 3: Estratégia para os 10 Tokens do Higgsfield AI
- Como extrair o máximo valor visual dos 10 tokens disponíveis:
  - Retratos expressivos com estilo Cinema Noir consistente.
  - Animação de fala/reação (ex: micro-animações de interrogação).
  - Imagens de cena do crime e evidências periciais.
- Recomendações de prompts e modelo ideal no Higgsfield.

### Tópico 4: Dinâmica de Interrogatório Social & Blefe
- Como estruturar o interrogatório para que jogadores humanos possam blefar ou se defender, enquanto bots respondem com base estrita no grafo de conhecimento do caso.
- Como evitar que o culpado confesse facilmente sem pistas que o contradigam.

---

## 3. Instrução para o Oracle
1. Realize pesquisas na web utilizando suas ferramentas.
2. Formule um relatório completo contendo:
   - Resumo das melhores referências encontradas.
   - Propostas de mecânicas com prós, contras e viabilidade técnica para Go + SQLite.
   - Plano de priorização (quais ideias devem ser implementadas imediatamente).
3. Ao finalizar, envie seu relatório para o Orquestrador via:
   `maestri ask "Antigravity" "<resumo do relatorio>"`
