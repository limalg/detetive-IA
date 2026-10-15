# Design Document: Mesa Tátil Noir, Retratos Cartunescos Sérios, Trilha Diegética e Múltiplos Temas

**Data:** 2026-10-14  
**Versão:** v0.4-design  
**Status:** Revisado após Builder Check & Aprovado para Planejamento  
**Autor:** Antigravity (com Oracle, Raven, Buster, Cipher, Tracker)

---

## 1. Visão Geral & Intenção (The Why)

O usuário avaliou a versão v0.3 e definiu a visão para o jogo:
1. **Mesa Tátil Diegética:** Uma escrivaninha de mogno com atmosfera tátil (estilo *Papers, Please* e *Strange Horticulture*), com documentos em papel Kraft manipuláveis, carimbos com tinta vermelha, relógio de bolso analógico funcional e lupa de inspeção pericial.
2. **Direção de Arte dos Personagens:** Estilo **cartunesco sério** inspirado em *Graphic Novels Noir* (*Blacksad*, *The Wolf Among Us*, *Dick Tracy*), mantendo a sobriedade dramática e o suspense sem caricaturas cômicas.
3. **Sala de Interrogatório Centralizada:** O retrato do suspeito fica **perfeitamente centralizado** sob o foco de luz da luminária, acima da caixa de diálogo, com micro-animações expressivas em camadas CSS/SVG (fumaça de cigarro, respiração tensa, gotas de suor sob estresse elevado).
4. **Múltiplos Temas de Crime & Densidade de Suspeitos:** 4 temas procedurais ricos com narrativas próprias e um seletor para escolher de 3 a 6 suspeitos por partida.
5. **Correção de Bloqueios Críticos:**
   - Correção imediata do reinício de partida que travava o jogo afirmando que os mandados acabaram.
   - Mapeamento visual e suporte completo para todas as 5 armas do crime (Adaga, Cálice de Cianeto, Revólver Silenciado, Castiçal de Bronze e Navalha Pericial).
6. **Imersão Sonora Diegética:** Trilha sonora e efeitos sintetizados via Web Audio API nativo (tique-taque de relógio, carimbada seca com tinta, farfalhar de papel, máquina de escrever e chuva na janela).

---

## 2. Metas, Não-Metas e Anti-Metas

### 2.1 Metas (Goals)
- **Mesa Tátil Responsiva (PWA):** Escrivaninha de madeira, pastas Manila Kraft com abas táteis, carimbo interativo para emissão do mandado, relógio analógico de bolso funcional e lupa para inspeção de pistas.
- **Interrogatório Centralizado & Expressivo:** Foco visual centralizado no suspeito, com micro-animações CSS/SVG leves (fumaça, tremor de estresse e suor em estresse > 70%).
- **Multi-Temas Procedurais:** Motor `Weaver` com 4 temas ricos (Mansão, Expresso da Meia-Noite, Ópera Municipal, Cassino Clandestino).
- **Seletor de Suspeitos:** Configuração de 3 a 6 suspeitos por partida (solo ou sala cooperativa).
- **Correção da Economia de Tempo & Mandados:** Garantir que cada nova partida (solo ou sala) gere um caso e investigação virgens com 420 minutos e 2 mandados intactos.
- **Sonoplastia Diegética Zero-Dependency:** Módulo de áudio em Web Audio API nativo do navegador, sem dependências externas nem consumo excessivo de rede.
- **Memória & Performance:** Manter a pegada monolítica em Go estritamente abaixo do teto de 60 MB de RAM.

### 2.2 Não-Metas (Non-Goals)
- Motor 3D WebGL pesado (a ilusão tátil será construída com CSS3, SVG, sombras dinâmicas e transições fluidas, garantindo fluidez em celulares).
- Dublagem completa por voz (manteremos o diálogo textual estilizado como máquina de escrever com áudio sintetizado).

### 2.3 Anti-Metas (Anti-Goals)
- Não transformar o jogo em desenho animado infantil ou pastelão; o tom deve ser maduro, tenso e investigativo.
- Não introduzir dependências externas de CGO ou servidores de banco de dados pesados (permanecer 100% puro Go com modernc.org/sqlite).

---

## 3. Decisões Arquiteturais e Resolução do Builder Check

