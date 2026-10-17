# Auditoria Diagnóstica de QA, Testes, Chaos Engineering e Acessibilidade (WCAG)
**Projeto:** Detetive IA (Detiv)  
**Fase:** Fase 1 – Revisão Geral do Projeto (Auditoria Diagnóstica)  
**Líder Responsável:** Buster (Líder de QA, Testes, Chaos Engineering e Acessibilidade)  
**Data:** 17/10/2026  
**Status da Auditoria:** Concluída (Fase de Documentação e Diagnóstico – Sem alterações em arquivos de produção)

---

## 1. Diagnóstico Geral da Suíte de Testes e Acessibilidade

### 1.1 Panorama da Suíte de Testes Atual
A suíte atual de testes do **Detetive IA** é construída primariamente em Go padrão (`testing`), com integração em memória e arquivos de teste dedicados para pacotes internos e testes E2E com `httptest.NewServer`.

| Pacote / Diretório | Arquivos de Teste | Cobertura / Status | Diagnóstico Técnico |
| :--- | :--- | :--- | :--- |
| `internal/auth` | `auth_test.go` | 77.1% (PASS) | Boa cobertura de hash bcrypt, sessões SQLite e validação de credenciais. |
| `internal/db` | `db_test.go` | 78.4% (PASS) | Valida DDL, migrações e chaves estrangeiras com SQLite modernc.org. |
| `internal/game` | `game_test.go`, `weaver_test.go` | 57.4% (PASS) | **Ponto de Atenção:** Menor cobertura unitária do projeto. Caminhos de fallback do cliente IA, bifurcações raras no grafo de pistas e geração com múltiplos suspeitos possuem lacunas de teste. |
| `internal/room` | `room_test.go` | 77.4% (PASS) | Testa criação de códigos de 4 letras, concorrência básica com mutex e bloqueio de lobby. |
| `internal/server` | `server_test.go`, `admin_test.go` | 74.5% (PASS) | Cobre rotas REST principais, telemetria e endpoints de administração. |
| `internal/telemetry` | `telemetry_test.go` | 87.2% (PASS) | Alta cobertura de coleta de métricas, spans e logs estruturados. |
| `web` | `web_test.go` | 88.9% (PASS) | **Limitação Crítica:** Testa apenas se os templates e assets estáticos embutidos (`embed.FS`) contêm determinadas strings literais. Não valida parseamento DOM real, integridade de tags ou execução JS. |
| `tests/integration` | `e2e_test.go`, `room_e2e_test.go`, `detective_night_e2e_test.go`, `multitheme_e2e_test.go` | PASS (4 suítes) | Excelente robustez no nível de contrato HTTP. Todos usam `httptest.NewServer` isolado com SQLite temporário, validando jornadas completas de investigação. |
| `tests/chaos` | `chaos_test.go` | **FALHA em `go test ./...`** | **Falha Arquitetural Crítica:** `chaos_test.go` depende estritamente de um servidor ativo em `http://localhost:8080`. Se executado em ambiente CI ou sem o binário rodando em background, 100% dos testes de chaos falham por recusa de conexão (`connection refused`). Falta fallback in-process com `httptest.NewServer`. |

---

### 1.2 Diagnóstico de Acessibilidade (WCAG 2.1 AA / Vercel Web Interface Guidelines)
A interface visual do Detetive IA possui uma estética diegética rica e cinematográfica (Mesa de Detetive Noir dos anos 1940, Dossiê Manila Kraft, Dossiê Digital Detective Night, Lupa Pericial e Polaroids com alfinetes). No entanto, a auditoria revelou **graves lacunas de acessibilidade e ergonomia interativa**:

