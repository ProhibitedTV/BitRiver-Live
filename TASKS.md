## Scoped change: PowerShell verify wrapper for Windows install confidence

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Establish install-verification scope
  - Acceptance criteria:
    - `PLAN.md` captures the current scope, assumptions, risks, and test plan.
    - `TASKS.md` lists ordered tasks before source edits for this pass.
    - Existing quickstart/verify scripts and docs are reviewed.
  - Check:
    - Read-only analysis only.

- [x] Task 2 - Add a PowerShell verify wrapper
  - Acceptance criteria:
    - `scripts/verify.ps1` exists and delegates to `./scripts/verify.sh`.
    - The wrapper tests Bash candidates before use and gives clear Git Bash/WSL guidance when no usable Bash is found.
    - Verify flags pass through to the canonical shell script.
  - Check:
    - `powershell -ExecutionPolicy Bypass -File .\scripts\verify.ps1 --help` passed.

- [x] Task 3 - Add wrapper regression coverage
  - Acceptance criteria:
    - `scripts/quickstart_test.go` guards the PowerShell verify wrapper's delegation and Bash discovery contract.
    - Existing quickstart wrapper coverage remains intact.
  - Check:
    - `go test ./scripts -run "TestPowerShellVerify|TestQuickstart" -count=1` passed with offline Go env and temp cache directories.

- [x] Task 4 - Update install and testing docs
  - Acceptance criteria:
    - README and testing/quickstart docs show the PowerShell verify entrypoint.
    - Docs continue to identify `./scripts/verify.sh` as the canonical gate.
  - Check:
    - `git diff --check` passed.

- [-] Task 5 - Verify, publish, and merge
  - Acceptance criteria:
    - Focused tests pass or host blockers are recorded.
    - `git diff --check` passes.
    - `./scripts/verify.sh` or the PowerShell wrapper path runs, or host blockers are recorded.
    - Changes are committed, pushed, opened as a PR, monitored, and merged when checks pass.
  - Check:
    - `powershell -ExecutionPolicy Bypass -File .\scripts\verify.ps1 --help` passed.
    - `go test ./scripts -run "TestPowerShellVerify|TestQuickstart" -count=1` passed with offline Go env and temp cache directories.
    - `git diff --check` passed.
    - `powershell -ExecutionPolicy Bypass -File .\scripts\verify.ps1 --go-packages ./scripts` reached `./scripts/verify.sh` through Git Bash and then stopped at host prerequisite: no usable Python 3 interpreter (`python3`, `python`, or `py -3`) is installed behind the Windows Python launcher.

### Execution log
- Task 1 read-only pass:
  - Confirmed open issue queue remains focused on chat follow-up, 4K work, and release gates; no existing issue directly covers Windows verify entrypoint friction.
  - Reviewed `scripts/quickstart.ps1`, `scripts/quickstart.sh`, `scripts/verify.sh`, `scripts/quickstart_test.go`, `README.md`, `docs/quickstart.md`, and `docs/testing.md`.
  - Found quickstart has a PowerShell wrapper, but the default verify gate only documents Bash and known broken-WSL workarounds.
- Task 2 implementation:
  - Added `scripts/verify.ps1` as a thin PowerShell wrapper around `./scripts/verify.sh`.
  - Bash discovery now checks `BITRIVER_VERIFY_BASH`, Git for Windows locations, and then `bash` on `PATH`, testing each candidate before use.
  - Missing Bash guidance now calls out Git for Windows and `WSL_E_DEFAULT_DISTRO_NOT_FOUND`.
- Task 3 implementation:
  - Added `TestPowerShellVerifyWrapperDelegatesToCanonicalGate` to keep the wrapper thin, flag-pass-through oriented, and tied to the canonical Bash verify script.
- Task 4 documentation:
  - Updated `README.md` with the PowerShell verify entrypoint and broken-WSL rationale.
  - Updated `docs/testing.md` to describe `.\scripts\verify.ps1` as a wrapper around the canonical gate.
  - Updated `docs/quickstart.md` so source-checkout evaluators can verify from native PowerShell after first success.
- Verification so far:
  - `powershell -ExecutionPolicy Bypass -File .\scripts\verify.ps1 --help` passed.
  - `go test ./scripts -run "TestPowerShellVerify|TestQuickstart" -count=1` passed with offline Go env and temp cache directories.
  - `git diff --check` passed.
  - `powershell -ExecutionPolicy Bypass -File .\scripts\verify.ps1 --go-packages ./scripts` confirmed wrapper delegation through Git Bash, then stopped because this host has `C:\Windows\py.exe` but no installed Python 3 interpreter.
