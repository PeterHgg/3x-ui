# Push 改动并创建 GitHub Release（触发 Action 构建）
# 在项目根目录用 PowerShell 运行： .\push-and-release.ps1
# 或以普通用户权限打开终端执行下方命令

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

$tag = "v2.8.79"
$commitMsg = "style(login): 美化登录页伪装官网 UI - Teal 配色、Plus Jakarta Sans、客户登录入口"
$releaseNotes = @"
登录页 UI 美化：

- Teal 青绿配色、Plus Jakarta Sans 字体
- 导航栏新增「客户登录」按钮
- 保留隐藏入口：键盘输入 admin / Logo 连点 5 次
- 下载区、卡片、弹窗样式统一优化
"@

Write-Host "=== 1. 暂存并提交 ===" -ForegroundColor Cyan
git add web/html/login.html
git status
git commit -m $commitMsg
if (-not $?) { Write-Host "提交失败或暂无改动" -ForegroundColor Yellow; exit 1 }

Write-Host "`n=== 2. 推送到 origin ===" -ForegroundColor Cyan
$branch = git rev-parse --abbrev-ref HEAD
git push origin $branch
if (-not $?) { Write-Host "Push 失败" -ForegroundColor Red; exit 1 }

Write-Host "`n=== 3. 打 tag 并推送 ===" -ForegroundColor Cyan
git tag -a $tag -m "Release $tag"
git push origin $tag
if (-not $?) { Write-Host "Tag 推送失败" -ForegroundColor Red; exit 1 }

Write-Host "`n=== 4. 创建 Release ===" -ForegroundColor Cyan
gh release create $tag --title $tag --notes $releaseNotes
if (-not $?) { Write-Host "Release 创建失败（需安装 gh 且已登录）" -ForegroundColor Red; exit 1 }

Write-Host "`n完成。Action 将自动构建并上传资产到 Release。" -ForegroundColor Green
