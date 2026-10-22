# Changelog - Banco Imobiliário

Todas as alterações notáveis do módulo Banco Imobiliário são documentadas neste arquivo.

## [banco-v0.7.1] - 2026-10-22
### Fixed & Added
- **Resolução de `place_id` e Extração de Locais Reais (`places.go`)**:
  - Resolução robusta de `place_id` (ex: `osm_420024688` para Londrina, `pl_recife`, `pl_poa`) mesmo quando o parâmetro `city` vem vazio.
  - Integração de busca geográfica via **Wikipedia Geosearch** e **Nominatim /details** extraindo patrimônios históricos, parques, teatros, praças e catedrais reais por proximidade de coordenadas.
  - 4 candidatos icônicos autênticos e 28 propriedades classificadas sem stubs procedurais (`"Centro Histórico de "`).
- **Catálogo Expandido de Cidades com Locais Reais (`generator.go` & `places.go`)**:
  - Presets dedicados e autênticos para **Londrina**, **Recife**, **Porto Alegre**, **Brasília**, **Fortaleza** e **Florianópolis**, somando-se a Curitiba, São Paulo, Rio de Janeiro, Belo Horizonte, Salvador, Nova York e Paris.
  - Exemplos autênticos integrados:
    - **Londrina**: Lago Igapó, Calçadão de Londrina, Gleba Palhano, Av. Higienópolis, Catedral de Londrina.
    - **Recife**: Rua do Bom Jesus, Marco Zero, Praia de Boa Viagem, Jaqueira, Derby, Casa Forte.
    - **Porto Alegre**: Orla do Guaíba, Redenção, Moinhos de Vento, Mercado Público de POA, Cidade Baixa.
    - **Brasília**: Eixo Monumental, Esplanada dos Ministérios, Asa Sul/Norte, Ponte JK, Lago Sul.
    - **Fortaleza**: Av. Beira-Mar, Praia do Futuro, Dragão do Mar, Meireles, Aldeota.
    - **Florianópolis**: Ponte Hercílio Luz, Lagoa da Conceição, Jurerê Internacional, Praia da Joaquina.
- **Persistência de Salas em Disco (`internal/banco3d/hub.go`)**:
  - Mecanismo de snapshots em arquivo (`data/banco3d/rooms.json`, espelhado em `data/banco3d/rooms` e `data/banco3d_rooms.json`) com sincronização em `SaveToDisk()` e recuperação em `LoadFromDisk()`.
  - Tolerância total a F5 e reinicializações do servidor: jogadores, saldos, propriedades compradas, eventos e prazos de decisão pendentes são restaurados na íntegra.
  - Modo in-memory (`:memory:`) para isolamento hermético nas suítes de testes.

## [banco-v0.7.0] - 2026-10-22
### Added
- **Inteligência Artificial dos Bots (`internal/banco3d/bot.go`)**:
  - Motor de tomada de decisão com parâmetros específicos por dificuldade (fácil, normal e difícil).
  - Reserva mínima de caixa calibrada: R$ 100 (fácil), R$ 250 (normal) e R$ 400 (difícil).
  - Regra de squeeze estratégico no nível difícil (compra agressiva para completar grupos de cor).
  - Construção uniforme de casas em grupos completos e avaliação proporcional de propostas de troca.
  - Execução automatizada de turnos de bots via `ExecuteBotTurn` com rolagem, compra, construção e passagem automática de vez.
- **Transição de Turno Automática & Janela de Decisão**:
  - Eliminação da dependência de botão manual "Passar" para fluxo contínuo.
  - Abertura de janela de decisão de 8 segundos (`decision_deadline` e `pending_decision: "buy"`) ao aterrissar em propriedade comprável.
  - Expiração automática ou compra concluída passam a vez instantaneamente para o próximo participante.
