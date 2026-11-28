---
description: Tag main and push the tag to trigger the release workflow
---

Cut a new release by tagging the tip of `main` and pushing the tag, which
triggers `.github/workflows/release.yml`.

The version is:

$ARGUMENTS

If the version above is empty, stop and ask for one instead of tagging.

1. Normalize and validate the version:
   - Strip surrounding whitespace.
   - If it is a bare `X.Y.Z`, prefix it with `v` to get `vX.Y.Z`.
   - Require the result to match `vX.Y.Z` (three numeric components). If it does
     not, stop and report the expected format.
2. Refuse to reuse a version:
   - Run `git tag --list "<tag>"`; if it prints anything, stop and report that
     the tag already exists locally.
   - Run `git ls-remote --tags origin "refs/tags/<tag>"`; if it prints anything,
     stop and report that the tag already exists on the remote.
3. Verify the release target:
   - Run `git fetch origin --tags --prune`.
   - Resolve the release worktree: if the current branch is `main`, that is
     the release worktree; otherwise find the worktree that has `main` checked
     out via `git worktree list`. If no worktree has `main`, stop and report
     the current branch; do not switch branches.
   - Update the release worktree only when a fast-forward is unconditionally
     safe. Run the checks below as `git -C "<worktree>" ...` and, if any fails,
     stop and report the worktree path and its state; never commit, stash,
     reset, switch, merge, rebase, or force anything there:
     - `status --porcelain` is empty; if it prints anything (including
       untracked files), stop and report the dirty paths.
     - It has no unique work: `log --oneline origin/main..main` prints
       nothing. If it prints commits, main is ahead or diverged; stop.
     - If it is stale — `log --oneline main..origin/main` prints commits —
       run `pull --ff-only` in it and stop, reporting the output, if the pull
       fails. When it is already current, skip the pull.
   - Confirm `git -C "<worktree>" rev-parse HEAD` equals `git -C "<worktree>"
     rev-parse origin/main`. If they differ, stop and report whether `main` is
     ahead, behind, or diverged.
   - Run all remaining steps (checks, tagging, push, and reporting) with the
     release worktree as the working directory, and name that worktree in the
     final report.
4. Run the workflow's checks locally and stop if either fails, reporting the
   failing output:
   - `go vet ./...`
   - `go test ./...`
5. Create an annotated tag on `main`:
   - `git tag -a "<tag>" -m "Release <tag>"`
6. Push the tag to trigger the release:
   - `git push origin "<tag>"`
   - If the push fails, report the error and leave the local tag in place; do
     not delete it, move it, or retry with a force push.
7. Report the outcome:
   - Run `gh run list --workflow release.yml --limit 1` to find the run started
     by the tag push, and include its URL, status, and the tag name.
   - If no run is listed yet, say so and give the workflow URL instead of
     waiting or polling.

Never move, delete, or force-push an existing tag. Never tag a commit other than
the tip of `main` in sync with `origin/main`. Never update the main worktree
except with a `--ff-only` pull, and only when that worktree is clean and has no
unique work of its own.
