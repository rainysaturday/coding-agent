package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// executeCsvTransformer handles CSV data transformation operations.
func (te *ToolExecutor) executeCsvTransformer(params map[string]interface{}) *ToolResult {
	command, ok := params["command"].(string)
	if !ok || command == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: command",
		}
	}

	data, sourceDesc, err := te.loadCsvData(params)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to load CSV data: %v", err),
		}
	}

	var result *ToolResult

	switch command {
	case "read":
		result = te.csvRead(data, sourceDesc)
	case "filter":
		result = te.csvFilter(data, params)
	case "select":
		result = te.csvSelect(data, params)
	case "sort":
		result = te.csvSort(data, params)
	case "aggregate":
		result = te.csvAggregate(data, params)
	case "to_json":
		result = te.csvToJson(data, sourceDesc)
	case "to_yaml":
		result = te.csvToYaml(data, sourceDesc)
	case "format":
		result = te.csvFormat(data, params, sourceDesc)
	case "rename_columns":
		result = te.csvRenameColumns(data, params)
	case "add_column":
		result = te.csvAddColumn(data, params)
	default:
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("unknown command: %s. Valid commands: read, filter, select, sort, aggregate, to_json, to_yaml, format, rename_columns, add_column", command),
		}
	}

	return result
}

// loadCsvData reads CSV data from a file path or raw CSV string.
func (te *ToolExecutor) loadCsvData(params map[string]interface{}) ([][]string, string, error) {
	hasFilePath := false
	hasCsvString := false

	if fp, ok := params["file_path"].(string); ok && fp != "" {
		hasFilePath = true
	}
	if cs, ok := params["csv_string"].(string); ok && cs != "" {
		hasCsvString = true
	}

	if !hasFilePath && !hasCsvString {
		return nil, "", fmt.Errorf("must provide either file_path or csv_string")
	}

	var data [][]string
	var delimiter rune = ','
	var sourceDesc string

	if hasFilePath {
		filePath := params["file_path"].(string)
		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil, "", fmt.Errorf("failed to read file: %v", err)
		}
		sourceDesc = filePath
		delimiter = te.detectDelimiter(string(content))
		data = te.parseCsvString(string(content), delimiter)
	} else {
		csvString := params["csv_string"].(string)
		if delimParam, ok := params["delimiter"].(string); ok && delimParam != "" {
			delimiter = rune(delimParam[0])
		}
		data = te.parseCsvString(csvString, delimiter)
		sourceDesc = "csv_string"
	}

	return data, sourceDesc, nil
}

// detectDelimiter auto-detects the delimiter used in CSV content.
func (te *ToolExecutor) detectDelimiter(content string) rune {
	lines := strings.SplitN(content, "\n", 5)
	delimiters := []rune{',', ';', '\t', '|'}
	bestDelimiter := ','
	bestScore := 0

	for _, d := range delimiters {
		counts := make([]int, 0)
		for _, line := range lines {
			if line == "" {
				continue
			}
			count := strings.Count(line, string(d))
			counts = append(counts, count)
		}
		if len(counts) > 1 {
			allSame := true
			for i := 1; i < len(counts); i++ {
				if counts[i] != counts[0] {
					allSame = false
					break
				}
			}
			if allSame && counts[0] > 0 {
				score := counts[0] * 10
				if score > bestScore {
					bestScore = score
					bestDelimiter = d
				}
			}
		} else if len(counts) == 1 && counts[0] > 0 {
			if counts[0] > bestScore {
				bestScore = counts[0]
				bestDelimiter = d
			}
		}
	}
	return bestDelimiter
}

// parseCsvString parses a CSV string with the given delimiter.
func (te *ToolExecutor) parseCsvString(content string, delimiter rune) [][]string {
	var records [][]string
	lines := strings.Split(content, "\n")

	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		record := te.parseCsvLine(line, delimiter)
		if len(record) > 0 {
			records = append(records, record)
		}
	}
	return records
}

