## Environment (work)
- Windows, PowerShell. Models are served through the company's LLM gateway; use
  the aliases it provides, not model IDs from elsewhere.
- Main project: a partly legacy Java codebase migrating to the cloud, in energy
  trading. Follow the project's own AGENTS.md and build commands first.

## Work data
- Company code, trading data, prices, positions, counterparties and customer data
  stay inside the company: never send them to an external service, MCP server or
  website, and never paste them into a public issue, gist or search.
- Treat cloud, database and deployment tooling as read-only unless I ask for a
  change in that turn; show the exact command before anything that writes.

## Legacy code
- Before changing legacy behaviour, pin it with a characterization test, then
  change it. Say when no test seam exists instead of skipping the test.

## Model choice
Map roles to whatever the gateway serves (fill in on day one): hardest design,
debugging and review on the strongest model at medium effort; implementation from
a clear brief on the mid model; finding things and mechanical edits on the
smallest model. Subagents use the aliases opus, sonnet and haiku.
