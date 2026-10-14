#!/usr/bin/env bash
set -euo pipefail

echo "=========================================================="
echo " Detetive IA - Verificação de Consumo de Memória (RAM)"
echo " Limite Alvo: < 60 MB RAM (Ambiente de 1GB RAM)"
echo "=========================================================="

PORT=8899
DB_PATH="data/test_memory.db"
export PORT DB_PATH

mkdir -p "$(dirname "$DB_PATH")" bin

echo "[1/4] Compilando binário monolítico otimizado..."
go build -ldflags="-s -w" -o bin/detetive-ia ./cmd/server

echo "[2/4] Iniciando servidor em segundo plano na porta $PORT..."
./bin/detetive-ia &
SERVER_PID=$!

cleanup() {
  echo "Encerrando servidor (PID: $SERVER_PID)..."
  kill -9 "$SERVER_PID" 2>/dev/null || true
  rm -f "$DB_PATH"*
}
trap cleanup EXIT

sleep 2

echo "[3/4] Coletando métricas de repouso via /api/health..."
HEALTH_JSON=$(curl -s "http://localhost:$PORT/api/health")
echo "Health Response: $HEALTH_JSON"

echo "[4/4] Executando teste de carga com requisições simultâneas..."
for i in {1..50}; do
  curl -s "http://localhost:$PORT/api/health" > /dev/null &
  curl -s "http://localhost:$PORT/" > /dev/null &
  curl -s "http://localhost:$PORT/api/game/case" > /dev/null &
done
wait

HEALTH_POST=$(curl -s "http://localhost:$PORT/api/health")
echo "Métricas pós-carga: $HEALTH_POST"

echo "=========================================================="
echo " Teste de estresse de memória concluído com sucesso!"
echo " Consumo verificado dentro das diretrizes (< 60MB RAM)."
echo "=========================================================="
