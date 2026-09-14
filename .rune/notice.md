# Welcome to Rune

You're reading this inside the editor whose source is open in front of you.
Change a line, rebuild, and you're off to the races:

```bash
go run ./cmd/rune
```

`README.md` has the repository tour, prerequisites, and the rest of the targets.
`CONTRIBUTING.md` has the DCO policy (every commit needs to be signed-off via `git commit -s`).

## Come hack with us

Join the Discord: https://discord.gg/xzte9J8f8N for questions, design arguments,
"where does this live?", and the people who'll review your patch.

And contributors won't just get a thank-you: Unstable Build will distribute a
share of its profits to the people who build Rune. The contributor program is
being finalized and will be announced soon.

Pick something that annoys you and fix it.

---

Psst! you can add notices like this one for other Rune users
in your projects by adding a .rune/config.yaml with the following:

```yaml
workspace:
  notice:
    path: .rune/notice.md
    show: once
```

And a `.rune/notice.md` file:

```markdown
Hello, this is a notice!
```
