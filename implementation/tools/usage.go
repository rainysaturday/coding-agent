package tools

// usageGuide maps tool names to guidance rendered in the system prompt
// ("How to call" and "Example use case").
var usageGuide = map[string][2]string{
	"bash":           {"Use the bash tool when you need to run shell commands, install packages, build projects, check file system, etc.", `"ls -la", "cat file.txt", "npm install", "pip install -r requirements.txt"`},
	"read_file":      {"Use read_file to view the contents of any file before making changes.", "Reading source files, configuration files, documentation"},
	"write_file":     {"Use write_file to create new files or completely overwrite existing files.", "Creating new source files, writing configuration, saving output"},
	"read_lines":     {"Use read_lines when you only need to view a portion of a large file.", "Viewing lines 1-50 of a large source file, checking specific sections"},
	"insert_lines":   {"Use insert_lines to add new content without replacing existing content.", "Adding imports, inserting new functions, adding comments"},
	"replace_text":   {"Use replace_text when you know the text to find but not the line numbers.", "Renaming variables, updating function names, fixing typos throughout a file"},
	"move_text":      {"Use move_text to move code blocks or text between files or within the same file.", "Moving a function from one file to another, reorganizing code sections"},
	"list_files":     {"Use list_files to see files, folders, sizes, permissions, and other information formatted like a simple ls command.", "Listing directory contents with details, checking file sizes, viewing hidden files"},
	"grep":           {"Use grep to search through file contents using grep-like pattern matching.", "Finding where a function is defined, searching for error messages, locating configuration values"},
	"git_log":        {"Use git_log to view commit history and understand changes in the repository.", "Reviewing recent changes, finding when a bug was introduced, understanding project history"},
	"git_show":       {"Use git_show to examine the details of a specific commit, including its changes and metadata.", "Examining a specific commit's changes, reviewing what was modified in a particular update"},
	"git_diff":       {"Use git_diff to compare different versions of files, branches, or commits.", "Comparing changes between two branches, viewing modifications in a specific commit, checking changes in the working tree"},
	"todo":           {"Use the todo tool to break down complex tasks into tracked sub-items. This helps you remember what to do between turns.", "Creating a checklist for a multi-step refactoring task"},
	"subagent":       {"Use the subagent tool to spawn a sub-agent for parallel or independent tasks.", "Delegating a code review to a sub-agent with a security persona"},
	"view_image":     {"Use view_image when you need to see what is in an image file, read text from screenshots, or analyze diagrams.", `"What does this screenshot show?", "Read the text in this diagram"`},
	"activate_skill": {"Activate a skill when a task matches one of the AVAILABLE SKILLS descriptions.", `Activating a "pdf-to-markdown" skill before converting a PDF`},
}

// UsageGuide returns the "How to call" and "Example use case" text for a tool.
func UsageGuide(name string) (string, string) {
	if g, ok := usageGuide[name]; ok {
		return g[0], g[1]
	}
	return "", ""
}
