# Linux amd64 构建脚本（PowerShell）
# 用法：在 PowerShell Core (pwsh) 下运行：
#   ./build_linux_amd64.ps1 -ProjectPath "." -OutputName "myapp"

param(
    [string]$ProjectPath = ".",
    [string]$OutputName = "deepinfra",
    [string]$Version = "0.0.0",
    [switch]$Strip = $false
)

# 进入项目目录
Set-Location -Path $ProjectPath

# 设置交叉编译环境变量
$env:GOOS = "linux"
$env:GOARCH = "amd64"

# 输出文件名（Linux 可执行文件不带扩展名）
$outputPath = Join-Path -Path (Get-Location) -ChildPath $OutputName

# 确保输出目录存在
$outputDir = Split-Path -Path $outputPath -Parent
if (-not (Test-Path $outputDir)) {
    New-Item -ItemType Directory -Path $outputDir -Force | Out-Null
}

Write-Host "开始构建：目标=linux/amd64 输出=$outputPath 版本=$Version"

# 可选的 ldflags，用于注入版本信息
$ldflagsParts = @()
if ($Version -and $Version -ne "0.0.0") {
    $ldflagsParts += "-X main.Version=$Version"
    if ($Strip) {
        $ldflagsParts += "-s"
        $ldflagsParts += "-w"
    }
}
$ldflags = $ldflagsParts -join " "

# 运行 go build - 使用 Start-Process 以确保参数正确传递（避免被 PowerShell 按空格拆分）
if ([string]::IsNullOrEmpty($ldflags)) {
    $argList = @("build", "-o", $outputPath)
} else {
    # ldflags 必须作为单个参数传递
    $argList = @("build", "-ldflags", $ldflags, "-o", $outputPath)
}
Write-Host "运行: go $($argList -join ' ')"

$proc = Start-Process -FilePath "go" -ArgumentList $argList -NoNewWindow -Wait -PassThru -WorkingDirectory (Get-Location)
$exitCode = $proc.ExitCode
if ($exitCode -ne 0) {
    Write-Error "构建失败，退出码 $exitCode"
    exit $exitCode
}

# 可选：设置可执行权限（针对在 Windows 上生成后拷贝到 Linux）
try {
    if (Test-Path $outputPath) {
        & chmod +x $outputPath | Out-Null
    }
} catch {
    # 在某些 Windows 环境下 chmod 不可用，忽略。
}

Write-Host "构建完成：$outputPath"

# 打包为 tar.gz
$tarName = "$OutputName-linux-amd64.tar.gz"
Write-Host "打包为 $tarName"
try {
    # 使用 tar，如果不可用会抛出
    & tar -czf $tarName -C (Get-Location) $OutputName
    Write-Host "打包完成：$tarName"
} catch {
    Write-Warning "打包失败：$($_.Exception.Message)"
}
