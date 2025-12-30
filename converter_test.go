package main

import (
	"strings"
	"testing"
)

// TestParseSQL_BasicMySQL tests basic MySQL CREATE TABLE
func TestParseSQL_BasicMySQL(t *testing.T) {
	sql := `CREATE TABLE users (
		id INT NOT NULL,
		name VARCHAR(255) NOT NULL,
		email VARCHAR(255),
		created_at DATETIME NOT NULL
	)`

	structs, err := ParseSQL(sql)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(structs) != 1 {
		t.Fatalf("Expected 1 struct, got %d", len(structs))
	}

	s := structs[0]
	if s.Name != "Users" {
		t.Errorf("Expected struct name 'Users', got '%s'", s.Name)
	}

	if len(s.Fields) != 4 {
		t.Fatalf("Expected 4 fields, got %d", len(s.Fields))
	}

	// Check field types
	expectedFields := map[string]string{
		"Id":        "int",
		"Name":      "string",
		"Email":     "*string", // nullable
		"CreatedAt": "time.Time",
	}

	for _, field := range s.Fields {
		expectedType, ok := expectedFields[field.Name]
		if !ok {
			t.Errorf("Unexpected field: %s", field.Name)
			continue
		}
		if field.Type != expectedType {
			t.Errorf("Field %s: expected type '%s', got '%s'", field.Name, expectedType, field.Type)
		}
	}

	// Check column names are stored
	for _, field := range s.Fields {
		if field.ColumnName == "" {
			t.Errorf("Field %s missing ColumnName", field.Name)
		}
	}
}

// TestParseSQL_CodeSmell1_NullableLogic tests that nullable detection works correctly
// Should NOT be confused by "NOT NULL" in comments or strings
func TestParseSQL_CodeSmell1_NullableLogic(t *testing.T) {
	sql := `CREATE TABLE test_table (
		id INT NOT NULL,
		nullable_field VARCHAR(100),
		tricky_field VARCHAR(100) DEFAULT NULL COMMENT 'This is NOT NULL in production',
		another_tricky TEXT COMMENT 'NOT NULL check'
	)`

	structs, err := ParseSQL(sql)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	s := structs[0]

	// Check nullable detection
	for _, field := range s.Fields {
		switch field.Name {
		case "Id":
			if field.Type != "int" {
				t.Errorf("Id should be non-nullable int, got: %s", field.Type)
			}
		case "NullableField":
			if !strings.HasPrefix(field.Type, "*") {
				t.Errorf("NullableField should be nullable (pointer), got: %s", field.Type)
			}
		case "TrickyField":
			// Should be nullable despite "NOT NULL" in comment
			if !strings.HasPrefix(field.Type, "*") {
				t.Errorf("TrickyField should be nullable despite comment, got: %s", field.Type)
			}
		case "AnotherTricky":
			if !strings.HasPrefix(field.Type, "*") {
				t.Errorf("AnotherTricky should be nullable, got: %s", field.Type)
			}
		}
	}
}

// TestParseSQL_CodeSmell2_GreedyRegex tests that regex doesn't capture table options
func TestParseSQL_CodeSmell2_GreedyRegex(t *testing.T) {
	sql := `CREATE TABLE products (
		id INT NOT NULL,
		name VARCHAR(255) NOT NULL
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`

	structs, err := ParseSQL(sql)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	s := structs[0]
	if len(s.Fields) != 2 {
		t.Errorf("Expected 2 fields (should not capture ENGINE options), got %d", len(s.Fields))
	}
}

// TestParseSQL_CodeSmell3_MultipleSpaces tests whitespace normalization
func TestParseSQL_CodeSmell3_MultipleSpaces(t *testing.T) {
	sql := `CREATE    TABLE     users    (
		id      INT      NOT     NULL,
		name    VARCHAR(255)    NOT    NULL
	)`

	structs, err := ParseSQL(sql)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(structs) != 1 {
		t.Fatalf("Expected 1 struct, got %d", len(structs))
	}

	s := structs[0]
	if len(s.Fields) != 2 {
		t.Errorf("Expected 2 fields with multiple spaces, got %d", len(s.Fields))
	}
}

