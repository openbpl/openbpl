# OpenBPL

Every day, attackers register thousands of new domains that impersonate real brands. They buy a fresh TLS certificate, throw up a login page that looks identical to the real one, and start collecting passwords within minutes.

OpenBPL watches that happen, in real time, and tells you about it.

## What it actually does

OpenBPL is a small Go program that turns the public internet into a tripwire for your brand.

1. It listens to the Certificate Transparency stream, where every new TLS certificate on the internet shows up the moment it is issued.
2. It filters that firehose for domain names that look like one of your brands. If you protect "PayPal", a freshly issued cert for `paypa1-secure-login.xyz` will trip the wire.
3. It opens the suspect page in a real headless Chromium, grabs the rendered HTML and a screenshot.
4. It runs a stack of detection rules against the evidence. Built in rules include perceptual favicon matching (so attackers cannot just rename the file) and login form detection (a password field on a domain you do not own is a strong signal).
5. You can write your own rules in TypeScript using the included SDK. Each rule receives the page and your brand config, and returns labels with a confidence score.
6. It looks up the hosting and registrar abuse contacts through phish.report so you can actually do something about it.
7. Everything lands in a local SQLite file, with live activity streaming through a terminal UI.

No accounts, no SaaS, no upload of your brand assets to a vendor. Your config and your detections stay on your machine.

## Quick start

```bash
git clone https://github.com/openbpl/openbpl
cd openbpl
make setup            # installs the headless Chromium that Playwright drives
make build

./bin/openbpl create paypal     # opens a wizard to describe the brand
cd paypal
../bin/openbpl start            # opens the live TUI and starts hunting
```

If you want to skip the wizard and edit `config.yaml` by hand, use `openbpl create paypal --blank`. Fully wired example projects for PayPal and OpenAI live in `examples/`.

## Writing a rule

Rules are tiny. Here is a complete one that flags any page bragging about a "secure login" while sitting on a domain that is not yours.

```ts
import { defineRule } from "@openbpl/sdk";

export default defineRule({
  name: "fake-secure-banner",
  evaluate: ({ evidence, brand }) => {
    if (!/secure login/i.test(evidence.title)) return null;
    if (brand.urls.domains.some((d) => evidence.domain.endsWith(d))) return null;
    return {
      name: "fake-secure-banner",
      confidence: 0.7,
      detail: "claims to be a secure login but is not on a brand domain",
    };
  },
});
```

Drop it into your project's `rules/` folder. The runtime picks it up automatically and feeds it every capture.

## CLI

```
openbpl create <project-name> [--blank]   create a new brand project
openbpl start                             run the live detector and TUI
openbpl rule new <name>                   scaffold a new rule
openbpl rule list                         list rules in this project
openbpl rule test [name]                  test rules against saved captures
```

## Project layout

```
cmd/         the openbpl CLI entry point
internal/
  sources/   certstream client
  capture/   headless browser pool
  rule/      built in Go rules and the rule engine
  bridge/    talks to user written TypeScript rules over stdio
  detect/    glues a capture to the rules and produces verdicts
  store/     SQLite persistence
  tui/       the live terminal dashboard
  notify/    abuse contact lookups
  wizard/    interactive brand setup
sdk/         the @openbpl/sdk TypeScript package for rule authors
examples/    fully configured projects for PayPal and OpenAI
```

## Status

Early but working. The default ruleset already catches the obvious clones, and the SDK is stable enough to write real detections in. Expect rough edges in the wizard and the TUI while things settle.

## License

Apache 2.0. See `LICENSE`.
