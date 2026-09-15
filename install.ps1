# mano 安装脚本 (Windows)
# 用法: irm https://raw.githubusercontent.com/ParadiseWitch/mano/main/install.ps1 | iex
#       指定版本: $env:MANO_VERSION='v1.2.3'; irm ... | iex
$ErrorActionPreference = 'Stop'

$repo = 'ParadiseWitch/mano'
$version = if ($env:MANO_VERSION) { $env:MANO_VERSION } else { 'latest' }

$arch = $env:PROCESSOR_ARCHITECTURE.ToLower()
if ($arch -eq 'amd64') { $arch = 'amd64' }
elseif ($arch -eq 'arm64') { $arch = 'arm64' }
else { throw "mano: 不支持的架构 $arch" }

$asset = "mano-windows-$arch.exe"
$base = "https://github.com/$repo/releases"
if ($version -eq 'latest') {
  $base = "$base/latest/download"
} else {
  $base = "$base/download/$version"
}

$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ([System.IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
  Write-Host "==> 下载 $asset ($version)"
  Invoke-WebRequest -Uri "$base/$asset" -OutFile (Join-Path $tmp $asset) -UseBasicParsing
  Invoke-WebRequest -Uri "$base/checksums.txt" -OutFile (Join-Path $tmp 'checksums.txt') -UseBasicParsing

  $line = Get-Content (Join-Path $tmp 'checksums.txt') |
    Where-Object { $_ -match "(?m)\b$([regex]::Escape($asset))\s*$" } |
    Select-Object -First 1
  if (-not $line) { throw "mano: checksums.txt 中没有 $asset" }
  $expected = ($line -split '\s+')[0]
  $actual = (Get-FileHash (Join-Path $tmp $asset) -Algorithm SHA256).Hash.ToLower()
  if ($actual -ne $expected.ToLower()) { throw 'mano: 校验失败，中止安装' }
  Write-Host '==> sha256 校验通过'

  $destDir = Join-Path $env:LOCALAPPDATA 'Programs\mano'
  New-Item -ItemType Directory -Path $destDir -Force | Out-Null
  $dest = Join-Path $destDir 'mano.exe'
  Move-Item -Force (Join-Path $tmp $asset) $dest

  $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
  if ($userPath -notlike "*$destDir*") {
    [Environment]::SetEnvironmentVariable('Path', "$destDir;$userPath", 'User')
    Write-Host "==> 已把 $destDir 加入用户 PATH（重开终端生效）"
  }
  Write-Host "==> 已安装到 $dest"
} finally {
  Remove-Item -Recurse -Force $tmp
}