// TestParseSQL_CodeSmell4_QuotedIdentifiers tests backticks and double quotes
func TestParseSQL_CodeSmell4_QuotedIdentifiers(t *testing.T) {
	sql := "CREATE TABLE `users` (\n" +
		"\t`user_id` INT NOT NULL,\n" +
		"\t`user_name` VARCHAR(255) NOT NULL\n" +
		")"

	structs, err := ParseSQL(sql)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	s := structs[0]
	if s.Name != "Users" {
		t.Errorf("Expected struct name 'Users', got '%s'", s.Name)
	}

	// Check fields were extracted correctly
	if len(s.Fields) != 2 {
		t.Fatalf("Expected 2 fields, got %d", len(s.Fields))
	}

	if s.Fields[0].Name != "UserId" {
		t.Errorf("Expected field name 'UserId', got '%s'", s.Fields[0].Name)
	}
}

// TestParseSQL_CodeSmell5_BlobHandling tests that BLOB types don't use pointers when nullable
func TestParseSQL_CodeSmell5_BlobHandling(t *testing.T) {
	sql := `CREATE TABLE files (
		id INT NOT NULL,
		content BLOB,
		thumbnail TINYBLOB
	)`

	structs, err := ParseSQL(sql)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	s := structs[0]

	for _, field := range s.Fields {
		if field.Name == "Content" || field.Name == "Thumbnail" {
			// BLOB should be []byte, not *[]byte even if nullable
			if field.Type != "[]byte" {
				t.Errorf("Field %s should be []byte (not pointer), got: %s", field.Name, field.Type)
			}
		}
	}
}

// TestParseSQL_CodeSmell6_UnsignedSupport tests UNSIGNED integer handling
func TestParseSQL_CodeSmell6_UnsignedSupport(t *testing.T) {
	sql := `CREATE TABLE counters (
		tiny_unsigned TINYINT UNSIGNED,
		small_unsigned SMALLINT UNSIGNED,
		int_unsigned INT UNSIGNED,
		big_unsigned BIGINT UNSIGNED,
		normal_int INT NOT NULL
	)`

	structs, err := ParseSQL(sql)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	s := structs[0]

	expectedTypes := map[string]string{
		"TinyUnsigned":  "*uint8",
		"SmallUnsigned": "*uint16",
		"IntUnsigned":   "*uint32",
		"BigUnsigned":   "*uint64",
		"NormalInt":     "int",
	}

	for _, field := range s.Fields {
		expectedType, ok := expectedTypes[field.Name]
		if !ok {
			t.Errorf("Unexpected field: %s", field.Name)
			continue
		}
		if field.Type != expectedType {
			t.Errorf("Field %s: expected type '%s', got '%s'", field.Name, expectedType, field.Type)
		}
	}
}

// TestParseSQL_PostgreSQL tests PostgreSQL CREATE TABLE syntax
func TestParseSQL_PostgreSQL(t *testing.T) {
	sql := `CREATE TABLE IF NOT EXISTS users (
		id INTEGER NOT NULL,
		username VARCHAR(100) NOT NULL,
		email TEXT,
		created_at TIMESTAMP NOT NULL
	)`

	structs, err := ParseSQL(sql)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	s := structs[0]
	if s.Name != "Users" {
		t.Errorf("Expected struct name 'Users', got '%s'", s.Name)
	}

	if len(s.Fields) != 4 {
		t.Errorf("Expected 4 fields, got %d", len(s.Fields))
	}
}

// TestParseSQL_SQLite tests SQLite CREATE TABLE syntax
func TestParseSQL_SQLite(t *testing.T) {
	sql := `CREATE TABLE products (
		id INTEGER NOT NULL,
		name TEXT NOT NULL,
		price DECIMAL(10, 2) NOT NULL,
		in_stock BOOLEAN
	);`

	structs, err := ParseSQL(sql)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	s := structs[0]

	if len(s.Fields) != 4 {
		t.Errorf("Expected 4 fields, got %d", len(s.Fields))
	}

	// Check types
	for _, field := range s.Fields {
		switch field.Name {
		case "Price":
			if field.Type != "float64" {
				t.Errorf("Price should be float64, got: %s", field.Type)
			}
		case "InStock":
			if field.Type != "*bool" {
				t.Errorf("InStock should be *bool (nullable), got: %s", field.Type)
			}
		}
	}
}

