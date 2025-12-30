package main

import (
	"fmt"
	"log"
	"regexp"
	"strings"
)

// Pre-compiled regex patterns for better performance
var (
	tableNameRegex   = regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?[` + "`" + `"']?([a-zA-Z0-9_]+)[` + "`" + `"']?\s*\(`)
	columnBlockRegex = regexp.MustCompile(`\(([\s\S]+)\)\s*(?:ENGINE|DEFAULT|AUTO_INCREMENT|COMMENT|;|$)`)
	typeRegex        = regexp.MustCompile(`(?i)^(TINYINT|SMALLINT|MEDIUMINT|INT|INTEGER|BIGINT|FLOAT|DOUBLE|DECIMAL|NUMERIC|CHAR|VARCHAR|TEXT|TINYTEXT|MEDIUMTEXT|LONGTEXT|DATETIME|TIMESTAMP|DATE|TIME|BOOLEAN|BOOL|BLOB|TINYBLOB|MEDIUMBLOB|LONGBLOB|JSON|ENUM|SET)(?:\s*\(([^)]+)\))?(?:\s+(UNSIGNED))?`)
	notNullRegex     = regexp.MustCompile(`(?i)\bNOT\s+NULL\b`)
)

// Config controls the code generation output
type Config struct {
	AddJSONTag bool // Add json:"field_name" tags
	AddGormTag bool // Add gorm:"column:field_name" tags
	AddXMLTag  bool // Add xml:"field_name" tags
	AddDBTag   bool // Add db:"field_name" tags (for sqlx)
}

// StructDef represents the definition of a Go struct
type StructDef struct {
	Name   string     // Struct name in PascalCase
	Fields []FieldDef // List of struct fields
}

// FieldDef represents a single field in a struct
type FieldDef struct {
	Name       string // Field name in PascalCase
	Type       string // Go type (e.g., "string", "*int", "time.Time")
	ColumnName string // Original column name from SQL (snake_case)
}

// ParseSQL parses a MySQL CREATE TABLE statement and converts it to Go struct definitions
func ParseSQL(sql string) ([]StructDef, error) {
	// Clean up the SQL string - normalize whitespace
	sql = strings.TrimSpace(sql)
	sql = normalizeWhitespace(sql)

	// Extract table name using pre-compiled regex
	matches := tableNameRegex.FindStringSubmatch(sql)
	if len(matches) < 2 {
		return nil, fmt.Errorf("failed to extract table name from SQL")
	}

	tableName := matches[1]
	structName := toPascalCase(tableName)

	// Extract the column definitions (content between parentheses)
	// Use non-greedy matching to avoid capturing table options
	columnMatches := columnBlockRegex.FindStringSubmatch(sql)
	if len(columnMatches) < 2 {
		// Fallback: try simple parentheses matching
		start := strings.Index(sql, "(")
		if start == -1 {
			return nil, fmt.Errorf("failed to extract column definitions")
		}
		// Find matching closing parenthesis
		end := findMatchingParen(sql, start)
		if end == -1 {
			return nil, fmt.Errorf("failed to find closing parenthesis")
		}
		columnMatches = []string{"", sql[start+1 : end]}
	}
	if len(columnMatches) < 2 {
		return nil, fmt.Errorf("failed to extract column definitions")
	}

	columnBlock := columnMatches[1]

	// Parse individual columns
	fields, err := parseColumns(columnBlock)
	if err != nil {
		return nil, fmt.Errorf("failed to parse columns: %w", err)
	}

	structDef := StructDef{
		Name:   structName,
		Fields: fields,
	}

	return []StructDef{structDef}, nil
}

// parseColumns parses the column definitions from the SQL CREATE TABLE statement
func parseColumns(columnBlock string) ([]FieldDef, error) {
	var fields []FieldDef

	// Split by comma, but be careful of commas inside parentheses
	lines := splitColumns(columnBlock)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Skip constraint definitions (PRIMARY KEY, FOREIGN KEY, INDEX, etc.)
		if isConstraint(line) {
			continue
		}

		field, err := parseColumnDefinition(line)
		if err != nil {
			// Log warning for skipped columns
			log.Printf("Warning: skipping line (not a valid column): %s - error: %v", line, err)
			continue
		}

		fields = append(fields, field)
	}

	if len(fields) == 0 {
		return nil, fmt.Errorf("no valid columns found")
	}

	return fields, nil
}

// parseColumnDefinition parses a single column definition
func parseColumnDefinition(line string) (FieldDef, error) {
	// Remove quotes (backticks, single, double)
	line = strings.TrimSpace(line)

	// Extract column name - handle quoted identifiers
	columnName, restOfLine := extractColumnName(line)
	if columnName == "" {
		return FieldDef{}, fmt.Errorf("invalid column definition: %s", line)
	}

	// Extract data type
	dataType := extractDataType(restOfLine)
	if dataType == "" {
		return FieldDef{}, fmt.Errorf("could not extract data type from: %s", line)
	}

	// Remove COMMENT and DEFAULT sections before checking NOT NULL
	// to avoid false positives from comments containing "NOT NULL"
	checkLine := removeCommentsAndDefaults(restOfLine)

	// Check if column is nullable using word boundary regex
	isNullable := !notNullRegex.MatchString(checkLine)

	// Detect UNSIGNED attribute
	isUnsigned := strings.Contains(strings.ToUpper(restOfLine), "UNSIGNED")

	// Map SQL type to Go type
	goType := mapSQLTypeToGo(dataType, isNullable, isUnsigned)

	field := FieldDef{
		Name:       toPascalCase(columnName),
		Type:       goType,
		ColumnName: columnName, // Store original column name for tag generation
	}

	return field, nil
}

// extractDataType extracts the data type from a column definition
func extractDataType(definition string) string {
	// Use pre-compiled regex with UNSIGNED support
	matches := typeRegex.FindStringSubmatch(definition)
	if len(matches) > 0 {
		dataType := strings.ToUpper(matches[1])
		size := ""
		if len(matches) > 2 && matches[2] != "" {
			size = matches[2]
		}

		// Special case for TINYINT(1) which is typically used for boolean
		if dataType == "TINYINT" && size == "1" {
			return "TINYINT(1)"
		}

		return dataType
	}

	return ""
}

// mapSQLTypeToGo maps MySQL data types to Go types
func mapSQLTypeToGo(sqlType string, nullable bool, unsigned bool) string {
	sqlType = strings.ToUpper(sqlType)

	var baseType string

	switch sqlType {
	case "TINYINT(1)", "BOOLEAN", "BOOL":
		baseType = "bool"
	case "TINYINT":
		if unsigned {
			baseType = "uint8"
		} else {
			baseType = "int8"
		}
	case "SMALLINT":
		if unsigned {
			baseType = "uint16"
		} else {
			baseType = "int16"
		}
	case "MEDIUMINT", "INT", "INTEGER":
		if unsigned {
			baseType = "uint32"
		} else {
			baseType = "int"
		}
	case "BIGINT":
		if unsigned {
			baseType = "uint64"
		} else {
			baseType = "int64"
		}
	case "FLOAT", "DOUBLE", "DECIMAL", "NUMERIC":
		baseType = "float64"
	case "CHAR", "VARCHAR", "TEXT", "TINYTEXT", "MEDIUMTEXT", "LONGTEXT", "JSON":
		baseType = "string"
	case "DATETIME", "TIMESTAMP", "DATE", "TIME":
		baseType = "time.Time"
	case "BLOB", "TINYBLOB", "MEDIUMBLOB", "LONGBLOB":
		// []byte is already nullable (nil), so don't use pointer
		return "[]byte"
	default:
		// Default to string for unknown types
		baseType = "string"
	}

	// If nullable, use pointer type
	// Exception: []byte (slices are already nullable)
	if nullable {
		return "*" + baseType
	}

	return baseType
}

// isConstraint checks if a line is a constraint definition rather than a column
func isConstraint(line string) bool {
	upperLine := strings.ToUpper(line)
	constraintKeywords := []string{
		"PRIMARY KEY",
		"FOREIGN KEY",
		"UNIQUE KEY",
		"KEY ",
		"INDEX ",
		"CONSTRAINT",
		"CHECK ",
	}

	for _, keyword := range constraintKeywords {
		if strings.HasPrefix(upperLine, keyword) {
			return true
		}
	}

	return false
}

// splitColumns splits the column block by commas, respecting parentheses
func splitColumns(columnBlock string) []string {
	var result []string
	var current strings.Builder
	parenCount := 0

	for _, char := range columnBlock {
		switch char {
		case '(':
			parenCount++
			current.WriteRune(char)
		case ')':
			parenCount--
			current.WriteRune(char)
		case ',':
			if parenCount == 0 {
				result = append(result, current.String())
				current.Reset()
			} else {
				current.WriteRune(char)
			}
		default:
			current.WriteRune(char)
		}
	}

	// Add the last part
	if current.Len() > 0 {
		result = append(result, current.String())
	}

	return result
}

// toPascalCase converts a snake_case string to PascalCase
func toPascalCase(s string) string {
	// Split by underscore
	parts := strings.Split(s, "_")

	var result strings.Builder
	for _, part := range parts {
		if len(part) > 0 {
			// Capitalize first letter, lowercase the rest
			result.WriteString(strings.ToUpper(part[:1]))
			if len(part) > 1 {
				result.WriteString(strings.ToLower(part[1:]))
			}
		}
	}

	return result.String()
}

// normalizeWhitespace replaces multiple spaces/tabs/newlines with single space
func normalizeWhitespace(s string) string {
	// Replace multiple whitespace with single space
	space := regexp.MustCompile(`\s+`)
	return space.ReplaceAllString(s, " ")
}

// findMatchingParen finds the matching closing parenthesis
func findMatchingParen(s string, start int) int {
	count := 1
	for i := start + 1; i < len(s); i++ {
		if s[i] == '(' {
			count++
		} else if s[i] == ')' {
			count--
			if count == 0 {
				return i
			}
		}
	}
	return -1
}

// extractColumnName extracts column name from line, handling quoted identifiers
func extractColumnName(line string) (string, string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", ""
	}

	// Check if starts with backtick
	if line[0] == '`' {
		end := strings.Index(line[1:], "`")
		if end == -1 {
			return "", ""
		}
		columnName := line[1 : end+1]
		rest := strings.TrimSpace(line[end+2:])
		return columnName, rest
	}

	// Check if starts with double quote
	if line[0] == '"' {
		end := strings.Index(line[1:], `"`)
		if end == -1 {
			return "", ""
		}
		columnName := line[1 : end+1]
		rest := strings.TrimSpace(line[end+2:])
		return columnName, rest
	}

	// Otherwise, extract first word
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return "", ""
	}
	return parts[0], strings.Join(parts[1:], " ")
}

