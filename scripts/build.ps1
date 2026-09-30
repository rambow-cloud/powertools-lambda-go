$ErrorActionPreference = 'Stop'
$env:CGO_ENABLED = '0'
$previousGOOS = $env:GOOS
$previousGOARCH = $env:GOARCH
$previousGOWORK = $env:GOWORK
Push-Location (Join-Path $PSScriptRoot '..')
try {
    $env:GOWORK = (Join-Path (Get-Location) 'go.work')
    foreach ($architecture in @('amd64', 'arm64')) {
        $env:GOOS = 'linux'
        $env:GOARCH = $architecture
        go build -trimpath -tags lambda.norpc -o "dist/$architecture/bootstrap" ./examples/basic
        if ($LASTEXITCODE -ne 0) { throw "Build failed for $architecture" }
    }
    Remove-Item Env:GOOS, Env:GOARCH -ErrorAction SilentlyContinue
    go run ./tools/package
    if ($LASTEXITCODE -ne 0) { throw 'Packaging failed' }
} finally {
    $env:GOOS = $previousGOOS
    $env:GOARCH = $previousGOARCH
    $env:GOWORK = $previousGOWORK
    Pop-Location
}
