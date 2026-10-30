<picture>
  <source media="(max-width: 700px), (hover: none) and (pointer: coarse)" srcset="https://raw.githubusercontent.com/mainplane-ai/github-media/main/hero-mobile.svg">
  <img src="https://raw.githubusercontent.com/mainplane-ai/github-media/main/hero-desktop.svg" alt="Mainplane, with the biplane flying over the clouds" width="960">
</picture>
<div><h3>Mainplane is an open-source agent harness</h3></div>
<picture>
  <source media="(prefers-color-scheme: dark)" srcset="https://raw.githubusercontent.com/mainplane-ai/github-media/main/tui-dark.svg">
  <img src="https://raw.githubusercontent.com/mainplane-ai/github-media/main/tui-light.svg" alt="The Mainplane TUI: a sidebar of sessions on different models, and an agent that moves a Qwen fine-tune from a GPU server to a Mac Studio and quantizes it to run there" width="960">
</picture>

# This fork: hybrid session memory

A fork of [mainplane-ai/mainplane](https://github.com/mainplane-ai/mainplane), everything upstream plus two additions to how an agent session remembers. Upstream, a tool result stays whole in the context of every later step: a 30 KB payload costs the same bytes again at every following step, and the session fails the moment the whole no longer fits the context limit. And what the session holds is only in raw record bodies, with no map back to them. This fork keeps a map of the session in the model's context and holds the exact raw bytes one retrieval tool call away: no summarizing anywhere, so nothing is lost and nothing is forgotten.

**The index record.** After every step the harness appends one `index` record to the log, one line per call, naming the record range that holds the raw bytes, the tool, the file or the command, the result size, and the exit code:

```
recs 40-41 read schema.json 31000B
recs 42-43 run on build "go test ./..." 187B exit 0
```

Only the newest index record stays in the context, and its body is capped at 4 KiB, oldest lines first. Built by the harness alone, so it costs no model call and is exact.

**Raw byte pruning.** A result body over 4 KiB leaves the context once a later step has acted on it. The body becomes one line pointing at its record and at the scratch file a run spilled to:

```
[result body 31000 bytes out of context, record 41 of the log has it, ...]
```

The step's own results stay complete, so the model is always answering what it can still read. The cut is a pure function of the chain prefix, so a provider request still rebuilds byte for byte.

**The `logs` tool.** Searches the raw record bodies of a session's log, this one by default or another session's by id, bounded by a query and a `from`/`to` record range, and reports a bounded window around each match:

```
logs(query="user schema", from=40, to=41)
```

The model knows exactly what to search for, because the index is in its context: semantic loss zero, drop loss zero.

This work is on branch `memory-index-logs`, open as a pull request against upstream. Read more in [docs](docs/) for everything else upstream.

# Self-Hosting Quickstart

Run this command on your server machine (any always-on machine will do):

```shell
# Linux, macOS
curl -fsSL https://mainplane.ai/install | sh -s -- server quickstart

# Windows
& ([scriptblock]::Create((irm https://mainplane.ai/install.ps1))) server quickstart
```

The quickstart walks you through adding a provider, workers (computers your agents control) and connectors (ways to message your agents).

Read more in [docs/self-hosting.md](docs/self-hosting.md).

<br><br>

# Features
### Model agnostic
- Mainplane can use any model from any provider, including Anthropic, OpenAI, Google, and more.

### Context efficient
- Mainplane agents have a restricted set of tools and manage their own context by retrieving key information from other sessions.

### Connects all your devices
<picture>
  <source media="(prefers-color-scheme: dark)" srcset="https://raw.githubusercontent.com/mainplane-ai/github-media/main/architecture-3d-dark.svg">
  <img src="https://raw.githubusercontent.com/mainplane-ai/github-media/main/architecture-3d-light.svg" alt="The Mainplane Architecture: connectors reach Mainplane Server, which holds secrets and controls, observability and the file server; the network joins the server and every worker" width="960">
</picture>

- Mainplane links agents and computers with fine-grained security controls. No Tailscale needed.

### Shared file drives
- Mainplane sets up a remote filesystem you can use concurrently without risk of de-sync.

### Browser and computer use
- Mainplane drives your real browsers and applications with native computer use.

# FAQ
<details>
<summary>Can I bring my Claude Code or ChatGPT subscription?</summary>

Yes, Mainplane can use the regular limits of a Claude Code or ChatGPT Codex subscription, or an API key with any model.
</details>

<details>
<summary>Does Mainplane share my data?</summary>

No data is shared by default. Users can opt in to sharing data to improve Mainplane in /settings.
</details>

<details>
<summary>Do I need more than one device?</summary>

No. Everything can run on one device.
</details>

<details>
<summary>Does mainplane-server need Linux?</summary>

No. mainplane-server runs on macOS, Windows and Linux. Hosting shared files requires a compatible connected Linux device.
</details>

<details>
<summary>Do you take contributions?</summary>

We welcome feedback as GitHub issues. Pull requests are open to collaborators only. Any agent can POST plain text to https://bugs.mainplane.ai to report a bug.
</details>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="https://raw.githubusercontent.com/mainplane-ai/github-media/main/footer-still-dark.svg">
  <img src="https://raw.githubusercontent.com/mainplane-ai/github-media/main/footer-still-light.svg" alt="A billboard turns through what Mainplane agents do, while the biplane tows a banner: Now you're flying!" width="960">
</picture>
