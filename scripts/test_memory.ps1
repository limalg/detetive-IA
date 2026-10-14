# PowerShell script to test memory usage of Detetive IA binary
$ErrorActionPreference = "Stop"

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host " Detetive IA - Verificacao de Consumo de Memoria (RAM)" -ForegroundColor Cyan
Write-Host " Limite Alvo: < 60 MB RAM (Ambiente de 1GB RAM)" -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan

$env:PATH = "C:\Program Files\Go\bin;" + $env:PATH
$port = "8899"
$env:PORT = $port
$dbPath = "data/test_memory.db"
$env:DB_PATH = $dbPath

if (!(Test-Path "data")) {
    New-Item -ItemType Directory -Path "data" | Out-Null
}

Write-Host "[1/4] Compilando binario monolitico com flags de reducao..." -ForegroundColor Yellow
go build -ldflags="-s -w" -o bin/detetive-ia.exe ./cmd/server

Write-Host "[2/4] Iniciando processo do servidor..." -ForegroundColor Yellow
$proc = Start-Process -FilePath ".\bin\detetive-ia.exe" -PassThru
Start-Sleep -Seconds 2

try {
    Write-Host "[3/4] Consultando /api/health e WorkingSet do processo..." -ForegroundColor Yellow
    $health = Invoke-RestMethod -Uri "http://localhost:$port/api/health"
    $wsMB = [math]::Round($proc.WorkingSet64 / 1MB, 2)
    Write-Host ("Process WorkingSet (RSS): {0} MB" -f $wsMB) -ForegroundColor Green
    Write-Host ("Runtime Alloc: {0} MB | Sys: {1} MB" -f [math]::Round($health.alloc_mb, 2), [math]::Round($health.sys_mb, 2)) -ForegroundColor Green

    if ($wsMB -gt 60.0) {
        throw "Consumo de RSS excessivo: $wsMB MB > 60MB"
    }

    Write-Host "[4/4] Executando carga de requisicoes..." -ForegroundColor Yellow
    1..50 | ForEach-Object {
        $null = Invoke-RestMethod -Uri "http://localhost:$port/api/health" -Method Get
        $null = Invoke-RestMethod -Uri "http://localhost:$port/api/game/case" -Method Get
    }

    $proc.Refresh()
    $postWsMB = [math]::Round($proc.WorkingSet64 / 1MB, 2)
    $postHealth = Invoke-RestMethod -Uri "http://localhost:$port/api/health"
    Write-Host ("Pos-carga - WorkingSet (RSS): {0} MB | Alloc: {1} MB" -f $postWsMB, [math]::Round($postHealth.alloc_mb, 2)) -ForegroundColor Green

    if ($postWsMB -gt 60.0) {
        throw "Consumo de memoria pos-carga excedeu limite: $postWsMB MB > 60MB"
    }

    Write-Host "==========================================================" -ForegroundColor Cyan
    Write-Host " SUCESSO: Memoria aprovada! Consumo pico: $postWsMB MB (< 60MB)" -ForegroundColor Green
    Write-Host " O servidor operara deixando >900MB livres em maquinas de 1GB." -ForegroundColor Green
    Write-Host "==========================================================" -ForegroundColor Cyan
} finally {
    Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
    Remove-Item -Path "data/test_memory.db*" -Force -ErrorAction SilentlyContinue
}
