# Repository instructions

Apply `docs/standards/` to all changes. Read the relevant Go, schema, client API,
and library standards before changing their subjects. The reusable template's
library checklist is maintained in `docs/checklist.md`.

Keep comparison notes and unrelated local files uncommitted. Do not commit
credentials, private captures, generated executables, or site build output.
Run `make lint` and `make check` before describing an implementation as complete.
Use two independent reviewers to audit the exact final commit against these
standards; keep unverified release requirements open.