1. **Zoom Bloqueado no Mobile (Violação WCAG 1.4.4):** A meta tag `viewport` define `maximum-scale=1.0, user-scalable=no`, impedindo ampliação por usuários com baixa visão.
2. **Uso de `<div>` para Elementos Interativos:** Abas de autenticação, cartões de armas no mandado e abas do admin foram implementados com `<div>` contendo `addEventListener('click')`, tornando-os invisíveis para navegação via teclado e tecnologias assistivas.
3. **Ausência Universal de `:focus-visible` e Remoção de Outlines:** Os seletores `.form-input:focus` e `.chat-input:focus` usam `outline: none;` sem anel substituto. Botões, abas e cartões não possuem estilo para `:focus-visible`.
4. **Touch Targets Subdimensionados (Violação WCAG 2.5.5 / 2.5.8):** Botões como `#sound-toggle`, `.modal-close` e chips de perguntas possuem áreas de toque entre 24px e 30px, bem abaixo do teto ergonômico mínimo de **44x44px**.
5. **Ícones e Fechamento sem `aria-label`:** Botões de fechar modal (`&times;`) e botões com ícones não fornecem descrição textual para leitores de tela.
6. **Falta de Regiões Vivas (`aria-live`):** Atualizações temporais do Relógio da Madrugada, decremento de mandados de prisão, reações do medidor de estresse e transcrições de chat no interrogatório não notificam leitores de tela quando atualizados dinamicamente.
7. **Modais Inacessíveis:** Os modais não implementam `role="dialog"` ou `role="alertdialog"`, não possuem aprisionamento de foco (*focus trap*), não movem o foco ao abrir, não devolvem o foco ao elemento disparador ao fechar e (com exceção da lupa pericial) ignoram a tecla `Escape`.
8. **Contraste de Cor em Texto Secundário Noir (Violação WCAG 1.4.3):** A cor `--text-dim: #71717a` sobre `--bg-primary: #121214` gera contraste de **3.78:1**, abaixo dos 4.5:1 exigidos para texto normal.
9. **Ausência de Suporte a `prefers-reduced-motion`:** Não há media query de redução de movimento para desativar partículas de fumaça, suor do suspeito e transições de modais.
10. **Imagens sem Dimensões Explícitas (CLS):** Todas as tags `<img>` (estáticas e dinâmicas) carecem de atributos `width` e `height`, causando deslocamentos indesejados de layout.

---

## 2. Tabela de Problemas Encontrados e Soluções Recomendadas

Formato padronizado: `arquivo:linha - [Problema] -> [Solucao Recomendada]`

### 2.1 Templates HTML (`web/templates/index.html` e `admin.html`)

