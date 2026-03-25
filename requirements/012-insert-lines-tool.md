# Requirement 012: Insert Lines Tool

## Description
The harness must support an `insert_lines` tool that allows inserting a set of lines at a specified line number in a file.

## Acceptance Criteria
- [ ] Tool named `insert_lines` is available
- [ ] Accepts file path as input parameter
- [ ] Accepts line number as input parameter (1-indexed position to insert before)
- [ ] Accepts lines to insert as input parameter
- [ ] Inserts lines before the specified line number
- [ ] Inserting at line 1 inserts at beginning of file
- [ ] Inserting beyond file end appends to end of file
- [ ] Existing lines are shifted down after insertion
- [ ] Creates file if it does not exist
- [ ] Handles permission errors gracefully
- [ ] Handles disk full errors gracefully
- [ ] Returns confirmation of insertion with details
- [ ] Tool call failures are tracked in statistics
