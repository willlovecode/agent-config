// The handoff procedure agtk appends to ~/.gemini/GEMINI.md, ported from geminiHandoff in
// internal/install/apply.go so the Node budget check can size that file without Go.
// internal/install/gemini_test.go compares the two texts; change both or `go test` fails.
export const GEMINI_HANDOFF =
  "## Writing a handoff\n" +
  "To hand work to another agent (usage limit, switching tools): run `git status` and `git diff --stat`, then write `.agent-handoff/latest.md` " +
  "in the repo root with the sections Goal, Done, Left, Files touched, Decisions and gotchas, How to run checks, Next step, and a summary of that git output. " +
  "Make sure `.agent-handoff/` is in `.git/info/exclude`; never commit it or put secrets in it. " +
  "Then give me this line for the next agent: \"Read .agent-handoff/latest.md, then run git status and git diff, then continue with Next step.\"\n";
