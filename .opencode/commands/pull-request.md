---
description: Push the current branch and open or update its pull request
---

Create a pull request for the current branch, targeting `main`.

The pull request title is:

$ARGUMENTS

If the title above is empty, stop and ask for one instead of creating the PR.

1. Validate the branch:
   - Run `git branch --show-current`. If it is `main` (or empty), stop and
     report that a pull request needs a feature branch.
2. Run pre-flight checks and stop on the first failure, reporting the output:
   - Run `git fetch origin --prune`.
   - Run `git status --porcelain`. If it prints anything (including untracked
     files), stop and report the dirty paths; do not commit, stash, or discard
     them.
   - Run `go vet ./...`.
   - Run `go test ./...`.
   - Run `git log --oneline origin/main..HEAD`. If it prints nothing, stop and
     report that the branch has no commits ahead of `main`.
3. Push the branch:
   - Run `git push -u origin HEAD`.
   - If the push is rejected, stop and report the error. Never force-push, and
     never amend or rebase existing commits as part of this command.
4. Check for an existing open pull request for this branch:
   - Run `gh pr list --head "$(git branch --show-current)" --state open --json
     number,url,title`.
   - If one exists, update it with `gh pr edit <number> --title "<title>"
     --body-file /tmp/opencode/pr-body.md` and report that it was updated
     rather than creating a duplicate.
5. Build the PR body before creating it:
   - Write it to `/tmp/opencode/pr-body.md` with a `## Summary` section of
     concise bullets derived from the commit subjects and messages in
     `git log origin/main..HEAD`, and a `## Test plan` section stating that
     `go vet ./...` and `go test ./...` passed locally.
6. Create the pull request:
   - `gh pr create --base main --head "$(git branch --show-current)" --title
     "<title>" --body-file /tmp/opencode/pr-body.md`
   - Do not open it as a draft.
7. Report the outcome:
   - Include the PR URL, number, title, base and head branches, and the number
     of commits included.
   - Do not merge the PR, enable auto-merge, or push further changes.

Never force-push, never push to `main`, and never open a second PR when one is
already open for the branch.
