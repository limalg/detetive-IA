# Changelog - Banco Imobiliário

Todas as alterações notáveis do módulo Banco Imobiliário são documentadas neste arquivo.

## [banco-v0.2.0] - 2026-10-21
### Added
- Frontend standalone Banco Imobiliário 3D (`web/banco3d/` por Pixel):
  - Interface arcade zero-scroll (100vh) com layout responsivo e HUD (`index.html`).
  - Estilização com perspectiva 3D (38deg tilt), efeitos de vidro (glassmorphism) e paleta neon/art-déco (`css/style.css`).
  - Cliente de jogo interativo com suporte a dados animados, visualização de mapa SVG procedural da cidade e modo demo offline para testes visuais (`js/app.js`).

## [banco-v0.1.0] - 2026-10-17
### Added
- Backend e motor de regras do Banco Imobiliário (`internal/banco/` e `internal/server/`):
  - Motor de regras completo com movimentação, compra de propriedades, aluguel, hipotecas e falência.
  - Gerador de tabuleiros temáticos por cidade (São Paulo, Rio de Janeiro, Curitiba e procedural).
  - Sistema de negociações inteligentes de propriedades, empréstimos de emergência com IA e salas multiplayer.
  - Endpoints REST para controle de partidas (`/api/banco/*`).
- Interface web 2D integrada no hub principal (`web/templates/index.html`, `web/static/css/style.css`, `web/static/js/app.js`).
