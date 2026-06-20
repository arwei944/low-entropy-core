# lec — Low-Entropy Core CLI v0.3.0
# Usage: .\lec.ps1 <command> [options] [args]
#
# Project Commands:
#   init      Create a new Low-Entropy Core project
#   add       Add a primitive (Atom/Port/Adapter/Composer) to existing project
#   check     Check if project follows 4-primitive constraints
#   upgrade   Upgrade project tier (l0 -> l1 -> l3)
#   list      List all lec-managed projects in current directory
#   version   Show version info
#
# Migration Commands:
#   analyze   Parse project source code
#   pattern   Recognize four-primitive patterns
#   plan      Generate migration plan
#   migrate   Execute full migration
#   log       Query migration logs
#   validate  Validate architecture constraints
#   rollback  Rollback migration steps
#   shim      Manage shim files
#   help      Show help

param(
    [Parameter(Position=0)]
    [string]$Command = "",

    [Parameter(Position=1, ValueFromRemainingArguments=$true)]
    [string[]]$RestArgs = @(),

    # init options
    [string]$Tier = "l0",
    [string]$Module = "",
    [string]$Desc = "",
    [string]$Remote = "",
    [string]$CorePath = "",

    # add options
    [string]$Type = "",       # atom, port, adapter, composer
    [string]$Name = "",       # e.g. "CalculatePrice"
    [string]$Target = "",     # target directory (default: current)

    # check options
    [switch]$Detailed,

    # analyze/pattern options
    [string]$Lang = "auto",
    [string]$Output = "text",

    # pattern options
    [double]$Threshold = 0.4,
    [switch]$GateOnly,

    # migrate options
    [switch]$Force,
    [switch]$DryRun,
    [string]$Only = "all",
    [string]$Skip = "none",

    # validate options
    [switch]$Fix,

    # rollback options
    [string]$Step = "last",
    [switch]$All
)

$LEC_VERSION = "0.3.0"

$ScriptDir = if ($PSScriptRoot) { $PSScriptRoot } else { Split-Path $MyInvocation.MyCommand.Path }
. "$ScriptDir\lec-helpers.ps1"
. "$ScriptDir\lec-init.ps1"
. "$ScriptDir\lec-templates.ps1"
. "$ScriptDir\lec-templates-l3.ps1"
. "$ScriptDir\lec-templates-snippets.ps1"
. "$ScriptDir\lec-commands.ps1"
. "$ScriptDir\lec-commands2.ps1"
. "$ScriptDir\lec-migrate.ps1"

switch ($Command) {
    "init"      { Cmd-Init }
    "add"       { Cmd-Add }
    "check"     { Cmd-Check }
    "upgrade"   { Cmd-Upgrade }
    "list"      { Cmd-List }
    "version"   { Cmd-Version }
    "analyze"   { Cmd-Analyze -ProjectDir $RestArgs[0] -Lang $Lang -Output $Output -Detailed:$Detailed }
    "pattern"   { Cmd-Pattern -ProjectDir $RestArgs[0] -Output $Output -Detailed:$Detailed -GateOnly:$GateOnly -Threshold $Threshold }
    "plan"      { Cmd-Plan -ProjectDir $RestArgs[0] -Tier $Tier -Detailed:$Detailed }
    "migrate"   { Cmd-Migrate -ProjectDir $RestArgs[0] -Tier $Tier -Lang $Lang -CorePath $CorePath -DryRun:$DryRun -Force:$Force -Step $Step -Only $Only -Skip $Skip -Detailed:$Detailed }
    "log"       { Cmd-Log -ProjectDir $RestArgs[0] -SubCommand $(if($RestArgs.Count -gt 1){$RestArgs[1]}else{"show"}) }
    "validate"  { Cmd-Validate -ProjectDir $RestArgs[0] -Detailed:$Detailed -Fix:$Fix }
    "rollback"  { Cmd-Rollback -ProjectDir $RestArgs[0] -Step $Step -All:$All -DryRun:$DryRun }
    "shim"      { Cmd-Shim -ProjectDir $RestArgs[0] -SubCommand $(if($RestArgs.Count -gt 1){$RestArgs[1]}else{"list"}) }
    "help"      { Cmd-Help }
    default     { Cmd-Help }
}
