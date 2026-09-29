# Security policy

## Supported versions

Only the latest released minor version receives fixes.

## Reporting a vulnerability

Please do not open a public issue. Use GitHub's private vulnerability reporting ("Security" tab, "Report a vulnerability") on this repository. Include the version (`gocouple version`), the command you ran, and a minimal reproduction.

You can expect an acknowledgement within a few days and a fix or a mitigation plan after triage. Reports are credited unless you ask otherwise.

## Scope

`gocouple` reads source code and git history of repositories you point it at and writes report files. It runs `git` through `os/exec` in temporary worktrees and never modifies your working tree. Relevant issues include path traversal, command injection through repository content, unsafe handling of untrusted repositories, and tampering with release artifacts.

## Release integrity

Releases publish `checksums.txt` signed with keyless cosign, per-archive SBOMs, and build provenance attestations. See "Verifying releases" in the README.