| Arquivo e Linha | Problema Encontrado | Solução Recomendada |
| :--- | :--- | :--- |
| `web/templates/index.html:5` | `maximum-scale=1.0, user-scalable=no` desativa o zoom do navegador, violando WCAG 1.4.4. | Remover `maximum-scale=1.0, user-scalable=no` e manter apenas `width=device-width, initial-scale=1.0`. |
| `web/templates/index.html:34` | Relógio da Madrugada `#night-clock` não possui `aria-live` nem descrição semântica legível por voz. | Adicionar `role="timer"`, `aria-live="polite"` e `aria-atomic="true"`, informando o tempo restante em formato audível. |
| `web/templates/index.html:41` | Crachá de Mandados `#warrants-badge` não notifica tecnologias assistivas ao decrementar mandados. | Adicionar `role="status"` e `aria-live="polite"` para alertar quando um mandado for gasto. |
| `web/templates/index.html:48` | Botão `#sound-toggle` possui touch target reduzido (~26px de altura) e falta `aria-pressed`. | Adicionar `min-height: 44px; min-width: 44px;`, atributo `aria-pressed="true"` e `aria-label="Alternar som ambiente"`. |
| `web/templates/index.html:50` | Botão `#pwa-install-btn` possui ícone sem `aria-hidden` e texto pouco descritivo. | Envolver emoji em `<span aria-hidden="true">📲</span>` e adicionar `aria-label="Instalar aplicativo Detetive IA no dispositivo"`. |
| `web/templates/index.html:52` | Indicador de conexão `#network-status` é puramente visual (círculo verde/vermelho). | Adicionar `role="status"` e texto oculto `<span class="sr-only" id="network-status-text">Conexão Online</span>`. |
| `web/templates/index.html:63-64` | Abas de login/registro são elementos `<div>` sem acessibilidade de teclado ou papéis ARIA. | Converter para contêiner `<div role="tablist">` com botões `<button role="tab" id="tab-login" aria-selected="true" class="auth-tab">`. |
| `web/templates/index.html:146` | Input `#cast-char-name` não possui tag `<label>` nem `aria-label` associado. | Adicionar `<label for="cast-char-name" class="sr-only">Nome do Personagem</label>` ou `aria-label="Nome do Personagem"`. |
| `web/templates/index.html:149` | Input `#cast-char-notes` não possui tag `<label>` nem `aria-label` associado. | Adicionar `<label for="cast-char-notes" class="sr-only">Notas do Personagem</label>` ou `aria-label="Notas do Personagem"`. |
| `web/templates/index.html:207` | Input `#room-code-input` não possui `<label>` ou `aria-label`. | Adicionar `<label for="room-code-input" class="sr-only">Código da Sala de 4 Letras</label>` ou `aria-label`. |
| `web/templates/index.html:223` | Botão `#btn-copy-room-link` possui touch target pequeno (~28px) e falta feedback de cópia audível. | Ampliar altura mínima para 44px e adicionar região viva para anunciar "Link copiado com sucesso". |
| `web/templates/index.html:247-252` | Navegação da mesa `#desk-workspace .nav-tabs` carece de papéis de acessibilidade `tablist`/`tab`. | Adicionar `role="tablist"` no `<nav>` e `role="tab"`, `aria-selected="true/false"`, `aria-controls="tab-content-..."` nos botões. |
| `web/templates/index.html:259` | `img#dossier-scene-img` não possui dimensões explícitas (`width`/`height`), gerando CLS. | Adicionar atributos `width="1200"` e `height="220"`, além de `fetchpriority="high"`. |
| `web/templates/index.html:296` | `img#crime-scene-banner-img` não possui atributos `width` e `height`. | Adicionar atributos `width="1200"`, `height="220"` e `loading="lazy"`. |
| `web/templates/index.html:346` | `<label class="form-label">` de provas não possui atributo `for`, não vinculando aos checkboxes. | Substituir por `<fieldset><legend class="form-label">Provas de Sustentação do Mandado...</legend>`. |
| `web/templates/index.html:374` | Botão `#close-interrogation-btn` contém apenas `&times;` sem texto alternativo. | Adicionar `aria-label="Fechar sala de interrogatório"` e garantir área de toque mínima de 44x44px. |
| `web/templates/index.html:381` | `img#interrogate-portrait-img` não possui atributos `width` e `height`. | Adicionar atributos `width="240"` e `height="320"`. |
| `web/templates/index.html:392-400` | Barra de estresse `#suspect-stress-container` não possui semântica de barra de progresso. | Adicionar `role="progressbar"`, `aria-valuenow="0"`, `aria-valuemin="0"`, `aria-valuemax="100"` e `aria-label="Nível de estresse do suspeito"`. |
| `web/templates/index.html:403-406` | Balão de fala `#interrogate-speech-balloon` não notifica tecnologias assistivas ao atualizar. | Adicionar `aria-live="polite"` para anunciar a resposta oral do suspeito. |
| `web/templates/index.html:409` | Transcrição de chat `#chat-transcript` não possui papel de histórico nem região viva. | Adicionar `role="log"`, `aria-live="polite"` e `aria-atomic="false"`. |
| `web/templates/index.html:414-418` | Botões `.chip` possuem área de toque reduzida (~28px de altura) e emojis sem isolamento. | Definir `min-height: 44px;`, isolar emojis com `<span aria-hidden="true">` e garantir contraste de foco. |
| `web/templates/index.html:431` | Input `#interrogation-input` não possui `<label>` associado. | Adicionar `<label for="interrogation-input" class="sr-only">Digite sua pergunta ao suspeito</label>`. |
| `web/templates/index.html:443` | Botão `#close-accuse-modal-btn` contém apenas `&times;` sem `aria-label`. | Adicionar `aria-label="Fechar confirmação de acusação"` e área de clique de 44x44px. |
| `web/templates/index.html:452` | Modal `#game-over-modal` não possui atributos de diálogo acessível nem captura de foco. | Adicionar `role="alertdialog"`, `aria-modal="true"`, `aria-labelledby="game-over-title"`, `aria-describedby="game-over-reason"`. |
| `web/templates/index.html:473` | Botão `#close-loupe-btn` contém apenas `&times;` sem `aria-label`. | Adicionar `aria-label="Fechar exame pericial com lupa"`. |
| `web/templates/index.html:479` | `img#loupe-img` não possui atributos `width` e `height`. | Adicionar atributos `width="400"` e `height="400"`. |
| `web/templates/admin.html:57-69` | Abas de administração `.admin-tab` são elementos `<div>` sem acessibilidade de teclado. | Converter para `<button role="tab" class="admin-tab">` agrupados em `<div role="tablist">`. |
| `web/templates/admin.html:191-197` | `<label>` de filtro de log não possui atributo `for="log-level-filter"`. | Adicionar `for="log-level-filter"` ao elemento `<label>`. |
| `web/templates/admin.html:203, 223` | Tabelas de log e spans OpenTelemetry não possuem `<caption class="sr-only">` para identificação assistiva. | Adicionar elemento `<caption>` descritivo em cada tabela. |

