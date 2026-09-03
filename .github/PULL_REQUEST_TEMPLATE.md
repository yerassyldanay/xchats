## What & why

<!-- The diff shows what changed; explain why. Link an issue if one exists. -->

## Testing

<!-- Commands you ran and their result. For a UI-facing change, confirm you
checked it in an actual browser — see CONTRIBUTING.md. -->

## Checklist

- [ ] `go build ./... && go vet ./... && go test -race -count=1 ./...` (backend, if touched)
- [ ] `npm run typecheck && npm run test:unit && npm run build` (frontend, if touched)
- [ ] Updated [`docs/overview.md`](../docs/overview.md) if this changes a recorded architecture decision
- [ ] Added a `CHANGELOG.md` entry if this is operator-visible
- [ ] Commits are signed off (`git commit -s`) — see CONTRIBUTING.md
