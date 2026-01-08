package tools

import (
	"strings"
	"testing"
)

func TestRedactor_AWSAccessKeys(t *testing.T) {
	r := NewRedactor()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "AWS access key in plain text",
			input:    "AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE",
			expected: "AWS_ACCESS_KEY_ID=[REDACTED]",
		},
		{
			name:     "AWS access key in log output",
			input:    "Using credentials AKIAIOSFODNN7EXAMPLE for deployment",
			expected: "Using credentials [REDACTED] for deployment",
		},
		{
			name:     "Multiple AWS access keys",
			input:    "Key1: AKIAIOSFODNN7EXAMPLE, Key2: AKIAJ7EXAMPLE1234567",
			expected: "Key1: [REDACTED], Key2: [REDACTED]",
		},
		{
			name:     "Not an AWS key (too short)",
			input:    "AKIASHORT is not a key",
			expected: "AKIASHORT is not a key",
		},
		{
			name:     "Not an AWS key (wrong prefix)",
			input:    "BKIAIOSFODNN7EXAMPLE is not valid",
			expected: "BKIAIOSFODNN7EXAMPLE is not valid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.Redact(tt.input)
			if result != tt.expected {
				t.Errorf("Redact() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestRedactor_AWSSecretKeys(t *testing.T) {
	r := NewRedactor()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "AWS secret key with equals",
			input:    "aws_secret_access_key=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			expected: "aws_secret_access_key=[REDACTED]",
		},
		{
			name:     "AWS secret key with quotes",
			input:    `secret_key: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"`,
			expected: `secret_key=[REDACTED]`,
		},
		{
			name:     "AWS secret in environment variable format",
			input:    "export AWS_SECRET=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			expected: "export AWS_SECRET=[REDACTED]",
		},
		{
			name:     "Case insensitive matching",
			input:    "AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			expected: "AWS_SECRET_ACCESS_KEY=[REDACTED]",
		},
		{
			name:     "Secret key too short (not 40 chars)",
			input:    "secret_key=shortkey123",
			expected: "secret_key=shortkey123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.Redact(tt.input)
			if result != tt.expected {
				t.Errorf("Redact() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestRedactor_BearerTokens(t *testing.T) {
	r := NewRedactor()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Bearer token in Authorization header",
			input:    "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			expected: "Authorization: Bearer [REDACTED]",
		},
		{
			name:     "Bearer token with dots (JWT format)",
			input:    "Bearer abc123.def456.ghi789",
			expected: "Bearer [REDACTED]",
		},
		{
			name:     "Case insensitive bearer",
			input:    "bearer token_value_here",
			expected: "Bearer [REDACTED]",
		},
		{
			name:     "Bearer token in log",
			input:    "Request sent with Bearer sk_live_1234567890abcdef",
			expected: "Request sent with Bearer [REDACTED]",
		},
		{
			name:     "Not a bearer token (word too short)",
			input:    "Bearer auth required",
			expected: "Bearer auth required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.Redact(tt.input)
			if result != tt.expected {
				t.Errorf("Redact() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestRedactor_APIKeys(t *testing.T) {
	r := NewRedactor()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "API key with underscore",
			input:    "api_key=sk_live_abcdef1234567890",
			expected: "api_key=[REDACTED]",
		},
		{
			name:     "API key with dash",
			input:    "api-key: pk_test_1234567890abcdefghij",
			expected: "api-key=[REDACTED]",
		},
		{
			name:     "camelCase apiKey",
			input:    `apiKey: "1234567890abcdefghijklmnopqrst"`,
			expected: `apiKey=[REDACTED]`,
		},
		{
			name:     "Case insensitive",
			input:    "API_KEY=abcdef1234567890ghijklmnop",
			expected: "API_KEY=[REDACTED]",
		},
		{
			name:     "API key too short",
			input:    "api_key=short",
			expected: "api_key=short",
		},
		{
			name:     "Generic token pattern",
			input:    "access_token=ghp_1234567890abcdefghijklmnopqrst",
			expected: "access_token=[REDACTED]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.Redact(tt.input)
			if result != tt.expected {
				t.Errorf("Redact() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestRedactor_PasswordsInURLs(t *testing.T) {
	r := NewRedactor()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "HTTP URL with password",
			input:    "https://user:secretpass@example.com/path",
			expected: "https://user:[REDACTED]@example.com/path",
		},
		{
			name:     "Database URL with password",
			input:    "postgresql://dbuser:dbpass123@localhost:5432/mydb",
			expected: "postgresql://dbuser:[REDACTED]@localhost:5432/mydb",
		},
		{
			name:     "MongoDB connection string",
			input:    "mongodb://admin:complexPass123@mongo.example.com:27017/db",
			expected: "mongodb://admin:[REDACTED]@mongo.example.com:27017/db",
		},
		{
			name:     "Redis URL with password",
			input:    "redis://default:secret@redis.example.com:6379/0",
			expected: "redis://default:[REDACTED]@redis.example.com:6379/0",
		},
		{
			name:     "URL without password",
			input:    "https://user@example.com/path",
			expected: "https://user@example.com/path",
		},
		{
			name:     "URL without credentials",
			input:    "https://example.com/path",
			expected: "https://example.com/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.Redact(tt.input)
			if result != tt.expected {
				t.Errorf("Redact() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestRedactor_DatabaseConnectionStrings(t *testing.T) {
	r := NewRedactor()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "SQL Server connection string",
			input:    "Server=myserver;Database=mydb;User Id=myuser;Password=mypassword;",
			expected: "Server=myserver;Database=mydb;User Id=myuser;Password=[REDACTED];",
		},
		{
			name:     "Connection string with pwd",
			input:    "Data Source=server;Initial Catalog=db;User ID=user;pwd=secret123",
			expected: "Data Source=server;Initial Catalog=db;User ID=user;pwd=[REDACTED]",
		},
		{
			name:     "Case insensitive password",
			input:    "PASSWORD=MySecretPass123",
			expected: "PASSWORD=[REDACTED]",
		},
		{
			name:     "Password with quotes",
			input:    `password='complex!pass@123'`,
			expected: `password=[REDACTED]`,
		},
		{
			name:     "Password with double quotes",
			input:    `Password="my secret pass"`,
			expected: `Password=[REDACTED]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.Redact(tt.input)
			if result != tt.expected {
				t.Errorf("Redact() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestRedactor_GenericSecrets(t *testing.T) {
	r := NewRedactor()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Secret in configuration",
			input:    "client_secret=abcdef1234567890ghijklmnopqrst",
			expected: "client_secret=[REDACTED]",
		},
		{
			name:     "Private key",
			input:    "private_key=MIIEvQIBADANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA",
			expected: "private_key=[REDACTED]",
		},
		{
			name:     "Private key with dash",
			input:    "private-key: base64encodedkeydata1234567890",
			expected: "private-key=[REDACTED]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.Redact(tt.input)
			if result != tt.expected {
				t.Errorf("Redact() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestRedactor_MultiplePatterns(t *testing.T) {
	r := NewRedactor()

	tests := []struct {
		name     string
		input    string
		validate func(t *testing.T, result string)
	}{
		{
			name: "Multiple sensitive values in one string",
			input: `export AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
export AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
export API_KEY=sk_live_1234567890abcdefghij`,
			validate: func(t *testing.T, result string) {
				if strings.Contains(result, "AKIAIOSFODNN7EXAMPLE") {
					t.Error("AWS access key not redacted")
				}
				if strings.Contains(result, "wJalrXUtnFEMI") {
					t.Error("AWS secret key not redacted")
				}
				if strings.Contains(result, "sk_live_1234567890abcdefghij") {
					t.Error("API key not redacted")
				}
				if !strings.Contains(result, "[REDACTED]") {
					t.Error("No redaction markers found")
				}
			},
		},
		{
			name:  "Log line with URL and bearer token",
			input: "Connecting to https://user:pass123@api.example.com with Authorization: Bearer token_abc123def456",
			validate: func(t *testing.T, result string) {
				if strings.Contains(result, "pass123") {
					t.Error("Password not redacted")
				}
				if strings.Contains(result, "token_abc123def456") {
					t.Error("Bearer token not redacted")
				}
			},
		},
		{
			name:  "Configuration file content",
			input: "db_url=postgresql://user:secret@localhost/db\napi_key=sk_test_abcdefghij1234567890\nbearer_token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.payload.signature",
			validate: func(t *testing.T, result string) {
				if strings.Contains(result, "secret@") {
					t.Error("Database password not redacted")
				}
				if strings.Contains(result, "sk_test_abcdefghij1234567890") {
					t.Error("API key not redacted")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.Redact(tt.input)
			tt.validate(t, result)
		})
	}
}

func TestRedactor_NoFalsePositives(t *testing.T) {
	r := NewRedactor()

	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "Normal text should not be redacted",
			input: "This is a normal log message without secrets",
		},
		{
			name:  "Variable names without values",
			input: "Please set API_KEY and SECRET in your environment",
		},
		{
			name:  "Short password values should not trigger (less than 3 chars)",
			input: "password=ab",
		},
		{
			name:  "URLs without credentials",
			input: "https://example.com/api/v1/users",
		},
		{
			name:  "Documentation examples",
			input: "Use format: api_key=YOUR_KEY_HERE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.Redact(tt.input)
			if result != tt.input {
				t.Errorf("False positive redaction: input=%q, output=%q", tt.input, result)
			}
		})
	}
}

func TestRedactor_ThreadSafety(t *testing.T) {
	r := NewRedactor()

	// Run multiple goroutines calling Redact concurrently
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = r.Redact("api_key=sk_live_1234567890abcdefghij")
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

func BenchmarkRedactor_Redact(b *testing.B) {
	r := NewRedactor()
	input := `export AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
export AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
export API_KEY=sk_live_1234567890abcdefghij
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9
Connection: postgresql://user:password123@localhost:5432/mydb`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.Redact(input)
	}
}
