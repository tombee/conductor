package expression

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPreprocessTemplate(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		ctx        map[string]interface{}
		want       string
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "no template markers",
			expression: `steps.check.status == "success"`,
			ctx:        map[string]interface{}{},
			want:       `steps.check.status == "success"`,
			wantErr:    false,
		},
		{
			name:       "empty expression",
			expression: "",
			ctx:        map[string]interface{}{},
			want:       "",
			wantErr:    false,
		},
		{
			name:       "string value replacement",
			expression: `{{.steps.check.status}} == "success"`,
			ctx: map[string]interface{}{
				"steps": map[string]interface{}{
					"check": map[string]interface{}{
						"status": "success",
					},
				},
			},
			want:    `"success" == "success"`,
			wantErr: false,
		},
		{
			name:       "string value without leading dot",
			expression: `{{steps.check.status}} == "success"`,
			ctx: map[string]interface{}{
				"steps": map[string]interface{}{
					"check": map[string]interface{}{
						"status": "success",
					},
				},
			},
			want:    `"success" == "success"`,
			wantErr: false,
		},
		{
			name:       "integer value replacement",
			expression: `{{.steps.analyze.score}} > 80`,
			ctx: map[string]interface{}{
				"steps": map[string]interface{}{
					"analyze": map[string]interface{}{
						"score": 95,
					},
				},
			},
			want:    `95 > 80`,
			wantErr: false,
		},
		{
			name:       "boolean value replacement",
			expression: `{{.steps.validate.passed}} == true`,
			ctx: map[string]interface{}{
				"steps": map[string]interface{}{
					"validate": map[string]interface{}{
						"passed": true,
					},
				},
			},
			want:    `true == true`,
			wantErr: false,
		},
		{
			name:       "nil value replacement",
			expression: `{{.steps.optional.value}} == nil`,
			ctx: map[string]interface{}{
				"steps": map[string]interface{}{
					"optional": map[string]interface{}{
						"value": nil,
					},
				},
			},
			want:    `nil == nil`,
			wantErr: false,
		},
		{
			name:       "multiple templates",
			expression: `{{.steps.a.x}} > {{.steps.b.y}}`,
			ctx: map[string]interface{}{
				"steps": map[string]interface{}{
					"a": map[string]interface{}{"x": 10},
					"b": map[string]interface{}{"y": 5},
				},
			},
			want:    `10 > 5`,
			wantErr: false,
		},
		{
			name:       "string with quotes needs escaping",
			expression: `{{.steps.check.message}} == "ok"`,
			ctx: map[string]interface{}{
				"steps": map[string]interface{}{
					"check": map[string]interface{}{
						"message": `He said "hello"`,
					},
				},
			},
			want:    `"He said \"hello\"" == "ok"`,
			wantErr: false,
		},
		{
			name:       "string with backslashes needs escaping",
			expression: `{{.steps.check.path}}`,
			ctx: map[string]interface{}{
				"steps": map[string]interface{}{
					"check": map[string]interface{}{
						"path": `C:\Users\test`,
					},
				},
			},
			want:    `"C:\\Users\\test"`,
			wantErr: false,
		},
		{
			name:       "path not found error",
			expression: `{{.steps.missing.value}} == true`,
			ctx: map[string]interface{}{
				"steps": map[string]interface{}{
					"check": map[string]interface{}{},
				},
			},
			wantErr: true,
			errMsg:  "path not found",
		},
		{
			name:       "nested path missing key",
			expression: `{{.steps.check.missing}} == true`,
			ctx: map[string]interface{}{
				"steps": map[string]interface{}{
					"check": map[string]interface{}{
						"status": "success",
					},
				},
			},
			wantErr: true,
			errMsg:  "missing key 'missing'",
		},
		{
			name:       "cannot index into non-map",
			expression: `{{.steps.check.status.invalid}} == true`,
			ctx: map[string]interface{}{
				"steps": map[string]interface{}{
					"check": map[string]interface{}{
						"status": "success",
					},
				},
			},
			wantErr: true,
			errMsg:  "cannot index into string",
		},
		{
			name:       "float value",
			expression: `{{.steps.measure.value}} > 3.14`,
			ctx: map[string]interface{}{
				"steps": map[string]interface{}{
					"measure": map[string]interface{}{
						"value": 3.14159,
					},
				},
			},
			want:    `3.14159 > 3.14`,
			wantErr: false,
		},
		{
			name:       "inputs reference",
			expression: `{{.inputs.name}} == "test"`,
			ctx: map[string]interface{}{
				"inputs": map[string]interface{}{
					"name": "test",
				},
			},
			want:    `"test" == "test"`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PreprocessTemplate(tt.expression, tt.ctx)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestResolvePath(t *testing.T) {
	ctx := map[string]interface{}{
		"steps": map[string]interface{}{
			"check": map[string]interface{}{
				"status": "success",
				"code":   0,
			},
		},
		"inputs": map[string]interface{}{
			"name": "test",
		},
	}

	tests := []struct {
		name    string
		path    string
		want    interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name:    "simple path",
			path:    "inputs.name",
			want:    "test",
			wantErr: false,
		},
		{
			name:    "nested path",
			path:    "steps.check.status",
			want:    "success",
			wantErr: false,
		},
		{
			name:    "integer value",
			path:    "steps.check.code",
			want:    0,
			wantErr: false,
		},
		{
			name:    "map value",
			path:    "steps.check",
			want:    map[string]interface{}{"status": "success", "code": 0},
			wantErr: false,
		},
		{
			name:    "empty path",
			path:    "",
			wantErr: true,
			errMsg:  "empty path",
		},
		{
			name:    "missing key",
			path:    "steps.missing",
			wantErr: true,
			errMsg:  "path not found",
		},
		{
			name:    "path with spaces",
			path:    " steps . check . status ",
			want:    "success",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolvePath(tt.path, ctx)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestValueToLiteral(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		want  string
	}{
		{name: "nil", value: nil, want: "nil"},
		{name: "true", value: true, want: "true"},
		{name: "false", value: false, want: "false"},
		{name: "int", value: 42, want: "42"},
		{name: "int64", value: int64(42), want: "42"},
		{name: "int32", value: int32(42), want: "42"},
		{name: "float64", value: float64(3.14), want: "3.14"},
		{name: "float32", value: float32(3.14), want: "3.14"},
		{name: "string", value: "hello", want: `"hello"`},
		{name: "empty string", value: "", want: `""`},
		{name: "string with quotes", value: `He said "hello"`, want: `"He said \"hello\""`},
		{name: "string with backslash", value: `C:\Users`, want: `"C:\\Users"`},
		{name: "string with both", value: `Path "C:\test"`, want: `"Path \"C:\\test\""`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := valueToLiteral(tt.value)
			assert.Equal(t, tt.want, got)
		})
	}
}