- **Fluxo de Eventos para Animação (`events[]`)**:
  - Histórico ordenado e sincronizado de até 30 eventos no `room/state` (`roll`, `buy`, `rent`, `card`, `pass`, `build`, `trade`).
  - Metadados completos: `seq`, `player_id`, `dice: [d1, d2]`, posições `from`/`to`, `amount`, `tile` e texto descritivo.
- **Geocodificação e Busca de Locais Reais (`places.go`)**:
  - Endpoint `GET /api/banco3d/places/suggest?q=...` integrado ao OpenStreetMap / Nominatim com cache em memória, rate limiting (1 req/s) e fallback para catálogo offline.
  - Endpoint `GET /api/banco3d/places/landmarks?city=...` com extração e classificação de ruas, avenidas, praças, parques, estações e concessionárias.
  - Enriquecimento de tiles do tabuleiro com `image_kind`, `mortgage_value` e `description`.
- **Resiliência e Reconexão (F5)**:
  - Preservação da sala e reconexão idempotente em `room/join` via `player_id` persistido no cliente.

## [banco-v0.6.1] - 2026-10-22
### Added
- Arte visual do centro do tabuleiro (`web/banco3d/assets/centro-cidade.webp`, 219KB, 1152px):
  - Fundo diegético na área central do tabuleiro em substituição ao mapa SVG procedimental, com vinheta e poça escura sob o logotipo.
  - Camada de UI preservada com logo, dados e pilhas de cartas HTML sobrepostas.
  - Zero scroll mantido e medido em 1440x900 (centro y=641, base 750, barra 806).
  - Arte de capa (`capa-imperio-urbano.webp`, 146KB) integrada com overlay e blur no hub, lobby, votação e configuração.

## [banco-v0.5.2] - 2026-10-22
### Fixed
- Correção de vulnerabilidades de autorização e consentimento na negociação de trocas (`Trade`):
  - **Autorização estrita de resposta**: apenas o destinatário (`receiver_id == player_id`) tem permissão para aceitar (`action: accept`) ou recusar (`action: decline`) uma proposta; tentativas de terceiros ou do próprio remetente retornam estritamente `HTTP 403 Forbidden`.
  - **Consentimento de jogadores humanos**: campo explícito `Player.IsBot` (`internal/banco3d/game.go`) garantindo que apenas bots legítimos realizem auto-aceite; jogadores humanos com IDs como `p2`/`p3` recebem status `pending` e exigem consentimento explícito.
  - Testes automatizados TDD adicionados e validados cobrindo domínio (`room_test.go`) e camada HTTP (`banco3d_test.go`).

## [banco-v0.5.1] - 2026-10-22
### Added
- Protocolo assíncrono de negociação (`TradeProposal`):
  - Suporte integral ao payload do frontend (`give: {props, cash}`, `want: {props, cash}`) com `player_id` e `to_player_id`.
  - Fluxo de ações `propose`, `accept` e `decline` via `POST /api/banco3d/room/trade`.
  - Exposição de `pending_trade` em `room/state` para sincronização em tempo real e resolução automática de trocas com bots.

## [banco-v0.5.0] - 2026-10-22
### Added
- Baralhos de Cartas Sorte e Revés temáticas por cidade (`internal/banco3d/cards.go`):
  - Textos e contextos autênticos para Curitiba, São Paulo, Rio de Janeiro e catálogo genérico.
  - Efeitos clássicos: recompensa/multa monetária, avanço de posição, ida para a prisão, carta Saída Livre da Prisão, custos de reformas por casa/hotel e divisão de dividendos entre jogadores.
  - Sorteio e resolução automática em tempo de execução ao cair nas casas Sorte/Revés no `RollDice`.
- Negociação avançada em lote (`TradeExchange`):
  - Troca atômica de múltiplas propriedades e compensação financeira entre jogadores com validação de propriedades construídas e saldo.
  - Compatibilidade com chamadas simples e em lote no endpoint `POST /api/banco3d/room/trade`.
