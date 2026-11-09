param(
    [Parameter(Mandatory = $true)]
    [string]$Version
)

$iscc = @(
    "$env:LOCALAPPDATA\Programs\Inno Setup 6\ISCC.exe",
    'C:\Program Files (x86)\Inno Setup 6\ISCC.exe',
    'C:\Program Files\Inno Setup 6\ISCC.exe'
) | Where-Object { Test-Path -LiteralPath $_ } | Select-Object -First 1

if (-not $iscc) {
    throw 'Inno Setup 6 not found. Install it first: winget install JRSoftware.InnoSetup'
}

& $iscc "/DMyAppVersion=$Version" (Join-Path $PSScriptRoot 'magpie.iss')
exit $LASTEXITCODE