# Changelog - Banco Imobiliário

Todas as alterações notáveis do módulo Banco Imobiliário são documentadas neste arquivo.

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