---

### 2.2 Estilos CSS (`web/static/css/style.css`)

| Arquivo e Linha | Problema Encontrado | Solução Recomendada |
| :--- | :--- | :--- |
| `web/static/css/style.css:14` | Variável `--text-dim: #71717a` possui contraste insuficiente de 3.78:1 contra `#121214`. | Atualizar `--text-dim` para `#94949e` (atingindo 5.1:1, conforme WCAG AA). |
| `web/static/css/style.css:137, 183, 260, 342, 444, etc.` | Uso repetitivo do anti-pattern `transition: all` em dezenas de classes, degradando renderização. | Substituir por transições explícitas: `transition: transform 0.2s ease, opacity 0.2s ease, background-color 0.2s ease;`. |
| `web/static/css/style.css:295-298` | `.form-input:focus` usa `outline: none;` sem anel substituto acessível. | Remover `outline: none;` e configurar anel com `:focus-visible`: `outline: 2px solid var(--accent-amber); outline-offset: 2px;`. |
| `web/static/css/style.css:567-575` | `.modal-close` não possui largura/altura fixas, resultando em touch target ~24px. | Adicionar `min-width: 44px; min-height: 44px; display: inline-flex; align-items: center; justify-content: center;`. |
| `web/static/css/style.css:879-882` | `.chat-input:focus` remove o outline sem foco visual acessível. | Implementar regra com `:focus-visible` e anel de foco destacado. |
| `web/static/css/style.css:892-901` | `.chip` possui padding `0.3rem 0.6rem`, gerando altura total de apenas ~30px. | Definir `min-height: 44px; min-width: 44px; display: inline-flex; align-items: center; justify-content: center;`. |
| `web/static/css/style.css (geral)` | Ausência de regras `:focus-visible` para `.btn`, `.nav-tab-btn`, `.polaroid-card` e `.weapon-card-item`. | Adicionar bloco global de foco com anel contrastante para teclado (`:focus-visible`). |
| `web/static/css/style.css (geral)` | Inexistência de `@media (prefers-reduced-motion: reduce)` em todo o arquivo. | Adicionar bloco que anula durações de transição e animação sob preferência do usuário. |