// TestParseSQL_AllDataTypes tests comprehensive type mapping
func TestParseSQL_AllDataTypes(t *testing.T) {
	sql := `CREATE TABLE all_types (
		col_tinyint TINYINT NOT NULL,
		col_smallint SMALLINT NOT NULL,
		col_mediumint MEDIUMINT NOT NULL,
		col_int INT NOT NULL,
		col_bigint BIGINT NOT NULL,
		col_float FLOAT NOT NULL,
		col_double DOUBLE NOT NULL,
		col_decimal DECIMAL(10,2) NOT NULL,
		col_char CHAR(10) NOT NULL,
		col_varchar VARCHAR(255) NOT NULL,
		col_text TEXT NOT NULL,
		col_datetime DATETIME NOT NULL,
		col_timestamp TIMESTAMP NOT NULL,
		col_date DATE NOT NULL,
		col_time TIME NOT NULL,
		col_boolean BOOLEAN NOT NULL,
		col_json JSON NOT NULL,
		col_blob BLOB,
		col_tinyint_bool TINYINT(1) NOT NULL
	)`

	structs, err := ParseSQL(sql)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	s := structs[0]

	expectedTypes := map[string]string{
		"ColTinyint":     "int8",
		"ColSmallint":    "int16",
		"ColMediumint":   "int",
		"ColInt":         "int",
		"ColBigint":      "int64",
		"ColFloat":       "float64",
		"ColDouble":      "float64",
		"ColDecimal":     "float64",
		"ColChar":        "string",
		"ColVarchar":     "string",
		"ColText":        "string",
		"ColDatetime":    "time.Time",
		"ColTimestamp":   "time.Time",
		"ColDate":        "time.Time",
		"ColTime":        "time.Time",
		"ColBoolean":     "bool",
		"ColJson":        "string",
		"ColBlob":        "[]byte",
		"ColTinyintBool": "bool",
	}

	if len(s.Fields) != len(expectedTypes) {
		t.Errorf("Expected %d fields, got %d", len(expectedTypes), len(s.Fields))
	}

	for _, field := range s.Fields {
		expectedType, ok := expectedTypes[field.Name]
		if !ok {
			t.Errorf("Unexpected field: %s", field.Name)
			continue
		}
		if field.Type != expectedType {
			t.Errorf("Field %s: expected type '%s', got '%s'", field.Name, expectedType, field.Type)
		}
	}
}

// TestParseSQL_ConstraintsSkipped tests that constraints are properly skipped
func TestParseSQL_ConstraintsSkipped(t *testing.T) {
	sql := `CREATE TABLE orders (
		id INT NOT NULL,
		user_id INT NOT NULL,
		total DECIMAL(10,2) NOT NULL,
		PRIMARY KEY (id),
		FOREIGN KEY (user_id) REFERENCES users(id),
		INDEX idx_user_id (user_id),
		UNIQUE KEY unique_order (id, user_id),
		CONSTRAINT chk_total CHECK (total >= 0)
	)`

	structs, err := ParseSQL(sql)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	s := structs[0]

	// Should only have 3 fields, not the constraints
	if len(s.Fields) != 3 {
		t.Errorf("Expected 3 fields (constraints should be skipped), got %d", len(s.Fields))
		for i, field := range s.Fields {
			t.Logf("Field %d: %s (%s)", i, field.Name, field.Type)
		}
	}
}

// TestParseSQL_ComplexRealWorld tests a complex real-world table
func TestParseSQL_ComplexRealWorld(t *testing.T) {
	sql := `CREATE TABLE IF NOT EXISTS user_profiles (
		user_id BIGINT UNSIGNED NOT NULL,
		username VARCHAR(50) NOT NULL,
		email VARCHAR(255) NOT NULL,
		full_name VARCHAR(200),
		bio TEXT,
		avatar_url VARCHAR(500),
		birth_date DATE,
		is_verified TINYINT(1) NOT NULL DEFAULT 0,
		follower_count INT UNSIGNED DEFAULT 0,
		following_count INT UNSIGNED DEFAULT 0,
		post_count MEDIUMINT UNSIGNED DEFAULT 0,
		rating DECIMAL(3,2),
		last_login_at TIMESTAMP,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		PRIMARY KEY (user_id),
		UNIQUE KEY unique_username (username),
		UNIQUE KEY unique_email (email),
		INDEX idx_verified (is_verified),
		INDEX idx_created (created_at)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`

	structs, err := ParseSQL(sql)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	s := structs[0]

	if s.Name != "UserProfiles" {
		t.Errorf("Expected struct name 'UserProfiles', got '%s'", s.Name)
	}

	// Should have 15 fields (not constraints)
	if len(s.Fields) != 15 {
		t.Errorf("Expected 15 fields, got %d", len(s.Fields))
	}

	// Spot check key fields
	fieldMap := make(map[string]FieldDef)
	for _, field := range s.Fields {
		fieldMap[field.Name] = field
	}

	// Check specific fields
	tests := []struct {
		name         string
		expectedType string
	}{
		{"UserId", "uint64"},
		{"Username", "string"},
		{"Email", "string"},
		{"FullName", "*string"},       // nullable
		{"Bio", "*string"},            // nullable
		{"IsVerified", "bool"},        // TINYINT(1)
		{"FollowerCount", "*uint32"},  // nullable UNSIGNED
		{"Rating", "*float64"},        // nullable DECIMAL
		{"LastLoginAt", "*time.Time"}, // nullable TIMESTAMP
		{"CreatedAt", "time.Time"},    // NOT NULL
	}

	for _, tt := range tests {
		field, ok := fieldMap[tt.name]
		if !ok {
			t.Errorf("Field '%s' not found", tt.name)
			continue
		}
		if field.Type != tt.expectedType {
			t.Errorf("Field %s: expected type '%s', got '%s'", tt.name, tt.expectedType, field.Type)
		}
	}
}

