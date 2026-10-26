# Build-only entry point. Does not register devices, install drivers or change signing policy.
[CmdletBinding()]
param(
    [Parameter(Mandatory=$true)][string]$WilInclude,
    [Parameter(Mandatory=$true)][string]$CppWinRTInclude,
    [string]$BuildDirectory = '',
    [ValidateSet('Debug','Release')][string]$Configuration = 'Release'
)
$ErrorActionPreference = 'Stop'
$source = Split-Path -Parent $PSScriptRoot
if (!$BuildDirectory) { $BuildDirectory = Join-Path $source 'build-windows' }
foreach ($tool in @('cmake','python','cl')) {
    if (!(Get-Command $tool -ErrorAction SilentlyContinue)) {
        throw "$tool missing. Use a Visual Studio x64 developer PowerShell with Windows SDK 22000+ installed."
    }
}
if (!(Test-Path (Join-Path $WilInclude 'wil/cppwinrt.h'))) { throw 'Invalid WIL include directory' }
if (!(Test-Path (Join-Path $CppWinRTInclude 'winrt/base.h'))) { throw 'Invalid C++/WinRT include directory' }
function Invoke-Checked([string]$Program, [string[]]$Arguments) {
    & $Program @Arguments
    if ($LASTEXITCODE -ne 0) { throw "$Program failed: $LASTEXITCODE" }
}
Invoke-Checked 'python' @((Join-Path $source 'tests/test_camera_source.py'))
Invoke-Checked 'cmake' @('-S',$source,'-B',$BuildDirectory,'-A','x64',
    '-DRELEVO_BUILD_CAMERA_PROVIDER=ON',"-DRELEVO_WIL_INCLUDE=$WilInclude", "-DRELEVO_CPPWINRT_INCLUDE=$CppWinRTInclude")
Invoke-Checked 'cmake' @('--build',$BuildDirectory,'--config',$Configuration)
Invoke-Checked 'ctest' @('--test-dir',$BuildDirectory,'-C',$Configuration,'--output-on-failure')
$artifacts = @('RelevoCameraSource.dll','RelevoCameraControl.exe')
$rows = foreach ($name in $artifacts) {
    $file = Join-Path (Join-Path $BuildDirectory $Configuration) $name
    if (!(Test-Path $file)) { throw "Missing expected build artifact: $file" }
    Get-FileHash -Algorithm SHA256 $file | Select-Object Path, Hash
}
$rows | ConvertTo-Json | Set-Content -Encoding UTF8 (Join-Path $BuildDirectory 'artifact-hashes.json')
Write-Host 'Build and unit tests passed. Devices have NOT been installed or runtime validated.'
Write-Host 'The microphone kernel endpoint needs a separate WDK build and accepted driver signature.'
