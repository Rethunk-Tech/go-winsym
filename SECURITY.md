# Security

## Supported versions

The latest tagged release. There are no long-lived release branches, so a fix ships in the next tag
rather than being backported.

## Reporting a vulnerability

Use GitHub's private vulnerability reporting on this repository: <https://github.com/Rethunk-Tech/go-winsym/security/advisories/new>, or the **Security** tab, then **Report a vulnerability**. The report stays private between you and us until a fix ships.

If private reporting is unavailable, email [oss@rethunk.tech](mailto:oss@rethunk.tech) with **[SECURITY]** in the subject.

Please do not open a public issue, pull request or discussion for a suspected vulnerability.

A useful report has the affected repository and version (or commit), steps to reproduce, what an attacker gets and under which conditions, and whether anyone else knows.

## What counts as a vulnerability

A vulnerability is a flaw that lets someone cross a boundary they should not: run code, read or change data they have no right to, escape a sandbox or path restriction, or leak a credential. A bug is anything else that misbehaves, such as a crash on your own input, wrong output or a slow path; open an ordinary issue for those. If you are unsure which one you have, report it privately and we will say.

## Coordinated disclosure

We handle the report inside the private advisory, so you see each status change and can comment on the fix. The advisory goes public when the fix ships, or 90 days after your report, whichever comes first. If a fix is close, either of us can ask for more time in the thread. You are credited in the advisory unless you would rather not be.

## Response timelines

- We acknowledge your report within 3 business days.
- We give you an assessment (accepted, needs more information or declined, with reasons) within 10 business days.
- We aim to ship a fix within 90 days of the report.

## What is in scope

- `ParseMinidump` reads a dump another program wrote: it must not panic, read outside the dump or allocate more than its fixed stream, module, name and CodeView bounds allow, however the counts and offsets lie.
- `PECodeView` reads a PE file at a path: a crafted debug directory must not panic or allocate beyond the bound on its size and record.
- `Lookup` reads a PDB a symbol server, or anyone on the path of a plain-http address, could answer: a crafted container must not panic or make it allocate beyond the directory and stream bounds or past the file's size.
- `crashdump` reads dump files, Unity crash folders, event-log XML and `coredumpctl` output: files open through a root on their own folder (a symlink or `..` cannot leave it), dumps over 4 GiB are skipped, `error.log` and the event stream are capped at 1 MiB and 2 MiB, a managed stack is cut at 16 KiB, and an exe name with a path in it is ignored.
- `CodeView.PDB` is the one string from those inputs that becomes a path and a URL: when non-empty it matches `^[A-Za-z0-9_.-]+\.pdb$`, so it cannot hold a folder or traversal.
- `Resolver` downloads from servers: a response must not exceed the download cap however its `Content-Length` reads, and nothing it writes may leave the cache folder.

## What is out of scope

- The caller chooses the paths it hands `PECodeView`, the `Dir` a `Resolver` owns and the `Servers` it trusts; a `Server` list that names a hostile host is a configuration choice. A server on plain http can serve a wrong PDB; the cache is not authenticated, and a wrong PDB yields a wrong name, never code execution.
- Names returned from a PDB are text from the input and are the caller's to display safely.
- A hand-placed file in `Dir` for a module no server serves is ignored by design.
- Time spent on the network by a slow server is bounded by the download timeout and `Wait`, and a `Resolver` runs at most two downloads at once; there is no rate limit.
- The default Unity server list keeps Unity's plain-http address as a fallback, so an observer on the path can see a module's PDB name; the default client does not follow a redirect from https to http.

## Safe harbour

We will not pursue legal action against good-faith research that avoids privacy violations, data destruction and disruption of services, stops at a proof of concept, and keeps the details private until the advisory is published. Rethunk.Tech, LLC maintains this repository.
