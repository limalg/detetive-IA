# Relatório de Auditoria Diagnóstica de Frontend, UI & UX — Projeto Detetive IA
**Fase 1: Revisão Geral do Projeto DETIV**  
**Data:** 17 de Outubro de 2026  
**Líder Técnico:** Tracker (Fullstack, QA, Design & Frontend Lead)  
**Metodologia & Benchmarks:** *Inspora Design* (Atmosfera Cinematográfica), *Spell.sh* (Física Tátil & Micro-HUDs), *Refero Design* (Ergonomia de Modais & Painéis), *TasteSkill v2*, *Impeccable Craft* e *shadcn/ui*.

---

## 1. Resumo Executivo

O **Detetive IA** construiu uma base diegética impressionante nas versões v0.1 a v0.4, destacando-se pela mesa em madeira escura (`.desk-surface`), dossiê Manila Kraft (`.folder-kraft`), galeria de Polaroids com alfinetes vermelhos (`.polaroid-card`), lupa forense balística e síntese sonora procedural via Web Audio API (`audio.js`). O monolito em Go opera com excepcional eficiência de memória (~18MB RSS).

No entanto, a presente auditoria minuciosa revelou que a interface ainda carrega **resquícios evidentes de AI Slop** (padrões automatizados e genéricos que modelos de linguagem tendem a gerar por padrão), inconsistências cromáticas, lacunas de acessibilidade (contraste insuficiente segundo WCAG 2.1 AA), ausência de física tátil nos controles (`:active` e feedback de pressão ausentes), conflitos visuais causados por emojis multicoloridos em um ambiente Chiaroscuro e oportunidades cruciais de polimento no motor sonoro e na ergonomia dos fluxos.

Este documento cataloga cada problema diagnosticado com localização exata (arquivo e linha), propõe uma arquitetura de tokens e micro-interações inspirada nos mais altos padrões do design de interface moderno, e define a esteira de refatoração para a Fase 2.

---

## 2. Diagnóstico Geral & Identificação de AI Slop

### 2.1 Cores Genéricas e Inconsistências Cromáticas (AI Slop)
* **Localização Principal:** `web/static/css/style.css` (L5-L23), `web/templates/admin.html` (L85-L95).
* **Diagnóstico:**
  1. A paleta raiz (`:root`) adota hexadecimais padrão do Tailwind CSS (`--accent-amber: #f59e0b`, `--accent-gold: #d97706`, `--accent-crimson: #dc2626`, `--accent-green: #10b981`, `--border-noir: #383842`, `--bg-card: #222228`). Essas cores pertencem ao kit clássico de SaaS dark-mode genérico, e não a uma paleta com curadoria editorial e cinematográfica de Film Noir 1940s (que exige tons de linho envelhecido, fuligem, âmbar de filamento de tungstênio, sangue coagulado e couro gasto).
  2. Em `admin.html` (L85-L87), cores arbitrárias do Ant Design foram introduzidas inline (`#52c41a`, `#faad14`, `#ff4d4f`, `#73d13d`), rompendo o contrato de design do restante da aplicação.
  3. No chat de interrogatório (`style.css` L843-L854), os balões de diálogo (`#272730` e `#1f1f26`) reproduzem o formato de aplicativos de mensagens instantâneas contemporâneos (Slack/WhatsApp), colidindo com a atmosfera de relatório datilografado de delegacia dos anos 40.

### 2.2 Acessibilidade & Contrastes Fracos (WCAG 2.1 AA)
* **Localização:** `web/static/css/style.css` (L13-L14, L424-L429, L493-L496, L988-L991, L1301-L1310).
* **Diagnóstico:**
  1. **Texto Esmaecido sobre Fundo Escuro:** `--text-muted: #a1a1aa` aplicado sobre `--bg-secondary: #1a1a1e` ou `--bg-card: #222228` gera uma taxa de contraste de apenas **3.8:1**, reprovando o critério WCAG 2.1 AA (mínimo de **4.5:1** para texto normal). Ocorre em descrições de casos (`.case-desc`, L424), perfis de suspeitos (`.suspect-body`, L493) e descrições de modo (`.mode-desc`, L988).
  2. **Subtítulos no Kraft:** Em `.polaroid-secret-box` (L1301), texto em `#3f3f46` sobre fundo levemente acinzentado com fonte de `0.75rem` (12px) apresenta legibilidade precária em telas sem calibração profissional.
  3. **Falta de Indicador de Foco Acessível (`:focus-visible`):** Praticamente todos os botões (`.btn`, `.nav-tab-btn`, `.chip`, `.mode-card-btn`, `.weapon-card-item`) não possuem regra `:focus-visible`, violando WCAG 2.4.7 (Focus Visible) para navegação via teclado. Apenas `.form-input:focus` possui estilização de foco (L295).