// TestParseSQL_MixedCase tests mixed case table and column names
func TestParseSQL_MixedCase(t *testing.T) {
	sql := `CREATE TABLE UserProfiles (
		UserID INT NOT NULL,
		FirstName VARCHAR(100) NOT NULL,
		last_name VARCHAR(100) NOT NULL
	)`

	structs, err := ParseSQL(sql)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	s := structs[0]

	// Check PascalCase conversion
	if s.Name != "Userprofiles" {
		t.Errorf("Expected struct name 'Userprofiles', got '%s'", s.Name)
	}
}

// TestParseSQL_NewlinesAndTabs tests various whitespace characters
func TestParseSQL_NewlinesAndTabs(t *testing.T) {
	sql := "CREATE TABLE\tusers\t(\n\tid\tINT\tNOT\tNULL,\n\tname\tVARCHAR(255)\tNOT\tNULL\n)"

	structs, err := ParseSQL(sql)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	s := structs[0]
	if len(s.Fields) != 2 {
		t.Errorf("Expected 2 fields with tabs/newlines, got %d", len(s.Fields))
	}
}

// TestParseSQL_ErrorCases tests various error scenarios
func TestParseSQL_ErrorCases(t *testing.T) {
	tests := []struct {
		name string
		sql  string
	}{
		{"Empty SQL", ""},
		{"No table name", "CREATE TABLE ()"},
		{"No parentheses", "CREATE TABLE users"},
		{"Invalid syntax", "INVALID SQL STATEMENT"},
		{"No columns", "CREATE TABLE users ()"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSQL(tt.sql)
			if err == nil {
				t.Errorf("Expected error for %s, got nil", tt.name)
			}
		})
	}
}

// TestParseSQL_ColumnNames tests that column names are properly stored
func TestParseSQL_ColumnNames(t *testing.T) {
	sql := `CREATE TABLE users (
		user_id INT NOT NULL,
		full_name VARCHAR(255) NOT NULL
	)`

	structs, err := ParseSQL(sql)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	s := structs[0]

	// Check that column names are stored
	expectedColumnNames := map[string]string{
		"UserId":   "user_id",
		"FullName": "full_name",
	}

	for _, field := range s.Fields {
		expectedColName, ok := expectedColumnNames[field.Name]
		if !ok {
			t.Errorf("Unexpected field: %s", field.Name)
			continue
		}
		if field.ColumnName != expectedColName {
			t.Errorf("Field %s: expected column name '%s', got '%s'", field.Name, expectedColName, field.ColumnName)
		}
	}
}

// TestParseSQL_MariaDB tests MariaDB specific syntax
func TestParseSQL_MariaDB(t *testing.T) {
	sql := `CREATE TABLE sessions (
		session_id CHAR(128) NOT NULL,
		user_id BIGINT UNSIGNED,
		data MEDIUMTEXT,
		last_activity TIMESTAMP NOT NULL,
		PRIMARY KEY (session_id)
	) ENGINE=InnoDB;`

	structs, err := ParseSQL(sql)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	s := structs[0]
	if len(s.Fields) != 4 {
		t.Errorf("Expected 4 fields, got %d", len(s.Fields))
	}
}

// TestParseSQL_EnumAndSet tests ENUM and SET types
func TestParseSQL_EnumAndSet(t *testing.T) {
	sql := `CREATE TABLE products (
		id INT NOT NULL,
		status ENUM('active', 'inactive', 'pending') NOT NULL,
		tags SET('featured', 'sale', 'new') NOT NULL
	)`

	structs, err := ParseSQL(sql)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	s := structs[0]

	// ENUM and SET should default to string
	for _, field := range s.Fields {
		if field.Name == "Status" || field.Name == "Tags" {
			if field.Type != "string" {
				t.Errorf("Field %s should be string, got: %s", field.Name, field.Type)
			}
		}
	}
}

