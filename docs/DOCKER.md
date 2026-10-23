# Docker (Detiv + Banco 3D)

    cp .env.example .env      # opcional
    docker compose up -d --build
    # Detiv: http://localhost:8080/   Banco 3D: http://localhost:8080/banco/

- Imagem multi-stage (golang 1.26 → alpine, usuário não-root, web embutido no binário).
- Dados em `/app/data` (volume nomeado `detetive-data`): `detetive.db`, `banco3d/rooms.json`, `banco3d_rooms.json`, `banco3d_places.json`.
- Variáveis: `PORT`, `DB_PATH`, `BANCO3D_CACHE_FILE`, `HOST_PORT` (porta no host).
- Healthcheck: `/` e `/banco/`. `restart: unless-stopped`.
- Backup: `docker run --rm -v detetive-data:/d -v "$PWD":/b alpine tar czf /b/data.tgz -C /d .`
- Logs / parar: `docker compose logs -f` / `docker compose down` (o volume é mantido).
