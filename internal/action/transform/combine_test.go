package transform

import (
	"context"
	"strings"
	"testing"
)

func TestMerge_ShallowStrategy(t *testing.T) {
	c, err := New(nil)
	if err != nil {
		t.Fatalf("failed to create integration: %v", err)
	}

	tests := []struct {
		name    string
		inputs  map[string]interface{}
		want    map[string]interface{}
		wantErr bool
	}{
		{
			name: "merge two objects - rightmost wins",
			inputs: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{"a": 1, "b": 2},
					map[string]interface{}{"b": 3, "c": 4},
				},
			},
			want: map[string]interface{}{"a": 1, "b": 3, "c": 4},
		},
		{
			name: "merge three objects",
			inputs: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{"a": 1},
					map[string]interface{}{"b": 2},
					map[string]interface{}{"c": 3},
				},
			},
			want: map[string]interface{}{"a": 1, "b": 2, "c": 3},
		},
		{
			name: "merge with sources parameter",
			inputs: map[string]interface{}{
				"sources": []interface{}{
					map[string]interface{}{"x": 10},
					map[string]interface{}{"y": 20},
				},
			},
			want: map[string]interface{}{"x": 10, "y": 20},
		},
		{
			name: "conflict resolution - rightmost wins",
			inputs: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{"key": "first"},
					map[string]interface{}{"key": "second"},
					map[string]interface{}{"key": "third"},
				},
			},
			want: map[string]interface{}{"key": "third"},
		},
		{
			name: "empty array",
			inputs: map[string]interface{}{
				"data": []interface{}{},
			},
			want: map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := c.merge(context.Background(), tt.inputs)
			if (err != nil) != tt.wantErr {
				t.Errorf("merge() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				got := result.Response.(map[string]interface{})
				if !mapsEqual(got, tt.want) {
					t.Errorf("merge() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestMerge_DeepStrategy(t *testing.T) {
	c, err := New(nil)
	if err != nil {
		t.Fatalf("failed to create integration: %v", err)
	}

	tests := []struct {
		name   string
		inputs map[string]interface{}
		want   map[string]interface{}
	}{
		{
			name: "deep merge nested objects",
			inputs: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{
						"a": 1,
						"nested": map[string]interface{}{
							"x": 10,
							"y": 20,
						},
					},
					map[string]interface{}{
						"b": 2,
						"nested": map[string]interface{}{
							"y": 30,
							"z": 40,
						},
					},
				},
				"strategy": "deep",
			},
			want: map[string]interface{}{
				"a": 1,
				"b": 2,
				"nested": map[string]interface{}{
					"x": 10,
					"y": 30,
					"z": 40,
				},
			},
		},
		{
			name: "deep merge with array concatenation",
			inputs: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{
						"items": []interface{}{1, 2},
					},
					map[string]interface{}{
						"items": []interface{}{3, 4},
					},
				},
				"strategy": "deep",
			},
			want: map[string]interface{}{
				"items": []interface{}{1, 2, 3, 4},
			},
		},
		{
			name: "deep merge multiple levels",
			inputs: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{
						"level1": map[string]interface{}{
							"level2": map[string]interface{}{
								"a": 1,
							},
						},
					},
					map[string]interface{}{
						"level1": map[string]interface{}{
							"level2": map[string]interface{}{
								"b": 2,
							},
						},
					},
				},
				"strategy": "deep",
			},
			want: map[string]interface{}{
				"level1": map[string]interface{}{
					"level2": map[string]interface{}{
						"a": 1,
						"b": 2,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := c.merge(context.Background(), tt.inputs)
			if err != nil {
				t.Errorf("merge() error = %v", err)
				return
			}
			got := result.Response.(map[string]interface{})
			if !deepEqual(got, tt.want) {
				t.Errorf("merge() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMerge_Arrays(t *testing.T) {
	c, err := New(nil)
	if err != nil {
		t.Fatalf("failed to create integration: %v", err)
	}

	tests := []struct {
		name   string
		inputs map[string]interface{}
		want   []interface{}
	}{
		{
			name: "merge arrays concatenates them",
			inputs: map[string]interface{}{
				"data": []interface{}{
					[]interface{}{1, 2},
					[]interface{}{3, 4},
				},
			},
			want: []interface{}{1, 2, 3, 4},
		},
		{
			name: "merge multiple arrays",
			inputs: map[string]interface{}{
				"data": []interface{}{
					[]interface{}{1},
					[]interface{}{2},
					[]interface{}{3},
				},
			},
			want: []interface{}{1, 2, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := c.merge(context.Background(), tt.inputs)
			if err != nil {
				t.Errorf("merge() error = %v", err)
				return
			}
			got := result.Response.([]interface{})
			if !arraysEqual(got, tt.want) {
				t.Errorf("merge() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMerge_Errors(t *testing.T) {
	c, err := New(nil)
	if err != nil {
		t.Fatalf("failed to create integration: %v", err)
	}

	tests := []struct {
		name      string
		inputs    map[string]interface{}
		wantError string
	}{
		{
			name:      "missing data and sources",
			inputs:    map[string]interface{}{},
			wantError: "missing required parameter",
		},
		{
			name: "invalid strategy",
			inputs: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{"a": 1},
				},
				"strategy": "invalid",
			},
			wantError: "invalid strategy",
		},
		{
			name: "mixed types - object and array",
			inputs: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{"a": 1},
					[]interface{}{1, 2},
				},
			},
			wantError: "not an object",
		},
		{
			name: "sources not an array",
			inputs: map[string]interface{}{
				"sources": "not an array",
			},
			wantError: "sources must be an array",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.merge(context.Background(), tt.inputs)
			if err == nil {
				t.Errorf("merge() expected error containing %q, got nil", tt.wantError)
				return
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Errorf("merge() error = %v, want error containing %q", err, tt.wantError)
			}
		})
	}
}

func TestConcat(t *testing.T) {
	c, err := New(nil)
	if err != nil {
		t.Fatalf("failed to create integration: %v", err)
	}

	tests := []struct {
		name    string
		inputs  map[string]interface{}
		want    []interface{}
		wantErr bool
	}{
		{
			name: "concat two arrays",
			inputs: map[string]interface{}{
				"data": []interface{}{
					[]interface{}{1, 2, 3},
					[]interface{}{4, 5, 6},
				},
			},
			want: []interface{}{1, 2, 3, 4, 5, 6},
		},
		{
			name: "concat multiple arrays",
			inputs: map[string]interface{}{
				"data": []interface{}{
					[]interface{}{1},
					[]interface{}{2},
					[]interface{}{3},
					[]interface{}{4},
				},
			},
			want: []interface{}{1, 2, 3, 4},
		},
		{
			name: "concat with sources parameter",
			inputs: map[string]interface{}{
				"sources": []interface{}{
					[]interface{}{"a", "b"},
					[]interface{}{"c", "d"},
				},
			},
			want: []interface{}{"a", "b", "c", "d"},
		},
		{
			name: "concat empty arrays",
			inputs: map[string]interface{}{
				"data": []interface{}{
					[]interface{}{},
					[]interface{}{},
				},
			},
			want: []interface{}{},
		},
		{
			name: "concat with one empty array",
			inputs: map[string]interface{}{
				"data": []interface{}{
					[]interface{}{1, 2},
					[]interface{}{},
					[]interface{}{3, 4},
				},
			},
			want: []interface{}{1, 2, 3, 4},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := c.concat(context.Background(), tt.inputs)
			if (err != nil) != tt.wantErr {
				t.Errorf("concat() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				got := result.Response.([]interface{})
				if !arraysEqual(got, tt.want) {
					t.Errorf("concat() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestConcat_Errors(t *testing.T) {
	c, err := New(nil)
	if err != nil {
		t.Fatalf("failed to create integration: %v", err)
	}

	tests := []struct {
		name      string
		inputs    map[string]interface{}
		wantError string
	}{
		{
			name:      "missing data and sources",
			inputs:    map[string]interface{}{},
			wantError: "missing required parameter",
		},
		{
			name: "non-array source",
			inputs: map[string]interface{}{
				"data": []interface{}{
					[]interface{}{1, 2},
					"not an array",
				},
			},
			wantError: "not an array",
		},
		{
			name: "sources not an array",
			inputs: map[string]interface{}{
				"sources": "not an array",
			},
			wantError: "sources must be an array",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.concat(context.Background(), tt.inputs)
			if err == nil {
				t.Errorf("concat() expected error containing %q, got nil", tt.wantError)
				return
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Errorf("concat() error = %v, want error containing %q", err, tt.wantError)
			}
		})
	}
}

func TestFlatten(t *testing.T) {
	c, err := New(nil)
	if err != nil {
		t.Fatalf("failed to create integration: %v", err)
	}

	tests := []struct {
		name    string
		inputs  map[string]interface{}
		want    []interface{}
		wantErr bool
	}{
		{
			name: "flatten one level",
			inputs: map[string]interface{}{
				"data": []interface{}{
					[]interface{}{1, 2},
					[]interface{}{3, 4},
				},
			},
			want: []interface{}{1, 2, 3, 4},
		},
		{
			name: "flatten with depth 1 (default)",
			inputs: map[string]interface{}{
				"data": []interface{}{
					[]interface{}{
						[]interface{}{1, 2},
						[]interface{}{3, 4},
					},
				},
			},
			want: []interface{}{
				[]interface{}{1, 2},
				[]interface{}{3, 4},
			},
		},
		{
			name: "flatten with depth 2",
			inputs: map[string]interface{}{
				"data": []interface{}{
					[]interface{}{
						[]interface{}{1, 2},
						[]interface{}{3, 4},
					},
				},
				"depth": 2,
			},
			want: []interface{}{1, 2, 3, 4},
		},
		{
			name: "flatten mixed nested and flat",
			inputs: map[string]interface{}{
				"data": []interface{}{
					1,
					[]interface{}{2, 3},
					4,
					[]interface{}{5, 6},
				},
			},
			want: []interface{}{1, 2, 3, 4, 5, 6},
		},
		{
			name: "flatten empty array",
			inputs: map[string]interface{}{
				"data": []interface{}{},
			},
			want: []interface{}{},
		},
		{
			name: "flatten array with no nesting",
			inputs: map[string]interface{}{
				"data": []interface{}{1, 2, 3, 4},
			},
			want: []interface{}{1, 2, 3, 4},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := c.flatten(context.Background(), tt.inputs)
			if (err != nil) != tt.wantErr {
				t.Errorf("flatten() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				got := result.Response.([]interface{})
				if !deepEqual(got, tt.want) {
					t.Errorf("flatten() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestFlatten_Errors(t *testing.T) {
	c, err := New(nil)
	if err != nil {
		t.Fatalf("failed to create integration: %v", err)
	}

	tests := []struct {
		name      string
		inputs    map[string]interface{}
		wantError string
	}{
		{
			name:      "missing data",
			inputs:    map[string]interface{}{},
			wantError: "missing required parameter",
		},
		{
			name: "null data",
			inputs: map[string]interface{}{
				"data": nil,
			},
			wantError: "null or undefined",
		},
		{
			name: "non-array data",
			inputs: map[string]interface{}{
				"data": "not an array",
			},
			wantError: "input must be an array",
		},
		{
			name: "invalid depth",
			inputs: map[string]interface{}{
				"data":  []interface{}{1, 2},
				"depth": 0,
			},
			wantError: "depth must be at least 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.flatten(context.Background(), tt.inputs)
			if err == nil {
				t.Errorf("flatten() expected error containing %q, got nil", tt.wantError)
				return
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Errorf("flatten() error = %v, want error containing %q", err, tt.wantError)
			}
		})
	}
}

// Helper functions

func mapsEqual(a, b map[string]interface{}) bool {
	if len(a) != len(b) {
		return false
	}
	for key, valA := range a {
		valB, exists := b[key]
		if !exists {
			return false
		}
		if !deepEqual(valA, valB) {
			return false
		}
	}
	return true
}

func arraysEqual(a, b []interface{}) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !deepEqual(a[i], b[i]) {
			return false
		}
	}
	return true
}

func deepEqual(a, b interface{}) bool {
	switch aVal := a.(type) {
	case map[string]interface{}:
		bVal, ok := b.(map[string]interface{})
		if !ok {
			return false
		}
		return mapsEqual(aVal, bVal)
	case []interface{}:
		bVal, ok := b.([]interface{})
		if !ok {
			return false
		}
		return arraysEqual(aVal, bVal)
	default:
		return a == b
	}
}