// TestPascalCase tests the toPascalCase function
func TestPascalCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"user_id", "UserId"},
		{"first_name", "FirstName"},
		{"USER_PROFILE", "UserProfile"},
		{"a_b_c_d", "ABCD"},
		{"single", "Single"},
		{"_leading_underscore", "LeadingUnderscore"},
		{"trailing_underscore_", "TrailingUnderscore"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toPascalCase(tt.input)
			if result != tt.expected {
				t.Errorf("toPascalCase(%s): expected '%s', got '%s'", tt.input, tt.expected, result)
			}
		})
	}
}

// TestGenerateGoCode_NoTags tests code generation without any tags
func TestGenerateGoCode_NoTags(t *testing.T) {
	sql := `CREATE TABLE users (
		id INT NOT NULL,
		name VARCHAR(255) NOT NULL
	)`

	structs, err := ParseSQL(sql)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	config := Config{
		AddJSONTag: false,
		AddGormTag: false,
		AddXMLTag:  false,
		AddDBTag:   false,
	}

	code := GenerateGoCode(structs, config)

	// Check basic structure
	if !strings.Contains(code, "package main") {
		t.Error("Generated code should contain package declaration")
	}

	if !strings.Contains(code, "type Users struct {") {
		t.Error("Generated code should contain struct declaration")
	}

	if !strings.Contains(code, "Id") {
		t.Error("Generated code should contain Id field")
	}

	if !strings.Contains(code, "Name") {
		t.Error("Generated code should contain Name field")
	}

	// Should NOT contain tags
	if strings.Contains(code, "json:") || strings.Contains(code, "gorm:") {
		t.Error("Generated code should not contain tags when config disables them")
	}

	t.Logf("Generated code:\n%s", code)
}

// TestGenerateGoCode_JSONTags tests code generation with JSON tags only
func TestGenerateGoCode_JSONTags(t *testing.T) {
	sql := `CREATE TABLE users (
		id INT NOT NULL,
		user_name VARCHAR(255) NOT NULL,
		email VARCHAR(255)
	)`

	structs, err := ParseSQL(sql)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	config := Config{
		AddJSONTag: true,
		AddGormTag: false,
		AddXMLTag:  false,
		AddDBTag:   false,
	}

	code := GenerateGoCode(structs, config)

	// Check JSON tags are present
	if !strings.Contains(code, `json:"id"`) {
		t.Error("Generated code should contain json:\"id\" tag")
	}

	if !strings.Contains(code, `json:"user_name"`) {
		t.Error("Generated code should contain json:\"user_name\" tag")
	}

	// Should NOT contain other tags
	if strings.Contains(code, "gorm:") {
		t.Error("Generated code should not contain gorm tags")
	}

	t.Logf("Generated code:\n%s", code)
}

// TestGenerateGoCode_AllTags tests code generation with all tag types
func TestGenerateGoCode_AllTags(t *testing.T) {
	sql := `CREATE TABLE user_profiles (
		user_id BIGINT NOT NULL,
		full_name VARCHAR(200) NOT NULL,
		created_at DATETIME NOT NULL
	)`

	structs, err := ParseSQL(sql)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	config := Config{
		AddJSONTag: true,
		AddGormTag: true,
		AddXMLTag:  true,
		AddDBTag:   true,
	}

	code := GenerateGoCode(structs, config)

	// Check all tag types are present
	if !strings.Contains(code, `json:"user_id"`) {
		t.Error("Should contain JSON tags")
	}

	if !strings.Contains(code, `gorm:"column:user_id"`) {
		t.Error("Should contain GORM tags")
	}

	if !strings.Contains(code, `xml:"user_id"`) {
		t.Error("Should contain XML tags")
	}

	if !strings.Contains(code, `db:"user_id"`) {
		t.Error("Should contain DB tags")
	}

	t.Logf("Generated code:\n%s", code)
}

// TestGenerateGoCode_TimeImport tests that time import is added when needed
func TestGenerateGoCode_TimeImport(t *testing.T) {
	sql := `CREATE TABLE events (
		id INT NOT NULL,
		created_at DATETIME NOT NULL,
		updated_at TIMESTAMP
	)`

	structs, err := ParseSQL(sql)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	config := Config{AddJSONTag: true}
	code := GenerateGoCode(structs, config)

	// Should contain time import
	if !strings.Contains(code, `import "time"`) {
		t.Error("Generated code should contain time import when using time.Time")
	}

	t.Logf("Generated code:\n%s", code)
}