---

### 2.3 Lógica JavaScript (`web/static/js/app.js`)

| Arquivo e Linha | Problema Encontrado | Solução Recomendada |
| :--- | :--- | :--- |
| `web/static/js/app.js:964-984` | Botão `.btn-inspect-envelope` expande documentos sem atualizar `aria-expanded` e `aria-hidden`. | Adicionar `btn.setAttribute('aria-expanded', isExpanded)` e no contêiner `preview.setAttribute('aria-hidden', !isExpanded)`. |
| `web/static/js/app.js:1060` | Imagens dinâmicas de polaroids de suspeitos são criadas sem atributos `width` e `height`. | Adicionar `width="200" height="200"` na string do template literal. |
| `web/static/js/app.js:1186, 1237, 1301` | Imagens periciais de pistas e armas são inseridas sem `width` e `height`. | Adicionar dimensões explícitas em todas as tags `<img>` dinâmicas. |
| `web/static/js/app.js:1209-1213` | Apenas o modal da lupa pericial é fechado pela tecla `Escape`. Demais modais ignoram `Escape`. | Criar gerenciador centralizado de modais que fecha o modal superior ao pressionar `Escape` e devolve o foco. |
| `web/static/js/app.js:1296-1312` | Cartões de armas `.weapon-card-item` são elementos `<div>` sem suporte a teclado nem papéis de rádio. | Converter para `<button type="button" role="radio" aria-checked="false">` e adicionar suporte a setas/Enter. |
| `web/static/js/app.js:1332-1355` | A função `updateStressMeter` altera largura visual, mas não atualiza `aria-valuenow` nem texto assistivo. | Atualizar `setAttribute('aria-valuenow', clamped)` e sincronizar `aria-valuetext` com a legenda. |
| `web/static/js/app.js:1583-1620` | Checkboxes de pistas do mandado não anunciam via leitor de tela quando a cota de 2 pistas é atingida. | Atualizar região viva `#clues-selection-hint` notificando "2 de 2 pistas selecionadas; mandado pronto". |
| `web/static/js/app.js (geral)` | Não há captura de foco (*focus trap*) dentro de nenhum modal ao ser aberto. | Implementar captura de foco em loop (Tab e Shift+Tab restritos aos elementos interativos internos do modal). |

---

### 2.4 Suíte de Testes Existente (`tests/chaos/`, `internal/`, `web/`)

| Arquivo e Linha | Problema Encontrado | Solução Recomendada |
| :--- | :--- | :--- |
| `tests/chaos/chaos_test.go:41, 155, 321, 528, 566` | Todos os testes de Chaos dependem de `http://localhost:8080` rodando previamente; falham em CI. | Implementar função `getTestServerURL(t)` com fallback transparente para `httptest.NewServer(router)` quando o daemon local estiver inativo. |
| `internal/game/weaver_test.go` e `game_test.go` | Cobertura de 57.4% com caminhos de erro e fallbacks de IA não cobertos. | Criar testes unitários parametrizados com mocks do `AIClient` e casos extremos de grafos desconexos. |
| `web/web_test.go:280-330` | Testes baseados em busca literal de strings em templates (`strings.Contains`), sem validação semântica. | Adicionar testes com analisador sintático HTML (`golang.org/x/net/html`) para validar pareamento de labels e botões. |

---

## 3. Avaliação de Automação E2E com Playwright (Token-Efficient) e Casos Limite de Chaos

Para viabilizar uma esteira ágil de testes ponta a ponta sem onerar o consumo de tokens das IAs nem tornar a execução lenta, desenhamos uma estratégia de automação leve baseada no `playwright-cli` e testes headless.