// removeCommentsAndDefaults removes COMMENT and DEFAULT clauses from column definition
// to prevent false positives when checking for NOT NULL
func removeCommentsAndDefaults(line string) string {
	upperLine := strings.ToUpper(line)

	// Remove everything after COMMENT
	if idx := strings.Index(upperLine, "COMMENT"); idx != -1 {
		line = line[:idx]
	}

	// Remove DEFAULT clause (but be careful with nested strings)
	if idx := strings.Index(upperLine, "DEFAULT"); idx != -1 {
		// Find the end of DEFAULT value
		// Simple approach: remove from DEFAULT to next keyword or end
		rest := line[idx+7:] // Skip "DEFAULT"
		rest = strings.TrimSpace(rest)

		// If starts with quote, find matching quote
		if len(rest) > 0 && (rest[0] == '\'' || rest[0] == '"') {
			quote := rest[0]
			endIdx := strings.Index(rest[1:], string(quote))
			if endIdx != -1 {
				// Remove DEFAULT and its value
				line = line[:idx] + rest[endIdx+2:]
			} else {
				// No closing quote, just remove DEFAULT to end
				line = line[:idx]
			}
		} else {
			// No quote, remove DEFAULT and first word
			words := strings.Fields(rest)
			if len(words) > 0 {
				line = line[:idx]
			}
		}
	}

	return strings.TrimSpace(line)
}