- Suporte a configurações personalizadas de partida (`RoomConfig`):
  - Suporte a `config` em `POST /api/banco3d/room/create` (limite de 2 a 6 jogadores, saldo inicial customizado, tempo de turno de 15s a 90s, bots e regras de prisão).
  - Botões do Hub de Jogos direcionando para `/banco/` (Jogar) e `/banco/?config=1` (Personalizar).

### Removed
- Remoção completa da interface 2D antiga do Banco Imobiliário (`templates/index.html`, `static/js/app.js`, `static/css/style.css`).
- Remoção das rotas legadas `/api/banco/*` e do pacote antigo `internal/banco/`.

## [banco-v0.4.0] - 2026-10-22
### Added
- Integração Fullstack Banco Imobiliário 3D (`web/banco3d` + `internal/banco3d`):
  - Fluxo completo validado: criação de sala por cidade, link compartilhável (`?sala=ID`), lobby, votação relâmpago de 15s e transição para tabuleiro de 40 casas.
  - Vencedor da votação atribuído dinamicamente à casa 39 (Rainha, R$ 800) com coroa dourada.
  - Reconexão instantânea de jogador por sessão (`localStorage`) ao atualizar o navegador.
  - Rotação de turnos, rolagem de dados (`d1`/`d2`), compra de propriedades, passagem de vez e auto-preenchimento de bots ("Gabriel", "Larissa", "Pedro") em partidas solo.
  - Métodos de backend para construção de casas (`BuildHouse`) e negociação de propriedades (`TradeProperties`) com testes unitários.

## [banco-v0.3.0] - 2026-10-21
### Added
- Módulo independente de backend 40 casas (`internal/banco3d/`):
  - Modelo canônico de domínio de 40 casas (`board.go`) com 8 grupos de cores, estações, utilitários, impostos e casa rainha (39).
  - Máquina de estados completa (`game.go`): turnos rápidos, bônus de passagem/queda no GO, cálculo de aluguel por grupo/casas e detecção de falência.
  - Gerador de cidades com busca por nome e catálogo offline enriquecido (`generator.go`) para SP, Rio, Curitiba, BH, Salvador, NY, Paris e gerador procedural, fornecendo 4 candidatos a ponto icônico.
  - Sistema de votação relâmpago pré-jogo de 15 segundos (`voting.go`) com desempate determinístico e atribuição à propriedade de maior valor (índice 39).
  - Hub de salas e gerenciamento multiplayer (`room.go`, `hub.go`) com suporte a reconexão por ID de jogador sem reiniciar partida e timeout de turno inativo de 30 segundos.
  - Endpoints REST em `internal/server/banco3d_handler.go` (`/api/banco3d/city`, `/api/banco3d/room/*`).

## [banco-v0.2.0] - 2026-10-21
### Added
- Frontend standalone Banco Imobiliário 3D (`web/banco3d/` por Pixel):
  - Interface arcade zero-scroll (100vh) com layout responsivo e HUD (`index.html`).
  - Estilização com perspectiva 3D (30deg tilt), efeitos de vidro (glassmorphism) e paleta neon/art-déco (`css/style.css`).
  - Cliente de jogo interativo com suporte a dados animados, visualização de mapa SVG procedural da cidade e modo demo offline para testes visuais (`js/app.js`).

## [banco-v0.1.0] - 2026-10-17
### Added
- Backend e motor de regras do Banco Imobiliário (`internal/banco/` e `internal/server/`):
  - Motor de regras completo com movimentação, compra de propriedades, aluguel, hipotecas e falência.
  - Gerador de tabuleiros temáticos por cidade (São Paulo, Rio de Janeiro, Curitiba e procedural).
  - Sistema de negociações inteligentes de propriedades, empréstimos de emergência com IA e salas multiplayer.
  - Endpoints REST para controle de partidas (`/api/banco/*`).
- Interface web 2D integrada no hub principal (`web/templates/index.html`, `web/static/css/style.css`, `web/static/js/app.js`).
