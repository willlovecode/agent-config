# Changes made to vendored skills

Vendored skills are copied unchanged from their pinned commit (see skills/baseline.json),
except for these edits. Re-apply them after updating a pin.

- research/paper-lookup: removed the final section "Citing Scientific Agent Skills"
  (told the agent to add K-Dense's paper to the user's manuscripts and code releases).
  `allowed-tools` is stripped at install time by skills/install-skills.ps1, not here.
- java/spring-boot-skill: appended "Known issues in this copy" (defects found in the
  2026-11-04 review). Folder named after the frontmatter name `spring-boot-skill`.
- extras/caveman: simplified mode status for standalone use without the upstream
  runtime or companion skills. Included the applicable notices and licenses.
- extras/find-skills: made lookup read-only and kept installation behind approval
  for the exact source and scope. Removed global and confirmation-bypass flags.