// GenerateGoCode generates formatted Go source code from struct definitions
func GenerateGoCode(defs []StructDef, config Config) string {
	if len(defs) == 0 {
		return ""
	}

	var output strings.Builder

	// Determine if we need time import
	needsTime := needsTimeImport(defs)

	// Generate package and imports
	output.WriteString("package main\n\n")
	if needsTime {
		output.WriteString("import \"time\"\n\n")
	}

	// Generate each struct
	for i, def := range defs {
		if i > 0 {
			output.WriteString("\n")
		}
		output.WriteString(generateStruct(def, config))
	}

	return output.String()
}

// generateStruct generates a single struct with proper field alignment
func generateStruct(def StructDef, config Config) string {
	var output strings.Builder

	output.WriteString(fmt.Sprintf("type %s struct {\n", def.Name))

	if len(def.Fields) == 0 {
		output.WriteString("}\n")
		return output.String()
	}

	// Calculate alignment widths
	maxNameLen, maxTypeLen := calculateAlignment(def.Fields)

	// Generate fields
	for _, field := range def.Fields {
		// Field name (aligned)
		output.WriteString("\t")
		output.WriteString(field.Name)
		output.WriteString(strings.Repeat(" ", maxNameLen-len(field.Name)+1))

		// Field type (aligned)
		output.WriteString(field.Type)

		// Generate tags if configured
		tags := generateStructTags(field.ColumnName, config)
		if tags != "" {
			output.WriteString(strings.Repeat(" ", maxTypeLen-len(field.Type)+1))
			output.WriteString("`")
			output.WriteString(tags)
			output.WriteString("`")
		}

		output.WriteString("\n")
	}

	output.WriteString("}\n")
	return output.String()
}

