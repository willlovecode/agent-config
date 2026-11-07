# agtk setup wizard

`agtk` guides setup for Claude Code, Codex, and Gemini CLI / Antigravity. The
terminal and `--answers` modes share the wizard core and install engine.
Personal profiles live on disk. The catalog, base instructions, and vendored
packs ship inside the binary.

## Screens and keys

`agtk wizard` (or plain `agtk`) works like `p10k configure`. It shows one screen
at a time: the terminal is cleared before each page and the page is drawn from
the top, so nothing scrolls back. Titles are centered and bold. Choices sit at
the left margin with the key in parentheses, `(1)  Personal.`, and the prompt
lists exactly the keys that work on the page: `Choice [12brq] (Enter = 1):`.
An answer is one keypress, taken at once without Enter. Keys that do nothing
on a page are ignored. Ctrl-C quits and restores the terminal.

- **Single choice** (Personal or Company, the routing preset): a number picks
  and moves on. Enter takes the default marked under the question. Each choice
  has a preview; a routing preset shows its routes for the confirmed models.
- **Multiple choice** (work, languages, frameworks, tools, accounts, models,
  items): a number toggles `[x]` or `[ ]` and redraws the same page. **y** or
  Enter continues. **?** then a number describes a choice. Rows are grouped
  under their headings. A page holds at most nine rows, since one key picks a
  row, and fewer when the terminal is short; **n** and **p** turn pages.
- **Items** show the token cost of each item and of the selection against the
  context budget. **v** cycles the views All, Recommended, General, Popular and
  Blocked.
- **b** goes back one screen, **r** restarts from the beginning with fresh
  answers and a new system check, **q** quits and writes nothing. Nothing is
  written before the final **y**.
- An answer that cannot stand (no client selected, Personal on a managed
  machine) shows a message above the prompt and keeps the page; every other key
  still works.

The last screens are the Review (what each client gets), the save question,
**Ready to apply?** with the exact files and commands, the installer's
progress, and a done screen with the next steps: the PATH command when
`~/.agtk/bin` is not on PATH, restarting the clients, and a note on
`~/.gemini/GEMINI.md` when Gemini CLI / Antigravity was chosen.

Pages are pure functions of the wizard state and the terminal size, and the
terminal is read again for every page, so resizing works. The window should be
at least 60x16; 80x24 is assumed when the size is unknown. A smaller window gets
a page that offers the plain wizard. Raw mode is released while the installer
runs and restored on every exit path, including Ctrl-C, SIGTERM and panics. On
Windows the console is switched to virtual terminal processing; if that fails,
the plain wizard runs. Output is ASCII; `NO_COLOR` removes bold and dim too.

### Plain mode

`--plain`, input or output that is not a terminal (pipes, CI), and `TERM=dumb`
use the scrolling wizard over the same core. Press Enter after each answer.
Choice numbers run across pages. Use `?N` to preview, `/text` to filter, `/` to
clear, or type the name of an item view. On a multiple-choice question, `1 3`
toggles two choices and continues. In Items, Enter opens Review. `b` goes back
and `q` quits. The older `--ascii` flag and `<` for Back are accepted.

## Setup flow

1. **System** asks before discovering hardware and tools on PATH. Skip discovery
   to make manual choices. Managed policy applies either way.
2. **Profile** asks Personal or Company, followed by tasks, languages, runtimes,
   frameworks, and platforms. The answers inform component recommendations.
3. **Models** asks which clients to configure. Detected clients start selected.
   Accounts and confirmed models are optional. When models are confirmed,
   preview the routing presets or choose **Keep current models**.
4. **Items** starts with recommended components selected. Browse by task and
   view (plain mode also filters by text). Details include purpose, requirements, and source
   when available. Use `agtk wizard --project DIR` to offer project skills.
5. **Review** shows the selections by client, routing, project scope, and
   external access. Choose whether to save the profile, then inspect the file
   and command plan. Back from confirmation returns to the save choice.
6. **Apply** starts only after an explicit **y** at the final prompt. The
   installer shows its progress, then the done screen lists the follow-up steps. Use `agtk undo`
   to restore files from the latest backup.

## Profiles and model routing

`agtk --answers FILE` applies a YAML profile without prompts. An omitted item
list uses recommendations. An explicit list, including `[]`, sets the items
exactly. Add `--save` to remember the profile for `agtk apply`.

The wizard saves a profile only during Apply. The default location is
`%APPDATA%\agtk\profile.yaml` on Windows or `~/.config/agtk/profile.yaml` on
Linux (under `$XDG_CONFIG_HOME` when set). Backups live under `~/.agtk`.

Model presets use confirmed access for their task routes. Saver favors included
usage, Balanced uses general models for routine work and deeper reasoning for
hard tasks, and Quality favors the strongest confirmed reasoning models. The
wizard asks which models your accounts can use. It makes no account queries.

Omitting routing or setting `answers.routing: none` keeps current model settings.
New profiles with routing record `answers.plans` and `answers.models` alongside
the preset. Older Personal plan-only profiles retain legacy Claude routing.
The old `gemini` plan alias means `gemini_pro`. agtk can set confirmed defaults
for Claude Code and Codex and write routing guidance in their instructions.
Gemini CLI and Antigravity share the generated `~/.gemini/GEMINI.md`.

## Company policy

Company setup keeps the current models by default. Company blocks unapproved
MCP servers, network and command skills, plugins, global package installs,
third-party hooks, and raw profile-supplied Claude or Codex settings. The same
policy checks apply to the wizard, answers files, and `agtk apply`.

IT can set a `REG_SZ` value named `Policy` under `HKLM\SOFTWARE\Policies\agtk`
(64-bit registry view), or use `/etc/agtk/company-policy.yaml` on Linux. The
Linux file must be owned by root, with the file and parent directory writable
only by root. Use your approved catalog IDs and exact model IDs and efforts.

```yaml
allow_items:
  - context7
allow_models:
  - plan: company_gateway
    tool: codex
    id: approved-model
    class: standard
    efforts: [medium]
```

Machine policy requires Company restrictions for every setup. The allowlist
covers what agtk installs. Use the clients' managed settings to enforce policy
during sessions. [Claude Code settings](https://code.claude.com/docs/en/managed-settings),
[Codex managed configuration](https://learn.chatgpt.com/docs/enterprise/managed-configuration)

For German deployments, assess what data is sent, the processors and accounts
used, retention and training settings, and international transfers. Involve the
data protection officer and works council where applicable.
[DSK AI and data protection guide](https://www.datenschutzkonferenz-online.de/media/oh/20240506_DSK_Orientierungshilfe_KI_und_Datenschutz.pdf),
[BSI guidance](https://www.bsi.bund.de/SharedDocs/Downloads/DE/BSI/Publikationen/Broschueren/Management_Blitzlicht/Management_Blitzlicht_Generative-KI.pdf?__blob=publicationFile&v=3),
[EDPB transfer guidance](https://www.edpb.europa.eu/sme-compliant/international-data-transfers_en),
[BetrVG § 87](https://www.gesetze-im-internet.de/betrvg/__87.html)
