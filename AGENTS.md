# AGENTS.md — Diretrizes da Equipe & Vault de 50 Skills (Detiv / Detetive IA)

## 🎭 Arquitetura da Equipe Maestri

O projeto **Detetive IA** é liderado pelo agente **Antigravity** (Maestro) e operado por 5 agentes especialistas conectados no Maestri Canvas:

| Agente | Papel (Role) | Frente de Atuação | Skills Chave Atribuídas |
| :--- | :--- | :--- | :--- |
| **Antigravity** | `Maestro / Lead Agent` | Orquestração, Memória & Mediação | `task-observer`, `claude-mem`, `claude-setup` |
| **Tracker** | `Fullstack-QA-Infra` (Frontend Lead) | Interface Noir, Mesa Tátil, Áudio & PWA | `frontend-design`, `ui-craft-inspiration`, `impeccable-craft`, `taste-skill`, `shadcn-ui-master`, `magic-mcp-21st` |
| **Cipher** | `Fullstack-QA-Infra` (Backend Lead) | Engine Go, SQLite, APIs, Segurança & Infra | `addy-security-and-hardening`, `addy-performance-optimization`, `chrome-devtools-mcp`, `addy-code-review-and-quality`, `addy-code-simplification`, `addy-debugging-and-error-recovery`, `addy-api-and-interface-design`, `addy-observability-and-instrumentation` |
| **Buster** | `Chaos-QA-Tester` | QA E2E, Resiliência, Chaos & Acessibilidade | `playwright-cli`, `web-design-guidelines`, `addy-test-driven-development` |
| **Raven** | `Narrative-Noir-Writer` | Procedural Mystery, Teia Social & Prompts IA | `claude-mem`, `source-driven-development` |
| **Oracle** | `Game-Researcher` | Game Design, Balanceamento & Ergonomia Lúdica | `addy-spec-driven-development`, `doubt-driven-development` |

---

## 🏛️ Localização do Vault Master de Skills

As 50 Skills profissionais encontram-se instaladas e indexadas em:
1. `~/.gemini/config/skills/`
2. `C:\Users\limae\Documents\Projects\Agent-Skills-Vault\skills\`

---

## 🧭 Fases da Revisão Geral do Projeto (v0.6)

1. **Fase 1: Auditoria Diagnóstica Paralela**
   - Cada agente inspeciona sua respectiva frente no código e gera um documento estruturado em `docs/reviews/`:
     - `docs/reviews/frontend_audit.md` (Tracker)
     - `docs/reviews/backend_security_perf_audit.md` (Cipher)
     - `docs/reviews/qa_accessibility_audit.md` (Buster)
     - `docs/reviews/narrative_noir_audit.md` (Raven)
     - `docs/reviews/game_design_audit.md` (Oracle)
   - Agentes notificam o Maestro via `maestri ask "Antigravity" "<resumo>"`.

2. **Fase 2: Consolidação do Plano Unificado**
   - O Maestro compila todos os diagnósticos em um plano priorizado e alinha com o usuário.

3. **Fase 3: Execução Sincronizada**
   - Aplicação de código, testes TDD e verificação contínua (`go test ./...` e smoke test).

4. **Fase 4: Encerramento, Tag & Memória**
   - Atualização da nota `Projeto`, documentação de lições aprendidas e publicação da release.

---

## ⚡ Gatilho `meta-skills` (Verificação & Priorização)

Ao receber o termo **`meta-skills`** ou ao iniciar novos módulos/features:
1. **Prioridade em Perguntas & Descoberta**: Ative imediatamente [`interview-me`](file:///C:/Users/limae/.gemini/config/skills/addy-interview-me/SKILL.md) e [`idea-refine`](file:///C:/Users/limae/.gemini/config/skills/addy-idea-refine/SKILL.md) para alinhar escopo e premissas antes de qualquer implementação.
2. **Validação de Restrições**: Utilize [`constraint-driven-development`](file:///C:/Users/limae/.gemini/config/skills/addy-constraint-driven-development/SKILL.md) e [`spec-driven-development`](file:///C:/Users/limae/.gemini/config/skills/addy-spec-driven-development/SKILL.md) antes de gerar tarefas de código.
3. **Distribuição aos Agentes**: O Maestro distribui as tarefas refinadas aos agentes especialistas ([Tracker](file:///C:/Users/limae/Documents/Projects/Detiv/AGENTS.md#L10), [Cipher](file:///C:/Users/limae/Documents/Projects/Detiv/AGENTS.md#L11), [Buster](file:///C:/Users/limae/Documents/Projects/Detiv/AGENTS.md#L12), [Raven](file:///C:/Users/limae/Documents/Projects/Detiv/AGENTS.md#L13), [Oracle](file:///C:/Users/limae/Documents/Projects/Detiv/AGENTS.md#L14)) munidos de suas respectivas skills.
