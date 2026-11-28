---
description: Review the diff, commit all changes with a message, then rebase onto origin/main
---

Review and commit the project's current changes, then sync with the remote.

The commit message is:

$ARGUMENTS

If the message above is empty, stop and ask for a commit message instead of committing.

1. Review the diff before staging:
   - Run `git status --short` to see modified, staged, and untracked files.
   - Run `git diff` and `git diff --cached` to inspect the changes.
   - Read any new untracked files.
   - Flag anything that looks accidental or should not be committed, such as debug
     output, commented-out code, secrets, credentials, or unrelated files. If you
     find secrets or clearly accidental changes, stop and report them rather than
     committing.
2. Stage the project changes with `git add -A`.
3. Commit with the provided message. Pass the message as a single literal argument
   to `git commit -m`; do not reinterpret, split, or quote it.
4. Run `git pull --rebase origin main`. If the rebase stops on conflicts, resolve
   only trivial ones introduced by this change. If a conflict is non-trivial or
   unclear, abort the rebase with `git rebase --abort` and report the conflict
   instead of guessing.

Finish by reporting the commit hash, the commit subject, and the outcome of the
rebase (in sync, rebased, or aborted due to conflicts).
