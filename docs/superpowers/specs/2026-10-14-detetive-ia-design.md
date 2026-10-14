# Especificação de Design: Detetive IA

## 1. Visão Geral
**Detetive IA** é um jogo investigativo interativo web com suporte a Progressive Web App (PWA), construído como um monolito leve em Go (Golang) com SQLite embutido e frontend servido diretamente através de `embed.FS`. O sistema é dimensionado para rodar com alta performance e baixo footprint em servidores com 1GB de RAM.

## 2. Metas e Restrições
- **Memória**: Consumo máximo alvo em execução < 60MB RAM (deixando > 900MB livres no servidor).
- **Arquitetura**: Monolítica, binário único em Go sem dependência externa de banco de dados ou CGO (`modernc.org/sqlite`).
- **PWA**: Instalável no celular e desktop, com `manifest.json`, ícones e `service-worker.js` para cache offline de assets estáticos.
- **Autenticação**: Tela de login simples com credenciais salvas com hash seguro (bcrypt) e sessão com cookies `HttpOnly`.
- **Inteligência Artificial**: Suspeitos interrogáveis dinamicamente via LLM (Gemini API com credencial configurável via Maestri / `.env`), além de modo offline/fallback roteirizado.

## 3. Arquitetura do Sistema

```
+---------------------------------------------------------+
|                  Navegador Web / PWA                    |
|  - UI Tema Noir (HTML5 + Tailwind CSS + Vanilla JS)     |
|  - Service Worker (Cache Offline)                       |
+----------------------------+----------------------------+
                             | HTTP/JSON (REST)
+----------------------------v----------------------------+
|             Monolito Go (detetive-ia binary)            |
|                                                         |
|  +---------------------+       +---------------------+  |
|  |   HTTP Router       |       |  embed.FS (Web UI)  |  |
|  +----------+----------+       +---------------------+  |
|             |                                           |
|  +----------v----------+       +---------------------+  |
|  | Serviços de Domínio |<----->| Interrogatório LLM  |  |
|  | - Auth / Sessão     |       | (Gemini / Fallback) |  |
|  | - Caso & Pistas     |       +---------------------+  |
|  | - Acusação / Veredito                                |
|  +----------+----------+                                |
|             |                                           |
|  +----------v----------+                                |
|  | SQLite (modernc)    |                                |
|  +---------------------+                                |
+---------------------------------------------------------+
```

## 4. Modelagem de Dados (SQLite)

- `users`: `id`, `username`, `password_hash`, `created_at`
- `sessions`: `id`, `user_id`, `token`, `expires_at`
- `cases`: `id`, `title`, `description`, `victim`, `solution_suspect_id`, `solution_weapon`, `solution_motive`
- `suspects`: `id`, `case_id`, `name`, `role`, `personality`, `alibi`, `secret`, `is_culprit`
- `clues`: `id`, `case_id`, `location`, `name`, `description`
- `interrogation_messages`: `id`, `session_id`, `suspect_id`, `sender` (detective/suspect), `message`, `timestamp`
- `investigations`: `id`, `user_id`, `case_id`, `status` (in_progress, solved, failed), `accused_suspect_id`, `accused_weapon`, `accused_motive`, `completed_at`

## 5. Endpoints da API

- `POST /api/auth/register` - Cadastro de usuário simples
- `POST /api/auth/login` - Login e criação de cookie de sessão
- `POST /api/auth/logout` - Encerramento de sessão
- `GET /api/auth/me` - Validação do usuário ativo
- `GET /api/game/case` - Dados do caso atual (vítima, locais, suspeitos, pistas descobertas)
- `POST /api/game/interrogate` - Envio de pergunta ao suspeito; retorna resposta gerada por IA
- `POST /api/game/accuse` - Submissão da dedução final do jogador (Suspeito, Arma, Motivo) e veredito
- `GET /api/health` - Healthcheck de integridade e métricas de memória (`runtime.MemStats`)

## 6. Frontend & Experiência do Usuário (PWA)
- **Tema Visual**: Estilo Noir cinematográfico (fundo escuro #121214, tons de cinza, acentos âmbar/dourado e iluminação dramática).
- **Telas**:
  1. *Login / Registro*: Formulário limpo, sem atrito.
  2. *Quartel / Dossiê do Caso*: Resumo do crime, relatório da autópsia e lista de suspeitos.
  3. *Cena do Crime / Locais*: Exploração de pistas com cards táteis.
  4. *Sala de Interrogatório*: Interface estilo chat dinâmico com avatar do suspeito, indicando reações e respostas em tempo real.
  5. *Quadro de Deduções*: Conexão de pistas para emitir o veredito final.
- **PWA**: Ícone da insígnia de detetive, suporte a tela cheia standalone e funcionamento suave no mobile.

## 7. QA, Testes e Monitoramento
- **Testes Unitários**: Cobertura de autenticação, hashing de senhas, validação de regras de dedução e fallback de diálogos.
- **Testes de Integração**: Testes HTTP simulando o fluxo completo de investigação.
- **Validação de Performance**: Endpoint `/api/health` expondo `Alloc` e `Sys` de memória para garantir que o monolito opere abaixo de 50MB.

## 8. Orquestração e Equipe no Maestri
- **Antigravity (Orquestrador)**: Gestão de metas, backlog decrescente na nota `Projeto`, code review e alinhamento com usuário.
- **Perito (Fullstack & Backend / Infra)**: Implementação do core Go, SQLite, endpoints, testes de performance e integração com LLM.
- **Lince (Fullstack & Frontend / PWA)**: Implementação da interface web, estilização Noir, service worker, manifest PWA e testes de interface.
- **Nota `Projeto`**: Registro de cada fix em ordem decrescente de execução.