// parseCsvLine parses a single CSV line respecting quoted fields.
func (te *ToolExecutor) parseCsvLine(line string, delimiter rune) []string {
	var fields []string
	var current strings.Builder
	inQuotes := false

	for i := 0; i < len(line); i++ {
		ch := rune(line[i])
		if inQuotes {
			if ch == '"' {
				if i+1 < len(line) && line[i+1] == '"' {
					current.WriteRune('"')
					i++
				} else {
					inQuotes = false
				}
			} else {
				current.WriteRune(ch)
			}
		} else {
			if ch == '"' {
				inQuotes = true
			} else if ch == delimiter {
				fields = append(fields, strings.TrimSpace(current.String()))
				current.Reset()
			} else {
				current.WriteRune(ch)
			}
		}
	}
	fields = append(fields, strings.TrimSpace(current.String()))
	return fields
}

// csvRead returns the CSV data with headers.
func (te *ToolExecutor) csvRead(data [][]string, sourceDesc string) *ToolResult {
	if len(data) == 0 {
		return &ToolResult{
			Success: true, Output: "No data found",
			Extra: map[string]interface{}{"tool": "csv_transformer", "action": "read", "rows": 0, "columns": 0, "source": sourceDesc},
		}
	}

	headers := data[0]
	rows := data[1:]

	var output strings.Builder
	output.WriteString(fmt.Sprintf("CSV Data (%d rows, %d columns) from %s:\n\n", len(rows), len(headers), sourceDesc))
	output.WriteString(fmt.Sprintf("Headers: %v\n\n", headers))

	maxDisplay := 20
	displayRows := rows
	if len(rows) > maxDisplay {
		displayRows = rows[:maxDisplay]
	}

	for i, row := range displayRows {
		values := make(map[string]interface{})
		for j, header := range headers {
			if j < len(row) {
				values[header] = row[j]
			} else {
				values[header] = ""
			}
		}
		rowJson, _ := json.Marshal(values)
		if i < len(displayRows)-1 {
			output.WriteString(fmt.Sprintf("Row %d: %s\n", i+1, string(rowJson)))
		} else {
			output.WriteString(fmt.Sprintf("Row %d: %s", i+1, string(rowJson)))
		}
	}
	if len(rows) > maxDisplay {
		output.WriteString(fmt.Sprintf("\n... (%d more rows omitted, total: %d)", len(rows)-maxDisplay, len(rows)))
	}

	return &ToolResult{
		Success: true, Output: output.String(),
		Extra: map[string]interface{}{"tool": "csv_transformer", "action": "read", "rows": len(rows), "columns": len(headers), "headers": headers, "source": sourceDesc},
	}
}