// TestGenerateGoCode_NoTimeImport tests that time import is NOT added when not needed
func TestGenerateGoCode_NoTimeImport(t *testing.T) {
	sql := `CREATE TABLE simple (
		id INT NOT NULL,
		name VARCHAR(255) NOT NULL
	)`

	structs, err := ParseSQL(sql)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	config := Config{AddJSONTag: true}
	code := GenerateGoCode(structs, config)

	// Should NOT contain time import
	if strings.Contains(code, `import "time"`) {
		t.Error("Generated code should not contain time import when not using time.Time")
	}

	t.Logf("Generated code:\n%s", code)
}

// TestGenerateGoCode_Alignment tests that fields and tags are properly aligned
func TestGenerateGoCode_Alignment(t *testing.T) {
	sql := `CREATE TABLE test (
		id INT NOT NULL,
		very_long_field_name VARCHAR(255) NOT NULL,
		x INT NOT NULL
	)`

	structs, err := ParseSQL(sql)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	config := Config{
		AddJSONTag: true,
		AddGormTag: true,
	}

	code := GenerateGoCode(structs, config)

	// Visual inspection - log for manual verification
	t.Logf("Generated code (check alignment visually):\n%s", code)

	// Check that fields are present
	if !strings.Contains(code, "Id") {
		t.Error("Should contain Id field")
	}
	if !strings.Contains(code, "VeryLongFieldName") {
		t.Error("Should contain VeryLongFieldName field")
	}
	if !strings.Contains(code, "X") {
		t.Error("Should contain X field")
	}

	// Basic structure check - should have consistent spacing
	lines := strings.Split(code, "\n")
	var structLines []string
	inStruct := false
	for _, line := range lines {
		if strings.Contains(line, "type Test struct {") {
			inStruct = true
			continue
		}
		if strings.Contains(line, "}") && inStruct {
			break
		}
		if inStruct && strings.TrimSpace(line) != "" {
			structLines = append(structLines, line)
		}
	}

	// All struct field lines should start with tab
	for _, line := range structLines {
		if !strings.HasPrefix(line, "\t") {
			t.Errorf("Struct field line should start with tab: %s", line)
		}
	}
}

// TestGenerateGoCode_ComplexRealWorld tests a complex real-world scenario
func TestGenerateGoCode_ComplexRealWorld(t *testing.T) {
	sql := `CREATE TABLE user_profiles (
		user_id BIGINT UNSIGNED NOT NULL,
		username VARCHAR(50) NOT NULL,
		email VARCHAR(255) NOT NULL,
		full_name VARCHAR(200),
		bio TEXT,
		is_verified TINYINT(1) NOT NULL DEFAULT 0,
		follower_count INT UNSIGNED DEFAULT 0,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	)`

	structs, err := ParseSQL(sql)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	config := Config{
		AddJSONTag: true,
		AddGormTag: true,
		AddDBTag:   true,
	}

	code := GenerateGoCode(structs, config)

	// Check structure
	if !strings.Contains(code, "type UserProfiles struct {") {
		t.Error("Should contain UserProfiles struct")
	}

	// Check imports
	if !strings.Contains(code, `import "time"`) {
		t.Error("Should contain time import")
	}

	// Check fields exist
	expectedFields := []string{"UserId", "Username", "Email", "FullName", "Bio", "IsVerified", "FollowerCount", "CreatedAt", "UpdatedAt"}
	for _, field := range expectedFields {
		if !strings.Contains(code, field) {
			t.Errorf("Should contain field: %s", field)
		}
	}

	// Check tags
	if !strings.Contains(code, `json:"user_id"`) {
		t.Error("Should contain JSON tags with original column names")
	}

	if !strings.Contains(code, `gorm:"column:user_id"`) {
		t.Error("Should contain GORM tags")
	}

	t.Logf("Generated code:\n%s", code)
}

// TestGenerateGoCode_EmptyInput tests handling of empty input
func TestGenerateGoCode_EmptyInput(t *testing.T) {
	var structs []StructDef
	config := Config{AddJSONTag: true}

	code := GenerateGoCode(structs, config)

	if code != "" {
		t.Errorf("Expected empty string for empty input, got: %s", code)
	}
}
