## Scope (current change)
- Improve Windows/source-checkout install confidence by adding a PowerShell verification entrypoint that delegates to the canonical `./scripts/verify.sh` gate through a known-good Bash.
- Prefer Git for Windows Bash candidates before falling back to `bash` on `PATH`, because Windows `bash.exe` can resolve to an unconfigured WSL distro.
- Keep `./scripts/verify.sh` as the single canonical verify implementation; do not duplicate its check sequence in PowerShell.
- Update contributor/evaluator docs so Windows users have a clear verify command after quickstart.
- Keep the deployment contract untouched.

## Assumptions
- Git for Windows Bash is the lowest-friction Bash runtime for native PowerShell users who do not have a healthy WSL distro.
- The PowerShell wrapper should pass through verify flags instead of inventing new behavior.
- Existing Go tests under `scripts/` are the right place to guard wrapper contract text and delegation behavior.

## Risks
- Duplicating the verify implementation in PowerShell would create drift, so the wrapper must stay thin.
- Bash discovery can be platform-sensitive; test candidates before selecting one and print actionable install guidance when none work.
- Local Windows verification may still be limited by Docker, Go, Python, Node, or disk availability after the wrapper successfully reaches `verify.sh`.

## Test plan
- `go test ./scripts -run "TestPowerShellVerify|TestQuickstart" -count=1`
- `powershell -ExecutionPolicy Bypass -File .\scripts\verify.ps1 -Help`
- `powershell -ExecutionPolicy Bypass -File .\scripts\verify.ps1 --go-packages ./scripts`
- `git diff --check`
- `./scripts/verify.sh`