### 3.1 Princípios de Eficiência de Tokens (Token-Efficient Playwright)
1. **Reutilização de Sessão (`storageState`):** Salvar o estado de autenticação em `auth.json` após o primeiro login bem-sucedido (`playwright-cli state-save auth.json`). Cada cenário de investigação carrega a sessão imediatamente via `playwright-cli state-load auth.json`, eliminando as telas de cadastro e login de cada teste.
2. **Localizadores Semânticos e Resilientes:** Proibir seletores hierárquicos frágeis (como `div > div:nth-child(2) > span`). Utilizar exclusivamente localizadores acessíveis do Playwright:
   - `page.getByRole('button', { name: 'Interrogar Suspeito' })`
   - `page.getByRole('tab', { name: 'Dossiê Manila' })`
   - `page.getByLabel('Código Secreto (Senha)')`
3. **Snapshots Compactos com Filtro de Profundidade:** No uso via CLI por agentes autônomos, utilizar snapshots limitados (`playwright-cli snapshot --depth=4`) e modo bruto (`--raw`), capturando capturas de tela e bounding boxes somente em caso de falha de asserção.
4. **Mocking Seletivo de Latência de Rede:** Interceptar chamadas pesadas ao backend quando necessário para simulação de desconexão sem congelar a suíte:
   ```javascript
   await page.route('/api/game/interrogate', route => route.abort());
   ```

---

### 3.2 Casos Limite de Chaos a Serem Cobertos

#### Caso Limite 1: Corrida do Relógio da Madrugada (Night Clock Depletion)
* **Objetivo:** Garantir que o limite diegético de 420 minutos (21:00 às 04:00) seja respeitado sem bugs de cálculo temporal ou partidas zumbis.
* **Cenário de Teste:**
  1. Iniciar partida solo no tema Mansão (Relógio marca 21:00, 420 minutos restantes).
  2. Disparar uma sequência de 14 interrogatórios rápidos consecutivos (cada um deduz 30 minutos).
  3. No 14º interrogatório, o tempo restante atinge exatamente 0 minutos (Relógio bate 04:00).
  4. **Asserções:**
     - O modal `#game-over-modal` deve abrir automaticamente na tela com foco capturado.
     - O título deve ser "FIM DE JOGO" e o subtítulo "Madrugada Esgotada".
     - Tentativas de enviar novas perguntas via API devem ser rejeitadas com HTTP 400.
     - O botão `#btn-restart-game` deve reiniciar a mesa limpa e redefinir o relógio para 21:00.

#### Caso Limite 2: Esgotamento de Mandados de Prisão (Warrants Depletion)
* **Objetivo:** Validar que o jogador só possui 2 chances periciais para acusar o suspeito antes de ter seu distintivo cassado pelo Tribunal.
* **Cenário de Teste:**
  1. Iniciar partida com 2 mandados ativos (badge marca `2/2`).
  2. Emitir primeiro mandado acusando o suspeito inocente (ou arma errada) com 2 pistas anexadas.
  3. O backend retorna veredito incorreto; o badge decresce para `1/2` e o tempo deduz 60 minutos.
  4. Emitir segundo mandado com acusação incorreta.
  5. O badge atinge `0/2`.
  6. **Asserções:**
     - O modal `#game-over-modal` deve disparar com a mensagem "Distintivo Cassado por Abuso de Autoridade".
     - O botão `#stamp-accuse-btn` deve ficar permanentemente desabilitado.
     - Não é possível mais consultar envelopes ou realizar interrogatórios na partida encerrada.

#### Caso Limite 3: Desconexão e Resiliência de Rede (Network Drop / Offline PWA)
* **Objetivo:** Verificar a persistência diegética e estabilidade visual sob perda de conectividade.
* **Cenário de Teste:**
  1. Carregar a mesa diegética com Dossiê, Polaroids e Evidências.
  2. Emular perda total de rede (`context.setOffline(true)`).
  3. **Asserções:**
     - O indicador `#network-status` deve alternar imediatamente para a classe `.offline` (vermelho).
     - O Service Worker (`sw.js`) deve servir todos os arquivos estáticos e imagens em cache sem quebrar o layout.
     - Ações de rede devem apresentar alerta amigável de reconexão sem fechar os modais ativos ou perder as anotações do detetive.
     - Ao restaurar a rede (`setOffline(false)`), o status deve retornar para online suavemente.

