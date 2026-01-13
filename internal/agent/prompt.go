package agent

import (
	"fmt"
	"strings"

	"github.com/nickcecere/btcx/internal/resource"
)

// SystemPrompt generates the system prompt for the agent
func SystemPrompt(collection *resource.Collection) string {
	var sb strings.Builder

	sb.WriteString("You answer coding questions by searching these repositories:\n\n")

	for _, r := range collection.Resources {
		sb.WriteString(fmt.Sprintf("## %s\n", r.Name))
		sb.WriteString(fmt.Sprintf("Directory: ./%s\n", r.Name))
		if r.Notes != "" {
			sb.WriteString(fmt.Sprintf("Notes: %s\n", r.Notes))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(`## Available Tools

You have EXACTLY these 4 tools available - use ONLY these tools:

1. **grep** - Search file contents using regex patterns
2. **glob** - Find files matching a glob pattern (e.g., "*.go", "**/*.md")
3. **read** - Read contents of a specific file
4. **list** - List directory contents

DO NOT try to use any other tools (like "search" or "find"). They do not exist.

## How to Answer

1. SEARCH FIRST using grep or glob before answering.
2. After finding relevant code (1-3 searches), STOP SEARCHING and write your answer.
3. Use grep to find code containing specific patterns.
4. Use glob to locate files by name.
5. Use read to examine specific files you found.
6. Quote code directly from results with file paths.
7. IMPORTANT: Once you have enough information to answer, respond immediately - do not keep searching.
8. Say "not found in repos" if you can't find relevant code after 2-3 searches.

## When to Stop Searching

- After 2-3 successful searches that return relevant code, WRITE YOUR ANSWER
- Do not search for every possible variation - synthesize from what you found
- If a search returns useful results, use them in your answer rather than searching more

## Response Format

- Use Markdown format
- Include code blocks with the source file path
- Keep explanations brief and focused
- Answer based on actual code, not assumptions

## Search Tips

- Search for function/class/variable names exactly
- Try multiple search terms if the first search fails
- Check imports to find related code
- Look at test files for usage examples
`)

	return sb.String()
}

// ToolDescriptions returns descriptions for all tools
var ToolDescriptions = map[string]string{
	"grep": `Search file contents using regex patterns. Use this to find code containing specific patterns.`,
	"glob": `Find files matching a glob pattern. Use this to locate files by name.`,
	"read": `Read the contents of a file. Use this to examine specific files.`,
	"list": `List directory contents. Use this to explore the codebase structure.`,
}

// StuckLoopHint returns a hint to add to the system prompt when the model appears stuck
func StuckLoopHint() string {
	return `

## IMPORTANT: Search Guidance

Your previous searches have not returned useful results. Please:

1. Try DIFFERENT search patterns - avoid repeating the same searches
2. Use simpler, more general patterns (e.g., just the function name, not the full signature)
3. Try searching in different directories or with different file extensions
4. If you've tried 2-3 different patterns without success, provide your best answer based on general knowledge and explain that specific code examples were not found in the repository
5. Do NOT repeat searches that returned "No files found"

If you cannot find relevant code, it's better to give a helpful general answer than to keep searching indefinitely.
`
}
