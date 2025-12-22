package expression

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContainsFunc_Slice(t *testing.T) {
	tests := []struct {
		name       string
		collection interface{}
		target     interface{}
		want       bool
	}{
		{
			name:       "string slice contains element",
			collection: []interface{}{"a", "b", "c"},
			target:     "b",
			want:       true,
		},
		{
			name:       "string slice missing element",
			collection: []interface{}{"a", "b", "c"},
			target:     "d",
			want:       false,
		},
		{
			name:       "int slice contains element",
			collection: []interface{}{1, 2, 3},
			target:     2,
			want:       true,
		},
		{
			name:       "empty slice",
			collection: []interface{}{},
			target:     "x",
			want:       false,
		},
		{
			name:       "nil collection",
			collection: nil,
			target:     "x",
			want:       false,
		},
		{
			name:       "mixed type slice",
			collection: []interface{}{"a", 1, true},
			target:     true,
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := containsFunc(tt.collection, tt.target)
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestContainsFunc_String(t *testing.T) {
	tests := []struct {
		name   string
		str    string
		substr string
		want   bool
	}{
		{
			name:   "contains substring",
			str:    "hello world",
			substr: "world",
			want:   true,
		},
		{
			name:   "missing substring",
			str:    "hello world",
			substr: "foo",
			want:   false,
		},
		{
			name:   "exact match",
			str:    "hello",
			substr: "hello",
			want:   true,
		},
		{
			name:   "empty string",
			str:    "",
			substr: "x",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := containsFunc(tt.str, tt.substr)
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestContainsFunc_Map(t *testing.T) {
	m := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
	}

	tests := []struct {
		name   string
		target interface{}
		want   bool
	}{
		{
			name:   "key exists",
			target: "key1",
			want:   true,
		},
		{
			name:   "key missing",
			target: "key3",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := containsFunc(m, tt.target)
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestContainsFunc_InvalidArgs(t *testing.T) {
	// Wrong number of arguments
	_, err := containsFunc()
	require.Error(t, err)

	_, err = containsFunc("one")
	require.Error(t, err)

	_, err = containsFunc("one", "two", "three")
	require.Error(t, err)
}

func TestContainsFunc_UnsupportedType(t *testing.T) {
	// Non-collection type returns false
	result, err := containsFunc(42, "x")
	require.NoError(t, err)
	assert.False(t, result.(bool))
}

func TestLenFunc(t *testing.T) {
	tests := []struct {
		name    string
		arg     interface{}
		want    int
		wantErr bool
	}{
		{
			name: "slice length",
			arg:  []interface{}{"a", "b", "c"},
			want: 3,
		},
		{
			name: "empty slice",
			arg:  []interface{}{},
			want: 0,
		},
		{
			name: "string length",
			arg:  "hello",
			want: 5,
		},
		{
			name: "empty string",
			arg:  "",
			want: 0,
		},
		{
			name: "map length",
			arg:  map[string]interface{}{"a": 1, "b": 2},
			want: 2,
		},
		{
			name: "nil",
			arg:  nil,
			want: 0,
		},
		{
			name:    "unsupported type",
			arg:     42,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := lenFunc(tt.arg)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestLenFunc_InvalidArgs(t *testing.T) {
	// Wrong number of arguments
	_, err := lenFunc()
	require.Error(t, err)

	_, err = lenFunc("one", "two")
	require.Error(t, err)
}