#### Caso Limite 4: Concorrência e Sobrescrita de Slots em Salas Multiplayer
* **Objetivo:** Validar a correção do bug de colisão de convidados (`userID = 2`) e garantir isolamento estrito de 8 jogadores.
* **Cenário de Teste:**
  1. Host cria sala e obtém código de 4 letras.
  2. 3 navegadores distintos entram como convidados anônimos.
  3. **Asserção:** A lista de jogadores no lobby deve conter 4 participantes distintos (Host + 3 convidados), sem que um jogador sobrescreva a persona do outro.

---

## 4. Plano de Implementação da Suíte de Testes E2E e Cobertura TDD

O plano de ação para as próximas fases seguirá rigorosamente o ciclo **Red-Green-Refactor (TDD)**:

```
[FASE 1: DIAGNÓSTICO (Concluída)]
       │
       ▼
[FASE 2: CRIAÇÃO DOS TESTES QUEBRADOS (RED)]
       ├── 1. Adicionar teste de fallback in-process em tests/chaos/chaos_test.go
       ├── 2. Criar suíte de testes de acessibilidade web_a11y_test.go
       └── 3. Criar cenários Playwright e2e (clock race, warrants, offline)
       │
       ▼
[FASE 3: IMPLEMENTAÇÃO DAS CORREÇÕES (GREEN)]
       ├── 1. Sanitizar viewport e converter <div> interativos para <button> semânticos
       ├── 2. Inserir anéis :focus-visible, min-size 44x44px e prefers-reduced-motion
       ├── 3. Implementar atributos ARIA (tablist, dialog, progressbar, aria-live)
       └── 4. Corrigir contraste de cores noir e dimensões de imagens (width/height)
       │
       ▼
[FASE 4: VERIFICAÇÃO E REFACTORING]
       ├── 1. Executar go test -v ./... (100% PASS independente de servidor externo)
       ├── 2. Auditoria automatizada de acessibilidade zerada (Zero violações WCAG AA)
       └── 3. Release v0.4 consolidada com selo de Acessibilidade & Robustez
```

### 4.1 Estrutura de Arquivos Proposta para a Fase 2 (TDD)
* `tests/chaos/chaos_test.go`: Atualização do helper de conexão para iniciar `httptest.NewServer` sob demanda quando a porta 8080 estiver livre.
* `web/web_a11y_test.go`: Nova suíte de testes em Go que faz parse dos templates HTML via `golang.org/x/net/html` e valida ausência de `maximum-scale=1`, presença de labels em inputs e botões com textos ou `aria-label`.
* `tests/e2e/playwright.config.ts`: Configuração otimizada do Playwright (headless, 1 worker para concorrência determinística, timeout de 10s).
* `tests/e2e/night_clock_chaos.spec.ts`: Teste E2E de corrida do relógio e esgotamento de mandados.
* `tests/e2e/accessibility_audit.spec.ts`: Auditoria automatizada de acessibilidade via `@axe-core/playwright`.

---

## 5. Conclusão da Fase 1
Todos os requisitos da Fase 1 foram satisfeitos com rigor técnico. Nenhum arquivo de produção foi alterado neste ciclo, assegurando integridade absoluta da base de código enquanto a equipe alinha o plano de remediação.

O relatório detalhado acima fornece à equipe de engenharia (Antigravity, Cipher, Tracker e Oracle) o roteiro cirúrgico para tornar o **Detetive IA** uma referência tanto em atmosfera artística quanto em padrões globais de engenharia de software e acessibilidade universal.
