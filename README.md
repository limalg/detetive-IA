# Detetive IA

Jogo investigativo interativo Web / PWA desenvolvido em Go com arquitetura monolítica leve, projetado para operar com eficiência e baixo consumo de memória (ambiente <= 1GB RAM).

## Arquitetura & Diretrizes
- **Linguagem**: Go (Golang)
- **Design**: Monolítico, binário único com assets Web/PWA embutidos (`embed.FS`)
- **Frontend**: Web responsiva com suporte a PWA (manifest, service worker) e tela de login simples
- **Infra/Recursos**: Máxima eficiência para rodar em VPS de 1GB de RAM
- **Gestão no Maestri**: Orquestrado em conjunto com agentes especializados (Orquestrador, Devs/QA/Git/Infra) e nota `Projeto` para tracking em ordem decrescente.
