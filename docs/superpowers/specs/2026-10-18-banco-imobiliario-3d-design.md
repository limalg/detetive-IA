# Especificação de Design: Banco Imobiliário 3D por Localização (Módulo Independente)

**Data:** 18 de Outubro de 2026  
**Status:** Em Revisão (Builder Check)  
**Tamanho:** Projeto com Documento de Design  

---

## 1. Visão Geral & Intenção

O **Banco Imobiliário 3D** é um jogo de tabuleiro multiplayer online em tempo real onde qualquer cidade ou endereço do mundo é transformado dinamicamente em um tabuleiro de Monopoly 3D personalizado. 

Diferente de versões genéricas, o anfitrião digita uma cidade (ex: *"Curitiba, PR"*, *"Recife, PE"* ou *"Tóquio"*), a Inteligência Artificial mapeia as principais ruas, bairros e pontos turísticos locais para compor os 40 espaços do tabuleiro, e os amigos conectados na sala realizam uma votação relâmpago para eleger o local mais icônico, que assumirá a posição de maior prestígio e aluguel do jogo.

O projeto é um **módulo independente** do Detetive IA, preservando integralmente o código e funcionalidades existentes no repositório.

---

## 2. Objetivos, Não-Objetivos e Anti-Objetivos

### Objetivos (Goals)
1. **Zero Rolagem (Viewport 100vh):** Toda a mesa 3D, peões, dados e botões de comando cabem na tela de uma só vez, tanto no PC quanto no smartphone.
2. **Geração Dinâmica por IA:** Identificar bairros populares, intermediários e nobres da cidade, organizando-os nos grupos clássicos de cores, além de transportes e serviços públicos da região.
3. **Votação Pré-Jogo:** Votação rápida de 15 segundos entre os jogadores conectados para coroar o ponto mais icônico da cidade.
4. **Mesa 3D Arcade (Opção C):** Visão superior/isométrica panorâmica, dados físicos 3D jogados no centro do tabuleiro e peões físicos coloridos.
5. **Multiplayer Online via Link:** Partidas sincronizadas em tempo real via WebSockets através de um link único de sala (`/sala/:id`), sem necessidade de download ou cadastro demorado.
6. **Textos Curtos e Objetivos:** Toda a comunicação é feita com microtextos, cartões diretos ("Comprar por R$ 350? [Sim] [Não]") e status visuais instantâneos.

### Não-Objetivos (Non-Goals)
1. **Não tocar no Detetive IA:** Manter as rotas, templates e lógica do Detetive IA intactos.
2. **Economia Real:** Não há moedas reais ou microtransações; todo o dinheiro é fictício.
3. **Regras Heterodoxas Complexas:** Manter as regras consagradas e intuitivas do Banco Imobiliário / Monopoly (compra de imóveis, pagamento de aluguel, prisão, sorte/revés e falência).

### Anti-Objetivos (Anti-Goals)
1. **Poluição Visual e Textos Longos:** Proibido parágrafos extensos de história ou regras intrusivas.
2. **Engines 3D Pesadas:** Proibido carregar motores pesados (como builds Unity WebGL de centenas de megas) que demoram a abrir no celular. Usar Three.js / WebGL leve com carregamento instantâneo.

---

## 3. Decisões do Usuário

- **Estilo de Interface:** Opção C (Mesa 3D Arcade, visão superior das peças e do tabuleiro, dados físicos centrais, zero scroll).
- **Mapeamento:** IA gera as sugestões de ruas e bairros a partir do endereço/cidade digitado.
- **Votação Comunitária:** Votação no lobby para definir o ponto mais icônico da cidade.
- **Multiplayer:** Link de sala online compartilhável (cada jogador no seu navegador).
- **Arquitetura de Código:** Módulo independente dentro do repositório Detiv.

---

## 4. O que Fica a Critério do Construtor (Builder's Calls)

- **Estrutura de Pastas:** Criação do módulo dedicado em `cmd/banco3d/` e `web/banco3d/` (ou pacote isolado `internal/banco3d/`).
- **Engine 3D:** Three.js leve com geometria procedural para o tabuleiro, peões e dados com física leve (`cannon-es` ou simulação de dados via Euler/impulso procedural).
- **Formato da Geração por IA:** Prompt estruturado em JSON com fallback local pré-definido para cidades principais caso a API esteja sem conexão ou chave.
- **Orquestração de Salas:** Gerenciador de salas em memória em Go (`sync.Map`) com WebSockets para broadcast em broadcast de baixa latência (<50ms).

---

## 5. Fluxo da Experiência do Jogador

1. **Criação da Sala:**
   - O anfitrião digita a cidade ou endereço (ex: *"Belo Horizonte"*).
   - O sistema gera a sala e devolve o link curto.
2. **Lobby & Entrada:**
   - Cada amigo abre o link, digita o apelido e escolhe a cor do seu peão.
   - A IA apresenta a lista de pontos mapeados.
3. **Votação do Ponto Icônico (15s):**
   - Todos os jogadores conectados recebem 3 a 4 opções e votam em tempo real.
   - O vencedor é posicionado no ponto mais nobre do tabuleiro.
4. **Partida 3D (Loop de Turno):**
   - Jogador da vez clica em *"Rolar Dados"*.
   - Os dados 3D rolam fisicamente no meio do tabuleiro.
   - O peão caminha casa por casa até o destino.
   - Caixa de ação objetiva: *"Você caiu em [Rua]. Comprar por R$ X? [Comprar] [Passar]"*.
   - Vez passa suavemente para o próximo amigo conectado.
