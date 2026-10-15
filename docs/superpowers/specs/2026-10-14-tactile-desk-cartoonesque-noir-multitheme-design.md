# Design Document: Mesa Tátil Noir, Retratos Cartunescos Sérios, Trilha Diegética e Múltiplos Temas

**Data:** 2026-10-14  
**Versão:** v0.4-design  
**Status:** Aprovado para Planejamento  
**Autor:** Antigravity (com Oracle, Raven, Buster, Cipher, Tracker)

---

## 1. Visão Geral & Intenção (The Why)

O usuário avaliou a versão v0.3 e identificou pontos cruciais de melhoria:
1. **Interface Simplória:** Necessidade de sair do padrão genérico e adotar uma **mesa tátil** de investigação (estilo *Papers, Please* e *Strange Horticulture*), com papéis físicos, carimbos, relógio de bolso analógico e lupa de perícia.
2. **Direção de Arte dos Personagens:** Adoção de um estilo **cartunesco sério** inspirado em *Graphic Novels Noir* (*Blacksad*, *The Wolf Among Us*, *Dick Tracy*), mantendo a sobriedade dramática e o clima de suspense sem cair em caricaturas cômicas.
3. **Sala de Interrogatório Centralizada:** O retrato do suspeito deve ficar **perfeitamente centralizado** sob o foco de luz da sala, acima da caixa de diálogo, com micro-animações expressivas (olhares desconfiados, fumaça de cigarro, gotas de suor sob estresse elevado).
4. **Múltiplos Temas de Crime & Densidade de Suspeitos:** Superar a limitação de um único caso ("Vale das Sombras") implementando 4 temas procedurais ricos e um seletor para escolher de 3 a 6 suspeitos por partida.
5. **Correção de Bloqueios Críticos:**
   - Correção imediata do reinício de partida que travava o jogo afirmando que os mandados acabaram.
   - Mapeamento visual e suporte completo para todas as armas do crime (Adaga, Cálice de Cianeto, Revólver Silenciado, Castiçal de Bronze e Navalha Pericial).
6. **Imersão Sonora Diegética:** Trilha sonora e efeitos sintetizados via Web Audio API (tique-taque de relógio, carimbada seca com tinta, farfalhar de papel, máquina de escrever e chuva na janela).

---

## 2. Metas, Não-Metas e Anti-Metas

### 2.1 Metas (Goals)
- **Mesa Tátil Imersiva:** Escrivaninha de madeira texturizada, documentos de papel Kraft manipuláveis, carimbo de tinta para emissão de mandado, relógio de bolso analógico funcional e lupa de aumento.
- **Interrogatório Centralizado & Expressivo:** Foco visual centralizado no suspeito, com micro-animações CSS (respiração, piscar de olhos, fumaça de cigarro e suor em estresse > 70%).
- **Multi-Temas Procedurais:** Motor `Weaver` com 4 temas ricos (Mansão, Expresso da Meia-Noite, Ópera Municipal, Cassino Clandestino).
- **Seletor de Suspeitos:** Configuração de 3 a 6 suspeitos por partida (solo ou sala cooperativa).
- **Correção da Economia de Tempo & Mandados:** Garantir que cada nova partida crie uma nova investigação isolada com 420 minutos e 2 mandados intactos.
- **Sonoplastia Diegética Zero-Dependency:** Módulo de áudio em Web Audio API nativo do navegador, sem downloads pesados ou bibliotecas externas.
- **Memória & Performance:** Manter a pegada monolítica em Go estritamente abaixo do teto de 60 MB de RAM.

### 2.2 Não-Metas (Non-Goals)
- Motor 3D WebGL pesado (a ilusão tátil será construída com CSS3, SVG, sombras dinâmicas e transições fluidas).
- Dublagem completa por voz (manteremos o diálogo textual estilizado como máquina de escrever com áudio sintetizado).

### 2.3 Anti-Metas (Anti-Goals)
- Não transformar o jogo em desenho animado infantil ou pastelão; o tom deve ser maduro, tenso e investigativo.
- Não introduzir dependências externas de CGO ou servidores de banco de dados pesados (permanecer 100% puro Go com modernc.org/sqlite).

---

## 3. Arquitetura e Decisões Técnicas

### 3.1 Backend & Motor Procedural (`internal/game/weaver.go`)
- **Estrutura de Temas (`ThemeDefinition`):**
  - **Mansão do Vale das Sombras:** Crime de herança e falsificação de testamento na alta sociedade.
  - **Expresso da Meia-Noite:** Assassinato de diplomata a bordo de trem de luxo bloqueado pela nevasca.
  - **A Última Ária na Ópera:** Crime passional e veneno nos camarins do Teatro Municipal.
  - **A Dívida do Cassino Clandestino:** Execução mafiosa e queima de arquivo no cofre subterrâneo.
- **Seletor de Suspeitos:** Função `WeaveCaseWithTheme(players []room.Player, themeID string, suspectCount int)` gerando grafos dedutivos rigorosos com a Regra das Três Pistas independentemente do número de suspeitos (3 a 6).
- **Armas e Pistas Periciais:** Pool expandido com 5 armas (Adaga, Cianeto, Revólver Silenciado, Castiçal, Navalha), cada uma associada a uma pista física detalhada e metadados de imagem.

### 3.2 Correção do Ciclo de Vida da Investigação (`internal/game/game.go` e `handlers.go`)
- **Causa Raiz do Bug:** `GetOrCreateInvestigation` buscava o último registro no SQLite sem filtrar `status = 'in_progress'`, reutilizando o registro falho da partida anterior. Além disso, o modo Solo sempre carregava o caso fixo ID 1.
- **Solução Arquitetural:**
  - Implementar endpoint `POST /api/game/solo/start` que aciona o `Weaver`, gera um novo caso procedural com o tema e número de suspeitos escolhidos, e cria uma investigação virgem (`status = 'in_progress'`, `time_remaining_minutes = 420`, `warrants_remaining = 2`).
  - Filtrar `status = 'in_progress'` em `GetOrCreateInvestigation` para nunca herdar partidas encerradas.

### 3.3 Frontend & Mesa Tátil (`web/templates/index.html`, `web/static/css/style.css`, `web/static/js/app.js`)
- **Mesa Tátil:**
  - Escrivaninha de madeira com iluminação quente de abajur bancário (*banker lamp*).
  - Documentos organizados em abas de papel Kraft, com carimbo interativo para submissão da acusação.
  - Relógio de bolso diegético com mostrador e ponteiros analógicos.
- **Sala de Interrogatório Centralizada:**
  - Retrato do suspeito centralizado com moldura de camafeu noir e iluminação cônica.
  - Micro-animações em CSS: oscilação de fumaça (`@keyframes smokeRise`), tremor de estresse (`@keyframes stressPulse`), e partículas sutis.
  - Balão de diálogo com efeito de datilografia e chips de perguntas rápidas.
- **Motor de Áudio Diegético (`web/static/js/audio.js`):**
  - Sintetizador Web Audio API para reproduzir: clique mecânico de máquina de escrever a cada caractere, tique-taque ritmado do relógio, batida surda do carimbo no papel, e ruído de fundo de chuva/vento.

---

## 4. O que Fica a Critério dos Construtores
- As curvas de frequência sonora exatas do oscilador no sintetizador de áudio Web Audio API.
- Os tempos precisos das animações de transição de abas e carimbadas.
- O nome das sub-rotinas e testes unitários.
