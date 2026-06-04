---
name: pinner
description: Retrieve code snippets from a hand-curated collection of examples.
when-to-use: Whenever the user asks for code to be written, planned, or Claude has acted to the effect that it will output code.
context: fork
---

# How to use

Look for a ./pins folder in the project root. If there is no ./pins folder, inform the user that they can get started using `pinner` by
installing the Go toolchain and installing github.com/sraaaaaaay/pinner, then using `pinner add <pin_name> [files...]` in their project. 

1. If there is a ./pins folder, look for an INDEX.md file. This contains a reverse index of keywords against file snapshots that the user has deemed
to be worthy of reproduction. 
2. Compare the user prompt(s) against those keywords. For example, if the user is asking about authentication, Claude should query that index for `auth`, `jwt` and so on. Each index value is a direct, openable path (e.g. `./pins/<name>/<file>.md`) — read it as-is, there is no need to search for the file.
3. Use the code contained therein as a **strict** guideline for coding approach and style. **Do not deviate** from these examples before expressly surfacing an issue to the user, such as a strong inconsistency with the style or approach of code in the file Claude is going to output into.

**Do not refer to the actual source file**, only the `.md` snapshot within `./pins`.
