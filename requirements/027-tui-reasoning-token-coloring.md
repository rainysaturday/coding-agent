# Requirement 027: TUI Reasoning Token Coloring

## Description
The coding agent harness must visually distinguish reasoning/thinking tokens from regular LLM output in the TUI. When LLMs respond with reasoning or thinking content (separate from the main response), these reasoning tokens should be displayed in a darker text color to make them visually distinct from non-reasoning tokens.

## Background
Many modern LLMs produce reasoning or thinking content before their final response. This can include:
- Chain-of-thought reasoning
- Internal monologue
- Step-by-step problem solving
- Self-correction or verification steps

This reasoning content is valuable for transparency but should be visually distinct from the final response to improve readability.

## Acceptance Criteria
- [ ] Reasoning tokens are detected and separated from regular response content
- [ ] Reasoning tokens are displayed in a darker/dimmed text color
- [ ] Non-reasoning tokens use the default/text color
- [ ] The color distinction is visible in both streaming and non-streaming modes
- [ ] The implementation uses ANSI color codes (no external dependencies)
- [ ] Users can clearly distinguish reasoning from final response at a glance
- [ ] Color choice is compatible with common terminal color schemes

## Implementation Details

### Color Scheme
- Use a darker ANSI color for reasoning content (e.g., dimmed gray or dark gray)
- ANSI code: `\033[90m` (bright black/dim) or `\033[2m` (dim/faint)
- Reset to normal color after reasoning content: `\033[0m`

### Streaming Mode
When streaming LLM responses:
- Detect reasoning content chunks (from `reasoning_content` field in API response)
- Apply darker color to reasoning chunks as they stream
- Apply normal color to regular response chunks
- Maintain color state across streaming chunks

### Non-Streaming Mode
When receiving complete responses:
- Parse and separate reasoning content from regular content
- Apply appropriate colors before displaying

### Content Separation
The inference layer provides separate fields:
- `Content`: The main response text
- `ReasoningContent`: The reasoning/thinking text

The TUI should color these differently when displaying.

## Examples

### Streaming Display
```
[Streaming]
[Reasoning in dark color]Let me think about this problem step by step...[/reasoning]
[Normal color]Here's my solution:
func main() {
    fmt.Println("Hello, World!")
}
[/normal]
```

### Visual Example (with colors)
```
> Write a Go hello world program

[Assistant is thinking...]
[Dark gray]Okay, I need to create a simple Go program that prints "Hello, World!". 
The main package is required, along with fmt for printing.
[/dark gray]
[Normal]Here's the code:

```go
package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
}
```
[/normal]
```

## Technical Requirements

### New Color Constant
Add a new color constant for reasoning content:
```go
const (
    ColorReset  = "\033[0m"
    ColorRed    = "\033[31m"
    ColorGreen  = "\033[32m"
    ColorYellow = "\033[33m"
    ColorBlue   = "\033[34m"
    ColorCyan   = "\033[36m"
    ColorDim    = "\033[90m"  // Dim/bright black for reasoning
)
```

### New TUI Methods
Add methods to handle colored streaming:
- `StreamReasoningChunk(text string)` - Stream reasoning content with dim color
- `StreamNormalChunk(text string)` - Stream regular content with normal color
- Or modify existing `StreamChunk` to accept a content type parameter

### Backward Compatibility
- Existing `StreamChunk` method should continue to work
- New methods should be optional enhancements
- Default behavior: if reasoning type not specified, use normal color

## User Experience Guidelines

1. **Subtlety**: The dimming should be noticeable but not hard to read
2. **Consistency**: Reasoning always uses the same color
3. **Readability**: Text should remain readable on common terminal backgrounds
4. **Transparency**: Users understand which content is reasoning vs. response

## Testing Requirements

- Unit tests for color constant values
- Unit tests for reasoning chunk streaming
- Unit tests for normal chunk streaming
- Test that color reset is properly applied after each chunk
- Test compatibility with existing streaming functionality

## Security and Privacy Considerations

- Reasoning content may contain sensitive planning or internal thoughts
- Ensure reasoning content is properly sanitized before display
- Consider user preferences for hiding reasoning content entirely
