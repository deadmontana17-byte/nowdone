# Local dev stack: test bot @nowdonetest_bot, DB "mydaily", ENV=test.
# docker compose only auto-loads .env, so local runs must pass --env-file .env.local.
# Usage:
#   .\dev-local.ps1            # build + up -d
#   .\dev-local.ps1 logs -f    # any compose subcommand, still bound to .env.local
#   .\dev-local.ps1 down

param([Parameter(ValueFromRemainingArguments = $true)] [string[]] $Args)

$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot

if (-not $Args -or $Args.Count -eq 0) {
    docker compose --env-file .env.local up -d --build
    docker compose --env-file .env.local ps
} else {
    docker compose --env-file .env.local @Args
}
