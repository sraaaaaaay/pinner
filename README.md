# Pinner

`pinner` is a simple command-line tool for curating code snippets into snapshots ('pins') that can be efficiently retrieved by AI coding tools. 

## Value
- **Reproducibility:** guidelines and examples help to reduce shotgunning for "existing patterns".
- **Positive feedback loop:** manual curation of examples encourages the user to slow down and read generated code. Pin good output and use it to improve future responses.

## Usage

### Command line

```
go install github.com/sraaaaaaay/pinner@latest
```
<br>

| Command                      	| Action                                                                                                                                                                         	|
|------------------------------	|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------	|
| pinner add `name` 	| Starts creating a named pin in `.pins/name` and updates the index 	|
| pinner clear                 	| Clears `.pins`                                                                                                                                                         	|
| pinner index                 	| Reindexes `.pins` without modifying content                                                                                                                        	|

### Claude Code Plugin

```
claude plugin marketplace add sraaaaaaay/pinner && claude plugin install pinner@pinner
```

The plugin comes with a `UserPromptSubmit` hook that will query the `pinner` index for relevant files if:  
 1. The user is asking Claude to plan or write code, and
 2. The current directory contains `.pins/INDEX.md`