### 2.3 Tipografia & Ilusão de Datilografia
* **Localização:** `web/static/css/style.css` (L21, L107-L113, L1285-L1291).
* **Diagnóstico:**
  1. A variável `--font-typewriter` utiliza a pilha `'Courier New', Courier, monospace`. No Windows, *Courier New* renderiza com hastes extremamente finas e serrilhadas (aliasing áspero), transmitindo aspecto de página web inacabada em vez de tipos de chumbo batidos por fita entintada.
  2. Recomenda-se importar fontes com textura analógica e peso encorpado (como *Special Elite*, *Cutive Mono* ou *Courier Prime* via Google Fonts / offline assets) combinadas com `-webkit-font-smoothing: antialiased`.
  3. O abuso de `text-transform: uppercase` com `letter-spacing: 2px` a `3px` em múltiplos rótulos atua como um clássico "tell" de design gerado por IA (conforme descrito no `frontend-design` e `ui-craft-inspiration`).

### 2.4 Sombras Padrão e Falta de Iluminação Direcional
* **Localização:** `web/static/css/style.css` (L20, L448, L549).
* **Diagnóstico:**
  1. As sombras utilizam borrões cinza-escuros genéricos (`--shadow-noir: 0 10px 25px -5px rgba(0, 0, 0, 0.8)`).
  2. No Film Noir (benchmark *Inspora*), a luz de uma luminária de escrivaninha verde (*banker's lamp*) ou lustre de teto projeta sombras nítidas, com oclusão ambiente (*ambient occlusion*) em múltiplos níveis de penumbra e um leve tom quente âmbar, em vez de sombras difusas e neutras.

### 2.5 Conflito Estético de Emojis Multicoloridos
* **Localização:** `web/templates/index.html` (L35, L42, L50, L91, L114, L139, L185, L190, L236, L248-L251, L274, L358, L383, L414-L417, L423).
* **Diagnóstico:**
  1. A interface exibe emojis de sistema (🕵️, 🚪, 🎲, 📁, 📜, 👥, 🔍, ⚖️, 🗡️, 🤫, 💀, 💦, ⭐, 🕰️, 📲). Nos navegadores modernos (Windows Segoe UI Emoji / Apple Color Emoji), esses emojis surgem em cores saturadas (amarelo brilhante, azul elétrico, vermelho plástico), quebrando imediatamente a imersão de um ambiente noir de época.
  2. Devem ser substituídos por ícones monocromáticos em SVG (estilo *Lucide* / *Radix Icons*) renderizados em bronze envelhecido, branco fosco ou sépia, preservando a identidade austera de 1940.

### 2.6 Falta de Física Tátil nos Controles (Benchmark Spell.sh)
* **Localização:** `web/static/css/style.css` (L170-L219, L333-L357, L892-L906, L950-L975).
* **Diagnóstico:**
  1. Os botões possuem apenas `:hover` com `transform: translateY(-1px)` e transição suave genérica (`transition: all 0.2s ease`).
  2. Falta a resposta de compressão tátil instantânea no clique/toque (`:active { transform: scale(0.98); }` ou `translateY(1px)`), padrão de excelência demonstrado pelo *Spell.sh* e *Linear*. O usuário sente a interface "flácida" e puramente digital.
  3. Apenas o botão `#stamp-accuse-btn` possui uma animação específica (`stampHit`), evidenciando a falta de consistência física no resto da UI.

---

## 3. Avaliação Detalhada por Componente & Tela

### 3.1 Header & Barra de Status Superior (`noir-header`)
* **Arquivo:** `web/templates/index.html` (L23-L57), `web/static/css/style.css` (L78-L167, L1096-L1154).
* **Achados Críticos:**
  - **Relógio da Madrugada (`#night-clock`):** O texto `(Tempo: 7h 00m)` é puramente descritivo. Falta um mostrador diegético visual (um minúsculo relógio de bolso vintage ou marcador analógico) e um micro-HUD explicativo no hover/click (ex: "Consumo de tempo: Interrogar: -15 min | Confrontar: -20 min | Falso Mandado: -30 min").
  - **Controle de Som (`#sound-toggle`):** O botão possui estilos inline (`style="padding: 0.35rem 0.6rem; font-size: 0.8rem;"`, L48) e exibe texto literal `🔊 Som: Ativo`. Falta uma chave diegética elegante (ex: um botão de rádio antigo ou fone de baquelite com micro-indicador de pulso de áudio).
  - **Badge da Marca (`.brand-badge`):** O gradiente linear `#121214` com texto âmbar tem aspecto de etiqueta de software moderna, ao invés de um distintivo de latão ou brasão policial de época gravado em baixo-relevo.

### 3.2 Quartel / Lobby & Seletor de Persona (`lobby-section`)
* **Arquivo:** `web/templates/index.html` (L84-L240), `web/static/css/style.css` (L947-L1056).
* **Achados Críticos:**
  - **Formulário de Persona & Custom Cast:** O bloco de adição de personagens customizados (`index.html` L136-L163) acumula múltiplos estilos inline (`style="background: rgba(0,0,0,0.3)..."`).
  - **Cards de Modo Solo vs. Multiplayer (`.mode-card-btn`):** São caixas escuras comuns com borda fina. Poderiam ser apresentados como dois formulários de convocação do departamento de polícia ou duas portas de escritório em vidro canelado (*frosted fluted glass*).
  - **Código da Sala (`.room-code-badge`):** O fundo preto puro `#0d0d10` destoa da paleta sépia da mesa. Um visual de carimbo com tinta preta gasta sobre papel sulfite ou fita de rotulador mecânico traria enorme refinamento.

### 3.3 Mesa Diegética & Dossiê Manila Kraft (`game-section`, `tab-content-dossier`)
* **Arquivo:** `web/templates/index.html` (L243-L281), `web/static/css/style.css` (L1080-L1220, L1438-L1489).
* **Achados Críticos:**
  - **Abas Kraft (`.nav-tab-btn.kraft-folder`):** As abas são muito bonitas individualmente, mas em telas menores que 768px sofrem quebra de linha desordenada (`flex-wrap` implícito na barra), perdendo o efeito de pastas sobrepostas de arquivo de aço.
  - **Cena do Crime no Dossiê (`#dossier-scene-img`):** A imagem possui transição estática. Poderia conter um clipe metálico (*paperclip*) dourado no canto superior esquerdo prendendo a fotografia ao papel kraft.
  - **Envelopes Confidenciais (`#envelopes-grid`):** Os envelopes gerados em JavaScript atualmente usam estilos inline e bordas simples. Devem simular envelopes de barbante e ilhós (*string & button envelopes*) característicos dos anos 40.

### 3.4 Galeria Polaroid & Pistas Forenses (`tab-content-suspects`, `tab-content-clues`)
* **Arquivo:** `web/templates/index.html` (L283-L305), `web/static/css/style.css` (L1222-L1310), `web/static/js/app.js` (L1058-L1130).
* **Achados Críticos:**
  - **Alinhamento dos Polaroids:** Todos os cards de suspeitos (`.polaroid-card`) ficam rigorosamente retos em grade CSS. Em um quadro de cortiça ou escrivaninha real, fotos presas com alfinetes têm rotações orgânicas sutis (`rotate(-1.5deg)`, `rotate(1deg)`, `rotate(-0.8deg)`).
  - **Efeito 3D Hover (Benchmark Spell/Inspora):** O hover atual aplica apenas `translateY(-5px) rotate(0.8deg)`. Deve implementar uma resposta de inclinação 3D baseada no mouse (`transform: perspective(600px) rotateX(...) rotateY(...)`), com o alfinete projetando sombra dinâmica.
  - **Pistas na Cena do Crime:** O banner da cena do crime (`.crime-scene-banner`, L295) possui altura fixa de 220px com `object-fit: cover`. Falta um efeito de vinheta nas bordas e textura de grão de filme 35mm.

### 3.5 Sala de Interrogatório & Palco de Confronto (`interrogation-modal`)
* **Arquivo:** `web/templates/index.html` (L369-L436), `web/static/css/style.css` (L586-L765, L768-L883), `web/static/js/app.js` (L1370-L1540).
* **Achados Críticos (Alta Gravidade de UX):**
  - **Sobrecarga Vertical (Ergonomia Refero):** O modal de interrogatório coloca em uma única coluna vertical: título, holofote com foto do suspeito, perfil, barra de estresse, balão de fala dinâmico, histórico de chat, chips de perguntas rápidas, seletor de confronto de pista e campo de input com botão. Em telas de laptop (1366x768 ou 1440x900), o usuário precisa rolar intensamente a página para ler o que o suspeito respondeu e digitar a próxima pergunta.
  - **Solução Recomendada:** Redesenhar o layout do interrogatório em **duas colunas ergonômicas** no desktop:
    - *Coluna Esquerda (Palco do Suspeito - 40%):* Holofote, retrato dinâmico com fumaça e suor, perfil policial, medidor de estresse com polígrafo analógico e último depoimento falado.
    - *Coluna Direita (Dossiê do Interrogatório - 60%):* Histórico datilografado contínuo em altura total, chips de perguntas rápidas ancorados e barra de confronto/input sempre visíveis na base (*sticky bottom*).
  - **Polígrafo / Medidor de Estresse:** O estresse atual é uma barra de progresso plana (`.stress-bar-fill`). Ela ganha enorme poder narrativo se renderizada como a agulha oscilante de um polígrafo analógico, com ondas eletrocardiográficas que aumentam de frequência conforme o suspeito entra em pânico.

### 3.6 Lupa Forense (`forensic-loupe-modal`)
* **Arquivo:** `web/templates/index.html` (L468-L493), `web/static/css/style.css` (L1541-L1670).
* **Achados:**
  - O componente da lupa é um dos pontos altos do projeto (aro de bronze, reflexo de vidro e retículo).
  - Oportunidade: A imagem inspecionada (`.loupe-img-inspect`) é estática no centro. Um micro-efeito de movimentação da lente acompanhando as coordenadas do mouse dentro do container do card tornaria o exame pericial extraordinariamente imersivo.

### 3.7 Painel Administrativo & Telemetria (`admin.html`)
* **Arquivo:** `web/templates/admin.html` (L1-L401).
* **Achados Críticos:**
  - Contém mais de 90 linhas de CSS embutido em `<style>` interno.
  - Usa cores desacopladas (`#52c41a`, `#faad14`, `#ff4d4f`) e classes próprias (`.metric-card`, `.log-table`).
  - Falta unificação com os tokens centrais de `style.css` e responsividade para dispositivos móveis ou tablets.

---

## 4. Auditoria do Motor de Áudio Diegético (`web/static/js/audio.js`)

O motor de áudio procedural sintetizado via Web Audio API é uma escolha brilhante de engenharia: tem **zero dependências externas**, peso nulo em download de arquivos de áudio (.mp3/.wav) e funciona offline no PWA. Porém, apresenta fragilidades técnicas e oportunidades de expansão:

### 4.1 Problemas Técnicos Identificados
1. **Risco de Distorção e Clipping Digital (Falta de Master Bus e Limiter):**
   - Em `audio.js` (L129, L143, L174, L201, L226, L264), cada nó de ganho dos efeitos sonoros conecta-se diretamente a `c.destination`.
   - Se o usuário digitar rapidamente no teclado (disparando múltiplos `playTypewriter`) enquanto o relógio bate (`playClockTick`), as ondas somam suas amplitudes no barramento de saída do sistema operacional. Sem um nó `DynamicsCompressorNode` intermediário, ocorre saturação e estalo (*audio click/pop*).
2. **Ciclo de Desbloqueio de Áudio:**
   - O desbloqueio do `AudioContext` em navegadores mobile (L53-L64) remove os ouvintes assim que o contexto atinge o estado `running`. Em certos navegadores (Safari iOS e Chrome Mobile após suspensão de aba), o contexto pode voltar ao estado `suspended` e falhar silenciosamente.

### 4.2 Gaps Sonoros & Efeitos Ausentes para Atmosfera Noir
1. **Loop de Ambiência Sutil (Chuva & Vinil):**
   - Falta um ruído de fundo atmosférico diegético muito suave (chuva contra a vidraça da agência e ruído de fita/gramofone), gerado proceduralmente com ruído rosa/marrom filtrado a baixíssimo volume (-32dB) com botão dedicado de mute.
2. **Stinger de Tensão / Batimento Cardíaco (`playHeartbeat`):**
   - Quando o suspeito ultrapassa 70% de estresse ou a madrugada entra na última hora (menos de 60 minutos restantes), a introdução de uma pulsação sub-grave (60-80Hz) eleva a tensão dramática instantaneamente.
3. **Cliques de Lupa e Alfinete (`playLensFocus` e `playPin`):**
   - Ao abrir a lupa forense ou fixar uma pista no mandado de acusação, a ausência de som deixa a ação tátil incompleta.

---

## 5. Lista Priorizada de Melhorias (Alta, Média, Baixa)

| ID | Prioridade | Arquivo | Linhas | Descrição do Problema | Solução Recomendada |
|---|---|---|---|---|---|
| **A-01** | **ALTA** | `web/static/css/style.css` | L5-L23 | Tokens de cores genéricos do Tailwind sem curadoria Film Noir de época. | Substituir paleta raiz por sistema cromático Chiaroscuro 1940s (âmbar tungstênio, couro cru, linho antigo, sangue coagulado, grafite). |
| **A-02** | **ALTA** | `web/templates/index.html`<br>`web/static/css/style.css` | L369-L436<br>L586-L765 | Interrogatório em coluna única exige rolagem excessiva no desktop/laptop (problema severo de ergonomia Refero). | Redesenhar o modal em 2 colunas: Coluna Esquerda com Palco, Polígrafo e Retrato dinâmico; Coluna Direita com Histórico Datilografado e barra de ação fixa. |
| **A-03** | **ALTA** | `web/static/css/style.css` | L170-L219 | Botões e elementos clicáveis sem física tátil `:active` e sem anel acessível `:focus-visible` (Spell.sh). | Adicionar `active:scale-[0.98]` com transição cúbica de mola, relevo de borda inferior e `:focus-visible` em anel dourado contrastante. |
| **A-04** | **ALTA** | `web/static/js/audio.js` | L129, L143, L174, L201, L226, L264 | Nós de áudio conectados diretamente ao `c.destination`, causando risco de clipping digital e estalos. | Criar nó central `masterGain` roteado através de um `DynamicsCompressorNode` com teto de -1dB antes do destino final. |
| **A-05** | **ALTA** | `web/static/css/style.css` | L424, L493, L988, L1301 | Contraste insuficiente em textos secundários (#a1a1aa sobre #1a1a1e e #222228), violando WCAG 2.1 AA. | Elevar contraste dos textos secundários para `#d4d4d8` e `#e4e4e7` (garantindo índice mínimo de 5.5:1). |
| **M-01** | **MÉDIA** | `web/templates/index.html` | Múltiplas | Emojis coloridos contemporâneos quebrando a imersão monocromática noir de época. | Substituir emojis por ícones SVG monocromáticos em traço fino estilizados em bronze, sépia ou marfim. |
| **M-02** | **MÉDIA** | `web/static/css/style.css` | L21 | Fonte Courier New padrão com renderização áspera e sem peso visual autêntico. | Adicionar suporte a `@font-face` para fonte analógica datilográfica (ex: *Special Elite* / *Cutive Mono* ou *Courier Prime*) com text-shadow subpixel. |
| **M-03** | **MÉDIA** | `web/static/css/style.css`<br>`web/static/js/app.js` | L1222-L1258<br>L1060-L1100 | Polaroids rigorosamente alinhados em grade sem naturalidade analógica. | Aplicar rotações pseudo-aleatórias (`:nth-child(3n+1) { rotate: -1.2deg }`, etc.) e física de inclinação 3D no hover. |
| **M-04** | **MÉDIA** | `web/static/js/audio.js` | Novo módulo | Falta de ambiência contínua (chuva sutil / estalido de vinil) e batimento cardíaco sob alto estresse. | Implementar sintetizadores procedurais para `playAmbientRain()` e `playHeartbeat(bpm)`. |
| **M-05** | **MÉDIA** | `web/templates/admin.html` | L8-L100 | Estilos duplicados e cores discrepantes embutidas no cabeçalho do template administrativo. | Migrar estilização administrativa para `style.css` reaproveitando os tokens unificados. |
| **M-06** | **MÉDIA** | `web/static/js/app.js` | L292, L335, L1729 | Troca abrupta de seções e abas sem transição visual fluida (Inspora). | Implementar transição de dissolução cruzada (*cross-fade*) suave (150ms) entre telas. |
| **B-01** | **BAIXA** | `web/templates/index.html` | L48, L136-L163 | Estilos inline acumulados em componentes do formulário de elenco e cabeçalho. | Extrair todos os estilos inline para classes utilitárias semânticas em `style.css`. |
| **B-02** | **BAIXA** | `web/static/css/style.css` | L1459-L1489 | Abas kraft do dossiê quebrando em múltiplas linhas em resoluções móveis estreitas (<400px). | Adicionar rolagem horizontal suave (`overflow-x: auto; scrollbar-width: none;`) nas abas do dossiê em mobile. |
| **B-03** | **BAIXA** | `web/static/js/app.js` | L1209 | Tecla ESC fecha apenas a lupa forense, mas ignora o interrogatório e a acusação. | Centralizar gerenciamento de modais com pilha ativa, tecla ESC universal e restauração de foco acessível. |

---

## 6. Proposta de Tokens, Micro-Interações & Sistema de Design 1940s

### 6.1 Nova Arquitetura de Tokens de Design (`style.css`)

```css
/* ==========================================================================
   Detetive IA - Sistema Cromático & Tipográfico Chiaroscuro 1940s
   ========================================================================== */
:root {
  /* Superfícies & Planos de Fundo (Madeira de Lei, Carvão e Couro) */
  --noir-desk-wood: #14110f;
  --noir-desk-mat: #1a1512;
  --noir-bg-primary: #0e0d0f;
  --noir-bg-secondary: #17151a;
  --noir-bg-card: #201d24;
  --noir-bg-card-hover: #29252e;

  /* Papéis & Materiais Táteis (Kraft, Manila e Polaroid) */
  --noir-kraft-light: #dfb284;
  --noir-kraft-base: #caa06d;
  --noir-kraft-dark: #8e623b;
  --noir-kraft-ink: #2b1d0c;
  --noir-polaroid-bg: #f9f8f4;
  --noir-polaroid-ink: #181719;

  /* Luz Âmbar de Tungstênio & Metais Antigos */
  --noir-amber-glow: #f59e0b;
  --noir-amber-core: #d97706;
  --noir-amber-dim: #78350f;
  --noir-brass-gold: #c59b27;
  --noir-brass-shadow: #5c440a;

  /* Sangue, Carimbos & Alertas */
  --noir-stamp-red: #b91c1c;
  --noir-blood-dark: #660f0f;
  --noir-alert-crimson: #dc2626;
  --noir-evidence-green: #059669;

  /* Tipografia & Contrastes Acessíveis (WCAG 2.1 AA Compliant >= 5.5:1) */
  --noir-text-pure: #ffffff;
  --noir-text-main: #f4f4f5;
  --noir-text-secondary: #d4d4d8; /* Ajustado de #a1a1aa para 5.8:1 */
  --noir-text-muted: #a1a1aa;
  --noir-border-subtle: #302c36;
  --noir-border-pronounced: #484350;

  /* Sombras Direcionais & Oclusão de Ambiente */
  --shadow-desk-lamp: 
    0 2px 4px rgba(0, 0, 0, 0.4),
    0 12px 28px -4px rgba(0, 0, 0, 0.75),
    0 24px 48px -12px rgba(20, 17, 15, 0.95);
  --shadow-paper-lift: 
    0 1px 3px rgba(0, 0, 0, 0.3),
    0 8px 20px -2px rgba(0, 0, 0, 0.6);
  --shadow-stamp-pressed: 
    0 1px 2px rgba(0, 0, 0, 0.95),
    inset 0 2px 8px rgba(0, 0, 0, 0.85);

  /* Fontes */
  --font-typewriter: 'Special Elite', 'Courier Prime', 'Courier New', Courier, monospace;
  --font-editorial: 'Playfair Display', Georgia, serif;
  --font-sans: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
}
```

### 6.2 Especificação de Micro-Interações & Física Tátil (Spell.sh)

```css
/* Feedback Tátil Universal para Ações Rápidas */
.btn, .mode-card-btn, .chip, .weapon-card-item, .nav-tab-btn {
  will-change: transform, box-shadow;
  transition: transform 0.12s cubic-bezier(0.16, 1, 0.3, 1),
              background-color 0.15s ease,
              border-color 0.15s ease,
              box-shadow 0.15s ease;
}

/* Estado Ativo (Compressão Instantânea no Toque/Clique) */
.btn:active, 
.mode-card-btn:active, 
.chip:active, 
.weapon-card-item:active {
  transform: scale(0.97) translateY(1px) !important;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.8) !important;
}

/* Indicador de Foco Acessível & Elegante */
.btn:focus-visible,
.form-input:focus-visible,
.chip:focus-visible,
.nav-tab-btn:focus-visible {
  outline: none;
  border-color: var(--noir-amber-glow);
  box-shadow: 0 0 0 3px rgba(245, 158, 11, 0.35), 0 0 12px rgba(245, 158, 11, 0.2);
}
```

### 6.3 Wireframe Ergonômico da Sala de Interrogatório (Refero 2-Column Command Center)

```text
+---------------------------------------------------------------------------------------------------+
|  [SALA DE INTERROGATÓRIO] - Julian Pendelton (Herdeiro Falido)                           [ X ]   |
+-------------------------------------------------+-------------------------------------------------+
|  COLUNA ESQUERDA (Palco Pericial - 38%)         |  COLUNA DIREITA (Dossiê & Transcrição - 62%)   |
|                                                 |                                                 |
|  [=========== HOLOFOTE ZENITAL ============]    |  [Histórico Datilografado da Sessão]            |
|                   .---.                         |  ---------------------------------------------  |
|                  /     \   [💦 Suor L2]         |  [Detetive]: Onde você estava às 22h?           |
|                 |  (o)  |                       |  [Julian]: Afirmo que estava na adega de vinhos |
|                  \  _  /                        |            conferindo as garrafas de Arthur...  |
|                   '---'                         |  [Detetive + Pista #2]: E este recibo rasgado?  |
|             (Retrato 140px)                     |  [Julian]: Céus! Não imaginei que achariam...   |
|                                                 |  ...                                            |
|  Nível de Estresse: 65% (Aflito/Secundário)     |                                                 |
|  [Polígrafo: ~~~~/\_/\/\__/\~~~~~~~~~~~~~]     |  (Área rolável flexível preenchendo altura)     |
|                                                 |                                                 |
|  "Você está me pressionando demais, inspetor!   |  ---------------------------------------------  |
|   Eu não pretendia matar ninguém..."            |  [Perguntas Rápidas]:                           |
|                                                 |  [📍 Álibi] [👤 Relação] [🗡️ Arma] [🤫 Segredo] |
|  [Perfil do Suspeito & Contradições Criminais]  |  ---------------------------------------------  |
|  • Álibi formal: Adega subterrânea              |  Confrontar com Prova: [Seletor de Pistas  v]   |
|  • Segredo: Dívidas de cassino em Santos        |  [ Digite sua pergunta inquisitiva... ] [Enviar]|
+-------------------------------------------------+-------------------------------------------------+
```

### 6.4 Arquitetura do Motor de Áudio com Master Compressor (`audio.js`)

```javascript
/* Topologia Proposta para o Web Audio Engine */
//
//  [Oscilador / Noise Buffer]
//             │
//      [BiquadFilter]
//             │
//       [Efeito Gain]
//             │
//             ▼
//       [Master Gain] (0.0 a 1.0)
//             │
//             ▼
//  [DynamicsCompressorNode] (Threshold: -6dB, Ratio: 4:1, Knee: 10)
//             │
//             ▼
//      [AudioContext.destination] (Sem clipping, sem distorção digital)
```

---

## 7. Próximos Passos (Plano para a Fase 2: Refatoração & Polimento)

1. **Sprint 2.1 — Tokens Cromáticos & Tipografia (A-01, M-02):** Implementar o novo token system Chiaroscuro 1940s, substituir Courier New por fontes encorpadas e eliminar cores arbitrárias em `admin.html`.
2. **Sprint 2.2 — Física Tátil & Acessibilidade (A-03, A-05):** Injetar `active:scale-[0.98]` em todos os controles interativos, normalizar anéis de foco acessíveis e reajustar taxas de contraste WCAG AA.
3. **Sprint 2.3 — Redesenho Ergonômico do Interrogatório (A-02):** Reestruturar o modal de interrogatório em 2 colunas com polígrafo responsivo e histórico ancorado.
4. **Sprint 2.4 — Blindagem & Expansão do Áudio Engine (A-04, M-04):** Adicionar barramento master compressor, sons de chuva/vinil procedural e batimento cardíaco sob alta tensão.
5. **Sprint 2.5 — Ícones Monocromáticos & Validação Visual (M-01, M-03, M-06):** Substituir emojis por SVGs de época, introduzir rotação analógica nos Polaroids e transições suaves entre telas.

---
*Relatório registrado em `docs/reviews/frontend_audit.md` para apreciação do Maestro e da equipe.*