// calculateAlignment calculates the maximum field name and type lengths for alignment
func calculateAlignment(fields []FieldDef) (maxNameLen, maxTypeLen int) {
	for _, field := range fields {
		if len(field.Name) > maxNameLen {
			maxNameLen = len(field.Name)
		}
		if len(field.Type) > maxTypeLen {
			maxTypeLen = len(field.Type)
		}
	}
	return maxNameLen, maxTypeLen
}

// generateStructTags generates struct tags based on config
func generateStructTags(columnName string, config Config) string {
	var tags []string

	// Normalize column name to lowercase snake_case for tags (industry standard)
	normalizedName := toSnakeCase(columnName)

	if config.AddJSONTag {
		tags = append(tags, fmt.Sprintf(`json:"%s"`, normalizedName))
	}

	if config.AddDBTag {
		tags = append(tags, fmt.Sprintf(`db:"%s"`, normalizedName))
	}

	if config.AddGormTag {
		tags = append(tags, fmt.Sprintf(`gorm:"column:%s"`, normalizedName))
	}

	if config.AddXMLTag {
		tags = append(tags, fmt.Sprintf(`xml:"%s"`, normalizedName))
	}

	return strings.Join(tags, " ")
}

// toSnakeCase converts a string to lowercase snake_case
// Handles: PascalCase, camelCase, SCREAMING_CASE, or already snake_case
func toSnakeCase(s string) string {
	if s == "" {
		return s
	}

	// If already all lowercase with underscores, return as-is
	if isLowerSnakeCase(s) {
		return s
	}

	var result strings.Builder
	for i, r := range s {
		// If uppercase letter
		if r >= 'A' && r <= 'Z' {
			// Add underscore before uppercase if:
			// - Not the first character
			// - Previous char is not underscore
			// - Previous char is lowercase or next char is lowercase (camelCase boundary)
			if i > 0 && s[i-1] != '_' {
				// Check if this is a camelCase boundary
				prevIsLower := s[i-1] >= 'a' && s[i-1] <= 'z'
				nextIsLower := i+1 < len(s) && s[i+1] >= 'a' && s[i+1] <= 'z'

				if prevIsLower || nextIsLower {
					result.WriteRune('_')
				}
			}
			result.WriteRune(r + 32) // Convert to lowercase
		} else {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// isLowerSnakeCase checks if string is already lowercase snake_case
func isLowerSnakeCase(s string) bool {
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			return false
		}
	}
	return true
}

// needsTimeImport checks if any field uses time.Time
func needsTimeImport(defs []StructDef) bool {
	for _, def := range defs {
		for _, field := range def.Fields {
			if strings.Contains(field.Type, "time.Time") {
				return true
			}
		}
	}
	return false
}