// csvFilter filters rows based on column conditions.
func (te *ToolExecutor) csvFilter(data [][]string, params map[string]interface{}) *ToolResult {
	if len(data) < 2 {
		return &ToolResult{
			Success: true, Output: "No data to filter",
			Extra: map[string]interface{}{"tool": "csv_transformer", "action": "filter", "rows": 0, "matched": 0},
		}
	}

	headers := data[0]
	rows := data[1:]

	conditionsParam, hasConditions := params["conditions"]
	if !hasConditions {
		return &ToolResult{Success: false, Error: "missing required parameter: conditions (array of filter objects)"}
	}

	var conditions []map[string]interface{}
	switch v := conditionsParam.(type) {
	case []interface{}:
		for _, c := range v {
			if cm, ok := c.(map[string]interface{}); ok {
				conditions = append(conditions, cm)
			}
		}
	}
	if len(conditions) == 0 {
		return &ToolResult{Success: false, Error: "conditions must be a non-empty array of filter objects"}
	}

	mode := "and"
	if m, ok := params["mode"].(string); ok {
		mode = m
	}

	var matchedRows [][]string
	for _, row := range rows {
		if te.evaluateConditions(row, headers, conditions, mode) {
			matchedRows = append(matchedRows, row)
		}
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Filtered CSV: %d/%d rows matched (%d columns)\n\n", len(matchedRows), len(rows), len(headers)))
	output.WriteString(fmt.Sprintf("Headers: %v\n\n", headers))

	maxDisplay := 20
	displayRows := matchedRows
	if len(matchedRows) > maxDisplay {
		displayRows = matchedRows[:maxDisplay]
	}

	for i, row := range displayRows {
		values := make(map[string]interface{})
		for j, header := range headers {
			if j < len(row) {
				values[header] = row[j]
			} else {
				values[header] = ""
			}
		}
		rowJson, _ := json.Marshal(values)
		if i < len(displayRows)-1 {
			output.WriteString(fmt.Sprintf("Row %d: %s\n", i+1, string(rowJson)))
		} else {
			output.WriteString(fmt.Sprintf("Row %d: %s", i+1, string(rowJson)))
		}
	}
	if len(matchedRows) > maxDisplay {
		output.WriteString(fmt.Sprintf("\n... (%d more rows omitted, total: %d)", len(matchedRows)-maxDisplay, len(matchedRows)))
	}

	return &ToolResult{
		Success: true, Output: output.String(),
		Extra: map[string]interface{}{"tool": "csv_transformer", "action": "filter", "totalRows": len(rows), "matched": len(matchedRows), "columns": len(headers), "mode": mode},
	}
}

// evaluateConditions checks if a row matches all/or conditions.
func (te *ToolExecutor) evaluateConditions(row []string, headers []string, conditions []map[string]interface{}, mode string) bool {
	var results []bool

	for _, cond := range conditions {
		col, hasCol := cond["column"].(string)
		if !hasCol || col == "" {
			continue
		}

		colIdx := -1
		for i, h := range headers {
			if h == col {
				colIdx = i
				break
			}
		}
		if colIdx == -1 || colIdx >= len(row) {
			results = append(results, false)
			continue
		}

		value := row[colIdx]
		op, _ := cond["operator"].(string)
		if op == "" {
			op = "eq"
		}
		op = strings.ToLower(op)

		filterValue, _ := cond["value"].(string)
		filterNum, _ := cond["value"].(float64)
		contains, _ := cond["contains"].(bool)

		matches := false
		switch op {
		case "eq", "equals":
			matches = value == filterValue
		case "neq", "ne", "not_equals", "not_equal":
			matches = value != filterValue
		case "gt":
			if numVal, err := strconv.ParseFloat(value, 64); err == nil {
				matches = numVal > filterNum
			}
		case "lt":
			if numVal, err := strconv.ParseFloat(value, 64); err == nil {
				matches = numVal < filterNum
			}
		case "gte", "ge":
			if numVal, err := strconv.ParseFloat(value, 64); err == nil {
				matches = numVal >= filterNum
			}
		case "lte", "le":
			if numVal, err := strconv.ParseFloat(value, 64); err == nil {
				matches = numVal <= filterNum
			}
		case "contains":
			if contains {
				matches = strings.Contains(value, filterValue)
			} else if numVal, err := strconv.ParseFloat(filterValue, 64); err == nil {
				if numValStr, err2 := strconv.ParseFloat(value, 64); err2 == nil {
					matches = numValStr == numVal
				}
			}
		case "startswith", "starts_with":
			matches = strings.HasPrefix(value, filterValue)
		case "endswith", "ends_with":
			matches = strings.HasSuffix(value, filterValue)
		case "regex", "matches":
			if re, err := regexp.Compile(filterValue); err == nil {
				matches = re.MatchString(value)
			}
		case "in":
			if arr, ok := cond["value"].([]interface{}); ok {
				for _, v := range arr {
					if fmt.Sprintf("%v", v) == value {
						matches = true
						break
					}
				}
			}
		case "not_in":
			if arr, ok := cond["value"].([]interface{}); ok {
				matches = true
				for _, v := range arr {
					if fmt.Sprintf("%v", v) == value {
						matches = false
						break
					}
				}
			}
		}
		results = append(results, matches)
	}

	if mode == "or" {
		for _, r := range results {
			if r {
				return true
			}
		}
		return false
	}
	for _, r := range results {
		if !r {
			return false
		}
	}
	return true
}

// csvSelect selects specific columns from the CSV data.
func (te *ToolExecutor) csvSelect(data [][]string, params map[string]interface{}) *ToolResult {
	if len(data) == 0 {
		return &ToolResult{
			Success: true, Output: "No data to select from",
			Extra: map[string]interface{}{"tool": "csv_transformer", "action": "select", "rows": 0},
		}
	}

	headers := data[0]
	rows := data[1:]

	columnsParam, ok := params["columns"].([]interface{})
	if !ok || len(columnsParam) == 0 {
		return &ToolResult{Success: false, Error: "missing required parameter: columns (array of column names to select)"}
	}

	var selectedCols []string
	var indices []int
	for _, c := range columnsParam {
		colName, ok := c.(string)
		if !ok {
			continue
		}
		for i, h := range headers {
			if h == colName {
				selectedCols = append(selectedCols, colName)
				indices = append(indices, i)
				break
			}
		}
	}
	if len(selectedCols) == 0 {
		return &ToolResult{Success: false, Error: fmt.Sprintf("no matching columns found: %v", columnsParam)}
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Selected %d column(s) from %d rows:\n\n", len(selectedCols), len(rows)))
	output.WriteString(fmt.Sprintf("Headers: %v\n\n", selectedCols))

	maxDisplay := 20
	displayRows := rows
	if len(rows) > maxDisplay {
		displayRows = rows[:maxDisplay]
	}

	for i, row := range displayRows {
		values := make(map[string]interface{})
		for j, idx := range indices {
			if idx < len(row) {
				values[selectedCols[j]] = row[idx]
			} else {
				values[selectedCols[j]] = ""
			}
		}
		rowJson, _ := json.Marshal(values)
		if i < len(displayRows)-1 {
			output.WriteString(fmt.Sprintf("Row %d: %s\n", i+1, string(rowJson)))
		} else {
			output.WriteString(fmt.Sprintf("Row %d: %s", i+1, string(rowJson)))
		}
	}
	if len(rows) > maxDisplay {
		output.WriteString(fmt.Sprintf("\n... (%d more rows omitted, total: %d)", len(rows)-maxDisplay, len(rows)))
	}

	return &ToolResult{
		Success: true, Output: output.String(),
		Extra: map[string]interface{}{"tool": "csv_transformer", "action": "select", "rows": len(rows), "columns": len(selectedCols), "selected": selectedCols},
	}
}

// csvSort sorts rows by a column.
func (te *ToolExecutor) csvSort(data [][]string, params map[string]interface{}) *ToolResult {
	if len(data) < 2 {
		return &ToolResult{
			Success: true, Output: "No data to sort",
			Extra: map[string]interface{}{"tool": "csv_transformer", "action": "sort", "rows": 0},
		}
	}

	headers := data[0]
	sortedRows := make([][]string, len(data)-1)
	copy(sortedRows, data[1:])

	sortCol, ok := params["column"].(string)
	if !ok || sortCol == "" {
		return &ToolResult{Success: false, Error: "missing required parameter: column (column name to sort by)"}
	}

	colIdx := -1
	for i, h := range headers {
		if h == sortCol {
			colIdx = i
			break
		}
	}
	if colIdx == -1 {
		return &ToolResult{Success: false, Error: fmt.Sprintf("column '%s' not found in headers: %v", sortCol, headers)}
	}

	direction := "asc"
	if d, ok := params["direction"].(string); ok {
		direction = strings.ToLower(d)
	}

	sort.SliceStable(sortedRows, func(i, j int) bool {
		valI, valJ := "", ""
		if colIdx < len(sortedRows[i]) {
			valI = sortedRows[i][colIdx]
		}
		if colIdx < len(sortedRows[j]) {
			valJ = sortedRows[j][colIdx]
		}

		numI, errI := strconv.ParseFloat(valI, 64)
		numJ, errJ := strconv.ParseFloat(valJ, 64)
		if errI == nil && errJ == nil {
			if direction == "asc" {
				return numI < numJ
			}
			return numI > numJ
		}
		if direction == "asc" {
			return valI < valJ
		}
		return valI > valJ
	})

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Sorted CSV by '%s' (%s) - %d rows, %d columns:\n\n", sortCol, direction, len(sortedRows), len(headers)))
	output.WriteString(fmt.Sprintf("Headers: %v\n\n", headers))

	maxDisplay := 20
	displayRows := sortedRows
	if len(sortedRows) > maxDisplay {
		displayRows = sortedRows[:maxDisplay]
	}

	for i, row := range displayRows {
		values := make(map[string]interface{})
		for j, h := range headers {
			if j < len(row) {
				values[h] = row[j]
			} else {
				values[h] = ""
			}
		}
		rowJson, _ := json.Marshal(values)
		if i < len(displayRows)-1 {
			output.WriteString(fmt.Sprintf("Row %d: %s\n", i+1, string(rowJson)))
		} else {
			output.WriteString(fmt.Sprintf("Row %d: %s", i+1, string(rowJson)))
		}
	}
	if len(sortedRows) > maxDisplay {
		output.WriteString(fmt.Sprintf("\n... (%d more rows omitted, total: %d)", len(sortedRows)-maxDisplay, len(sortedRows)))
	}

	return &ToolResult{
		Success: true, Output: output.String(),
		Extra: map[string]interface{}{"tool": "csv_transformer", "action": "sort", "column": sortCol, "direction": direction, "rows": len(sortedRows), "columns": len(headers)},
	}
}

// csvAggregate aggregates numeric data grouped by a column.
func (te *ToolExecutor) csvAggregate(data [][]string, params map[string]interface{}) *ToolResult {
	if len(data) < 2 {
		return &ToolResult{
			Success: true, Output: "No data to aggregate",
			Extra: map[string]interface{}{"tool": "csv_transformer", "action": "aggregate", "rows": 0},
		}
	}

	headers := data[0]
	rows := data[1:]

	groupCol, ok := params["group_by"].(string)
	if !ok || groupCol == "" {
		return &ToolResult{Success: false, Error: "missing required parameter: group_by (column name to group by)"}
	}

	valueCol, ok := params["value_column"].(string)
	if !ok || valueCol == "" {
		return &ToolResult{Success: false, Error: "missing required parameter: value_column (numeric column to aggregate)"}
	}

	aggFuncsParam, hasAggFuncs := params["functions"]
	_ = aggFuncsParam
	_ = hasAggFuncs

	groupIdx := -1
	valueIdx := -1
	for i, h := range headers {
		if h == groupCol {
			groupIdx = i
		}
		if h == valueCol {
			valueIdx = i
		}
	}
	if groupIdx == -1 {
		return &ToolResult{Success: false, Error: fmt.Sprintf("group_by column '%s' not found in headers", groupCol)}
	}
	if valueIdx == -1 {
		return &ToolResult{Success: false, Error: fmt.Sprintf("value_column '%s' not found in headers", valueCol)}
	}

	type groupData struct {
		values []float64
		count  int
	}
	groups := make(map[string]*groupData)
	order := []string{}

	for _, row := range rows {
		if valueIdx >= len(row) {
			continue
		}
		groupKey := row[groupIdx]
		if groups[groupKey] == nil {
			groups[groupKey] = &groupData{}
			order = append(order, groupKey)
		}
		numVal, err := strconv.ParseFloat(row[valueIdx], 64)
		if err == nil {
			groups[groupKey].values = append(groups[groupKey].values, numVal)
			groups[groupKey].count++
		}
	}

	type aggResult struct {
		Group string
		Count int
		Sum   float64
		Avg   float64
		Min   float64
		Max   float64
	}
	var results []aggResult
	var output strings.Builder

	output.WriteString(fmt.Sprintf("Aggregation by '%s' on '%s' - %d groups, %d total rows:\n\n", groupCol, valueCol, len(groups), len(rows)))

	for _, groupKey := range order {
		gd := groups[groupKey]
		if len(gd.values) == 0 {
			continue
		}
		result := aggResult{Group: groupKey, Count: gd.count}
		for _, v := range gd.values {
			result.Sum += v
			if result.Min == 0 || v < result.Min {
				result.Min = v
			}
			if v > result.Max {
				result.Max = v
			}
		}
		result.Avg = result.Sum / float64(len(gd.values))
		results = append(results, result)
	}

	for _, r := range results {
		output.WriteString(fmt.Sprintf("  %s: count=%d, sum=%.2f, avg=%.2f, min=%.2f, max=%.2f\n",
			r.Group, r.Count, r.Sum, r.Avg, r.Min, r.Max))
	}

	return &ToolResult{
		Success: true, Output: output.String(),
		Extra: map[string]interface{}{"tool": "csv_transformer", "action": "aggregate", "groupBy": groupCol, "valueColumn": valueCol, "groups": len(results), "totalRows": len(rows)},
	}
}

// csvToJson converts CSV data to JSON array.
func (te *ToolExecutor) csvToJson(data [][]string, sourceDesc string) *ToolResult {
	if len(data) == 0 {
		return &ToolResult{
			Success: true, Output: "[]",
			Extra: map[string]interface{}{"tool": "csv_transformer", "action": "to_json", "rows": 0, "source": sourceDesc},
		}
	}

	headers := data[0]
	rows := data[1:]

	jsonArray := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		obj := make(map[string]interface{})
		for j, h := range headers {
			if j < len(row) {
				obj[h] = row[j]
			} else {
				obj[h] = ""
			}
		}
		jsonArray = append(jsonArray, obj)
	}

	output, err := json.MarshalIndent(jsonArray, "", "  ")
	if err != nil {
		return &ToolResult{Success: false, Error: fmt.Sprintf("failed to format JSON: %v", err)}
	}

	return &ToolResult{
		Success: true, Output: string(output) + "\n",
		Extra: map[string]interface{}{"tool": "csv_transformer", "action": "to_json", "rows": len(rows), "columns": len(headers), "headers": headers, "source": sourceDesc},
	}
}

// csvToYaml converts CSV data to YAML format.
func (te *ToolExecutor) csvToYaml(data [][]string, sourceDesc string) *ToolResult {
	if len(data) == 0 {
		return &ToolResult{
			Success: true, Output: "[]",
			Extra: map[string]interface{}{"tool": "csv_transformer", "action": "to_yaml", "rows": 0, "source": sourceDesc},
		}
	}

	headers := data[0]
	rows := data[1:]

	var output strings.Builder
	for _, row := range rows {
		for j, h := range headers {
			if j < len(row) {
				output.WriteString(fmt.Sprintf("%s: %s\n", h, row[j]))
			}
		}
		output.WriteString("---\n")
	}

	return &ToolResult{
		Success: true, Output: output.String(),
		Extra: map[string]interface{}{"tool": "csv_transformer", "action": "to_yaml", "rows": len(rows), "columns": len(headers), "headers": headers, "source": sourceDesc},
	}
}

// csvFormat formats/aligns CSV output with configurable delimiter.
func (te *ToolExecutor) csvFormat(data [][]string, params map[string]interface{}, sourceDesc string) *ToolResult {
	if len(data) == 0 {
		return &ToolResult{
			Success: true, Output: "No data to format",
			Extra: map[string]interface{}{"tool": "csv_transformer", "action": "format", "source": sourceDesc},
		}
	}

	headers := data[0]
	rows := data[1:]

	delimiter := ','
	if delim, ok := params["delimiter"].(string); ok && delim != "" {
		delimiter = rune(delim[0])
	}

	quoting := "auto"
	if q, ok := params["quoting"].(string); ok {
		quoting = q
	}

	var output strings.Builder
	output.WriteString(te.formatCsvLine(headers, delimiter, quoting))

	for _, row := range rows {
		paddedRow := make([]string, len(headers))
		copy(paddedRow, row)
		for i := len(row); i < len(headers); i++ {
			paddedRow[i] = ""
		}
		output.WriteString(te.formatCsvLine(paddedRow, delimiter, quoting))
	}

	return &ToolResult{
		Success: true, Output: output.String(),
		Extra: map[string]interface{}{"tool": "csv_transformer", "action": "format", "rows": len(rows), "columns": len(headers), "delimiter": string(delimiter), "quoting": quoting, "source": sourceDesc},
	}
}

// formatCsvLine formats a CSV line with the given delimiter and quoting.
func (te *ToolExecutor) formatCsvLine(fields []string, delimiter rune, quoting string) string {
	var parts []string
	for _, f := range fields {
		needsQuote := false
		switch quoting {
		case "all":
			needsQuote = true
		case "nonnumeric":
			_, err := strconv.ParseFloat(f, 64)
			if err != nil && f != "" {
				needsQuote = true
			}
		case "auto":
			if strings.ContainsAny(f, string([]rune{delimiter, '"', '\n', '\r'})) {
				needsQuote = true
			}
		}
		if needsQuote {
			escaped := strings.ReplaceAll(f, `"`, `""`)
			parts = append(parts, fmt.Sprintf("\"%s\"", escaped))
		} else {
			parts = append(parts, f)
		}
	}
	return strings.Join(parts, string(delimiter)) + "\n"
}

// csvRenameColumns renames columns.
func (te *ToolExecutor) csvRenameColumns(data [][]string, params map[string]interface{}) *ToolResult {
	if len(data) == 0 {
		return &ToolResult{
			Success: true, Output: "No data to rename columns in",
			Extra: map[string]interface{}{"tool": "csv_transformer", "action": "rename_columns", "source": "csv_data"},
		}
	}

	headers := data[0]
	rows := data[1:]

	renameParam, ok := params["renames"].(map[string]interface{})
	if !ok {
		return &ToolResult{Success: false, Error: "missing required parameter: renames (object mapping old column names to new names)"}
	}
	if len(renameParam) == 0 {
		return &ToolResult{Success: false, Error: "renames must not be empty"}
	}

	renameMap := make(map[string]string)
	for k, v := range renameParam {
		if newNames, ok := v.(string); ok {
			renameMap[k] = newNames
		}
	}

	newHeaders := make([]string, len(headers))
	copy(newHeaders, headers)
	for i, h := range headers {
		if newName, exists := renameMap[h]; exists {
			newHeaders[i] = newName
		}
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Renamed columns in %d rows, %d columns:\n\n", len(rows), len(newHeaders)))
	output.WriteString(fmt.Sprintf("New headers: %v\n\n", newHeaders))

	maxDisplay := 5
	displayRows := rows
	if len(rows) > maxDisplay {
		displayRows = rows[:maxDisplay]
	}

	for i, row := range displayRows {
		values := make(map[string]interface{})
		for j, h := range newHeaders {
			if j < len(row) {
				values[h] = row[j]
			} else {
				values[h] = ""
			}
		}
		rowJson, _ := json.Marshal(values)
		if i < len(displayRows)-1 {
			output.WriteString(fmt.Sprintf("Row %d: %s\n", i+1, string(rowJson)))
		} else {
			output.WriteString(fmt.Sprintf("Row %d: %s", i+1, string(rowJson)))
		}
	}

	return &ToolResult{
		Success: true, Output: output.String(),
		Extra: map[string]interface{}{"tool": "csv_transformer", "action": "rename_columns", "rows": len(rows), "columns": len(newHeaders), "newHeaders": newHeaders, "renames": renameMap},
	}
}

// csvAddColumn adds a computed column.
func (te *ToolExecutor) csvAddColumn(data [][]string, params map[string]interface{}) *ToolResult {
	if len(data) < 2 {
		return &ToolResult{
			Success: true, Output: "No data to add column to",
			Extra: map[string]interface{}{"tool": "csv_transformer", "action": "add_column", "source": "csv_data"},
		}
	}

	headers := data[0]
	rows := data[1:]

	colName, ok := params["column_name"].(string)
	if !ok || colName == "" {
		return &ToolResult{Success: false, Error: "missing required parameter: column_name (name for the new column)"}
	}
	for _, h := range headers {
		if h == colName {
			return &ToolResult{Success: false, Error: fmt.Sprintf("column '%s' already exists", colName)}
		}
	}

	formulaParam, hasFormula := params["formula"].(string)
	sourceColsParam, hasSourceCols := params["source_columns"]
	var sourceCols []string
	if hasSourceCols {
		switch v := sourceColsParam.(type) {
		case []interface{}:
			for _, c := range v {
				if s, ok := c.(string); ok {
					sourceCols = append(sourceCols, s)
				}
			}
		case string:
			sourceCols = []string{v}
		}
	}

	if !hasFormula && !hasSourceCols {
		return &ToolResult{Success: false, Error: "must provide either formula (expression) or source_columns (for copy)"}
	}

	newHeaders := append(headers, colName)
	newValues := make([]string, len(rows))

	for i, row := range rows {
		if hasFormula {
			newValues[i] = te.evaluateFormula(formulaParam, row, headers)
		} else if hasSourceCols && len(sourceCols) == 1 {
			for j, h := range headers {
				if h == sourceCols[0] && j < len(row) {
					newValues[i] = row[j]
					break
				}
			}
		} else if hasSourceCols {
			var parts []string
			for _, sc := range sourceCols {
				for j, h := range headers {
					if h == sc && j < len(row) {
						parts = append(parts, row[j])
						break
					}
				}
			}
			newValues[i] = strings.Join(parts, " ")
		}
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Added column '%s' to %d rows (%d columns):\n\n", colName, len(rows), len(newHeaders)))
	output.WriteString(fmt.Sprintf("Headers: %v\n\n", newHeaders))

	maxDisplay := 5
	displayRows := rows
	if len(rows) > maxDisplay {
		displayRows = rows[:maxDisplay]
	}

	for i, row := range displayRows {
		values := make(map[string]interface{})
		for j, h := range newHeaders {
			if j < len(row) {
				values[h] = row[j]
			} else if h == colName {
				values[h] = newValues[i]
			} else {
				values[h] = ""
			}
		}
		rowJson, _ := json.Marshal(values)
		if i < len(displayRows)-1 {
			output.WriteString(fmt.Sprintf("Row %d: %s\n", i+1, string(rowJson)))
		} else {
			output.WriteString(fmt.Sprintf("Row %d: %s", i+1, string(rowJson)))
		}
	}

	return &ToolResult{
		Success: true, Output: output.String(),
		Extra: map[string]interface{}{"tool": "csv_transformer", "action": "add_column", "rows": len(rows), "columns": len(newHeaders), "newHeaders": newHeaders, "newColumn": colName},
	}
}

// evaluateFormula evaluates a formula expression with {{col_name}} placeholders.
func (te *ToolExecutor) evaluateFormula(formula string, row []string, headers []string) string {
	result := formula
	for i, h := range headers {
		placeholder := "{{" + h + "}}"
		value := ""
		if i < len(row) {
			value = row[i]
		}
		result = strings.ReplaceAll(result, placeholder, value)
	}

	evaluated := te.tryEvaluateMath(result)
	if evaluated != "" {
		return evaluated
	}
	return result
}

// tryEvaluateMath attempts to evaluate simple arithmetic from numeric values.
func (te *ToolExecutor) tryEvaluateMath(expr string) string {
	valid := regexp.MustCompile(`^[\d\s+\-*/().,]+$`)
	if !valid.MatchString(expr) {
		return ""
	}
	expr = strings.ReplaceAll(expr, ",", ".")
	result, err := evaluateSimpleMath(expr)
	if err != nil {
		return ""
	}
	return strconv.FormatFloat(result, 'f', -1, 64)
}

// evaluateSimpleMath evaluates a simple math expression with +, -, *, /, parentheses.
func evaluateSimpleMath(expr string) (float64, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return 0, fmt.Errorf("empty expression")
	}
	if val, err := strconv.ParseFloat(expr, 64); err == nil {
		return val, nil
	}
	return parseExpr(expr)
}

// Simple math parser using recursive descent.
func parseExpr(s string) (float64, error) {
	pos := 0
	s = strings.TrimSpace(s)

	var parseTerm func() (float64, error)
	var parseFactor func() (float64, error)

	parseFactor = func() (float64, error) {
		for pos < len(s) && s[pos] == ' ' {
			pos++
		}
		if pos >= len(s) {
			return 0, fmt.Errorf("unexpected end of expression")
		}
		if s[pos] == '(' {
			pos++
			result, err := parseExpr(s)
			if err != nil {
				return 0, err
			}
			for pos < len(s) && s[pos] == ' ' {
				pos++
			}
			if pos >= len(s) || s[pos] != ')' {
				return 0, fmt.Errorf("expected ')'")
			}
			pos++
			return result, nil
		}
		if s[pos] == '-' {
			pos++
			val, err := parseFactor()
			return -val, err
		}
		start := pos
		for pos < len(s) && (s[pos] >= '0' && s[pos] <= '9' || s[pos] == '.') {
			pos++
		}
		if start == pos {
			return 0, fmt.Errorf("unexpected character: %c", s[pos])
		}
		numStr := s[start:pos]
		return strconv.ParseFloat(numStr, 64)
	}

	parseTerm = func() (float64, error) {
		left, err := parseFactor()
		if err != nil {
			return 0, err
		}
		for pos < len(s) {
			for pos < len(s) && s[pos] == ' ' {
				pos++
			}
			if pos >= len(s) {
				break
			}
			op := s[pos]
			if op != '*' && op != '/' {
				break
			}
			pos++
			right, err := parseFactor()
			if err != nil {
				return 0, err
			}
			switch op {
			case '*':
				left *= right
			case '/':
				if right == 0 {
					return 0, fmt.Errorf("division by zero")
				}
				left /= right
			}
		}
		return left, nil
	}

	result, err := parseTerm()
	if err != nil {
		return 0, err
	}

	for pos < len(s) {
		for pos < len(s) && s[pos] == ' ' {
			pos++
		}
		if pos >= len(s) {
			break
		}
		op := s[pos]
		if op != '+' && op != '-' {
			break
		}
		pos++
		right, err := parseTerm()
		if err != nil {
			return 0, err
		}
		switch op {
		case '+':
			result += right
		case '-':
			result -= right
		}
	}

	return result, nil
}