### 3.1 Arquitetura de Salas & Estado de Investigação Compartilhado *(Decisão do Construtor)*
- **Modo Solo:** Ao clicar em "Iniciar Investigação Solo", o frontend chama `POST /api/game/solo/start` com `{ theme_id, suspect_count }`. O backend aciona o `Weaver`, gera o caso procedural, cria a investigação com 420 minutos e 2 mandados e retorna os dados completos do novo caso.
- **Salas Cooperativas:** Todos os jogadores humanos conectados à sala operam sob o mesmo caso e compartilham o mesmo Relógio da Madrugada (420 min) e o limite de 2 mandados de prisão como um esquadrão cooperativo de detetives.
- **Papéis e Suspeitos:** Os jogadores definem suas personas na entrada. Se o número de jogadores for menor que `suspect_count`, o `Weaver` preenche as vagas restantes com bots noir arquetípicos do tema escolhido.

### 3.2 Pipeline de Retratos & Micro-Animações Expressivas *(Decisão do Construtor)*
- **Componente de Retrato em Camadas (`.suspect-portrait-frame`):**
  - **Camada Base:** Imagem estilizada em traço cartunesco noir / graphic novel (formato WebP otimizado).
  - **Camada Atmosférica:** Cone de luz da luminária com iluminação radial suave.
  - **Camada de Partículas CSS:** Fumaça de cigarro animada com SVG/CSS (`@keyframes smokeDrift`).
  - **Camada de Reação a Estresse:** Gotas de suor discretas em SVG ativadas quando o estresse ultrapassar 70%, acompanhadas de leve tremor corporal (`@keyframes nervousBreath`).
  - Esse modelo não requer engines pesadas de animação nem spritesheets gigantes, funcionando perfeitamente em mobile e desktop.

### 3.3 Paradigma da Mesa Tátil & Lupa de Perícia *(Decisão do Construtor)*
- **Paradigma de Interface:** Interface tátil organizada em pastas Manila e documentos de papel Kraft dispostos na escrivaninha de mogno, garantindo 100% de usabilidade em celulares (PWA responsivo) sem os problemas de sobreposição caótica de janelas flutuantes.
- **Lupa de Perícia:** Ferramenta interativa que ativa um modal de inspeção macro com zoom óptico nos detalhes das pistas e armas do crime (revelando número de série do revólver, resíduo químico no cálice, fibras de veludo na lâmina).

### 3.4 Motor de Temas Procedurais (`internal/game/weaver.go`)
- **Quatro Temas Implementados:**
  1. **Mansão do Vale das Sombras:** Crime na alta sociedade (vítima: Sir Arthur Pendelton; motivos: herança, testamento adulterado).
  2. **Expresso da Meia-Noite:** Diplomata Viktor Orlov executado no vagão restaurante; contrabando e espionagem nos trilhos.
  3. **A Última Ária na Ópera:** Maestro Gabriel Fontaine envenenado antes do 3º ato; rivalidade artística e cartas de chantagem.
  4. **A Dívida do Cassino Clandestino:** Dono de bancas de jogo Emilio Ramos morto no cofre; agiotagem e queima de arquivo.
- **Pool de 5 Armas Mapeadas:**
  - `Adaga Cerimonial Veneziana` (ilustração dedicada)
  - `Cálice com Resíduo de Cianeto` (ilustração dedicada)
  - `Revólver .38 com Silenciador` (ilustração pericial dedicada)
  - `Castiçal de Bronze Maciço` (ilustração dedicada)
  - `Navalha Pericial de Aço` (ilustração dedicada)

### 3.5 Correção Definitiva do Bug de Mandados e Reinício
- No banco SQLite, a busca de investigações em andamento passa a usar:
  ```sql
  SELECT id, user_id, case_id, status, time_remaining_minutes, warrants_remaining 
  FROM investigations 
  WHERE user_id = ? AND case_id = ? AND status = 'in_progress' 
  ORDER BY id DESC LIMIT 1
  ```
- O botão "Reiniciar Partida" do Game Over e a inicialização de novas partidas chamam a rota de criação de novo caso procedural, garantindo que `status` comece sempre como `'in_progress'` com `time_remaining_minutes = 420` e `warrants_remaining = 2`.

---

## 4. O que Fica a Critério dos Construtores
- O design dos seletores de volume de áudio e toggle mudo.
- O mapeamento exato de micro-sons na Web Audio API (frequências e decays das notas de máquina de escrever e tique-taque).
