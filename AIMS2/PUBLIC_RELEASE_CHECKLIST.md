# Public Release Checklist

The current working tree is configured for public development, but the repository must not be made public until the history and hosting checks below are complete.

## Required before changing repository visibility

- [ ] Rotate the Ollama web-search credential that was previously committed.
- [ ] Invalidate any other credentials found during the provider-side audit.
- [ ] Publish from a sanitized history that does not contain the old credential, private model endpoint, inventory database, or spreadsheet export.
- [ ] Confirm `apikey.env`, `importer/inventory.db`, inventory spreadsheets, and conversation logs are not tracked.
- [ ] Run a secret scanner against every branch and tag in the final public repository.
- [ ] Review GitHub issues, pull requests, releases, Actions artifacts, wiki pages, and branch names for private information.
- [ ] Review commit author names and email addresses for intended public disclosure.
- [ ] Enable GitHub private vulnerability reporting and secret scanning.
- [ ] Enable branch protection and required CI checks for the default branch.

## Recommended history strategy

Because both credentials and real inventory artifacts existed in Git history, the safest publication path is a new public repository or a new sanitized root commit containing only the reviewed current tree. This avoids accidentally publishing recoverable objects from old commits, branches, or tags.

If preserving history is required, use a dedicated history-rewrite tool such as `git filter-repo`, remove sensitive paths from every ref, replace exposed values in historical source, expire old refs, and verify the result with a full-history secret scan before force-pushing. Keep a private backup outside the public repository.

## Verification commands

Run from the repository root after creating the sanitized history:

```sh
git ls-files | grep -E '(^|/)(\.env|apikey\.env|conversation_log.*\.jsonl|inventory\.db)$|\.(xlsx|xls)$'
git log --all --objects
git status --short
```

The first command should return no matches. Use a maintained secret scanner for the complete history rather than relying only on text matching.
