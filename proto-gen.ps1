# proto-gen.ps1
Write-Host "============================================" -ForegroundColor Cyan
Write-Host "Generating Go code from proto files..." -ForegroundColor Cyan
Write-Host "============================================" -ForegroundColor Cyan
Write-Host ""

$projectPath = "C:\Users\USER\GolandProjects\StreamApi\api-gateway"
$protoDir = "$projectPath\proto"
$genDir = "$projectPath\internal\gen"

Write-Host "Project path: $projectPath" -ForegroundColor Yellow
Write-Host "Proto files:  $protoDir" -ForegroundColor Yellow
Write-Host "Output dir:   $genDir" -ForegroundColor Yellow
Write-Host ""

# Проверяем наличие proto файлов
if (-Not (Test-Path $protoDir)) {
    Write-Host "❌ ERROR: Proto directory not found: $protoDir" -ForegroundColor Red
    exit 1
}

$protoFiles = Get-ChildItem -Path $protoDir -Filter "*.proto"
if (-Not $protoFiles) {
    Write-Host "❌ ERROR: No .proto files found in $protoDir" -ForegroundColor Red
    exit 1
}

Write-Host "Found proto files:" -ForegroundColor Green
foreach ($file in $protoFiles) {
    Write-Host "  • $($file.Name)" -ForegroundColor Green
}
Write-Host ""

# Создаем директорию для выходных файлов
if (-Not (Test-Path $genDir)) {
    New-Item -ItemType Directory -Path $genDir -Force | Out-Null
    Write-Host "Created output directory: $genDir" -ForegroundColor Gray
}

Write-Host "Starting Docker container for generation..." -ForegroundColor Cyan

# Запускаем Docker
docker run --rm `
    -v "${projectPath}:/workspace" `
    -w "/workspace" `
    namely/protoc-all:latest `
    -f proto/client.proto `
    -l go `
    -o internal/gen `
    --go-source-relative

if ($LASTEXITCODE -eq 0) {
    Write-Host ""
    Write-Host "============================================" -ForegroundColor Green
    Write-Host "✅ SUCCESS: Proto files generated!" -ForegroundColor Green
    Write-Host "============================================" -ForegroundColor Green
    Write-Host ""

    # Показываем сгенерированные файлы
    if (Test-Path $genDir) {
        $generated = Get-ChildItem -Path $genDir -Recurse -File
        Write-Host "Generated files ($($generated.Count)):" -ForegroundColor Yellow
        foreach ($file in $generated) {
            Write-Host "  • $($file.Name)" -ForegroundColor White
        }
    }

    Write-Host ""
    Write-Host "📁 Output directory: $genDir" -ForegroundColor Cyan
} else {
    Write-Host ""
    Write-Host "============================================" -ForegroundColor Red
    Write-Host "❌ ERROR: Failed to generate proto files" -ForegroundColor Red
    Write-Host "============================================" -ForegroundColor Red
}

Write-Host ""
Write-Host "Press any key to continue..."
$null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")