package transform

import (
	"context"
	"strings"
	"testing"
)

func TestSort(t *testing.T) {
	c, err := New(nil)
	if err != nil {
		t.Fatalf("failed to create connector: %v", err)
	}

	tests := []struct {
		name    string
		inputs  map[string]interface{}
		want    []interface{}
		wantErr bool
	}{
		{
			name: "sort numbers",
			inputs: map[string]interface{}{
				"data": []interface{}{3.0, 1.0, 4.0, 1.0, 5.0, 9.0, 2.0, 6.0},
			},
			want: []interface{}{1.0, 1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 9.0},
		},
		{
			name: "sort strings",
			inputs: map[string]interface{}{
				"data": []interface{}{"zebra", "apple", "mango", "banana"},
			},
			want: []interface{}{"apple", "banana", "mango", "zebra"},
		},
		{
			name: "sort by object field",
			inputs: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{"name": "charlie", "age": 30.0},
					map[string]interface{}{"name": "alice", "age": 25.0},
					map[string]interface{}{"name": "bob", "age": 35.0},
				},
				"expr": ".name",
			},
			want: []interface{}{
				map[string]interface{}{"name": "alice", "age": 25.0},
				map[string]interface{}{"name": "bob", "age": 35.0},
				map[string]interface{}{"name": "charlie", "age": 30.0},
			},
		},
		{
			name: "sort by numeric field",
			inputs: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{"id": 3.0},
					map[string]interface{}{"id": 1.0},
					map[string]interface{}{"id": 2.0},
				},
				"expr": ".id",
			},
			want: []interface{}{
				map[string]interface{}{"id": 1.0},
				map[string]interface{}{"id": 2.0},
				map[string]interface{}{"id": 3.0},
			},
		},
		{
			name: "sort empty array",
			inputs: map[string]interface{}{
				"data": []interface{}{},
			},
			want: []interface{}{},
		},
		{
			name: "sort single element",
			inputs: map[string]interface{}{
				"data": []interface{}{42.0},
			},
			want: []interface{}{42.0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := c.sort(context.Background(), tt.inputs)
			if (err != nil) != tt.wantErr {
				t.Errorf("sort() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				got := result.Response.([]interface{})
				if !deepEqual(got, tt.want) {
					t.Errorf("sort() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestSort_Errors(t *testing.T) {
	c, err := New(nil)
	if err != nil {
		t.Fatalf("failed to create connector: %v", err)
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
			name: "expr not a string",
			inputs: map[string]interface{}{
				"data": []interface{}{1, 2, 3},
				"expr": 123,
			},
			wantError: "expr must be a string",
		},
		{
			name: "empty expr",
			inputs: map[string]interface{}{
				"data": []interface{}{1, 2, 3},
				"expr": "",
			},
			wantError: "expr cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.sort(context.Background(), tt.inputs)
			if err == nil {
				t.Errorf("sort() expected error containing %q, got nil", tt.wantError)
				return
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Errorf("sort() error = %v, want error containing %q", err, tt.wantError)
			}
		})
	}
}

func TestSort_MaxItems(t *testing.T) {
	// Create connector with small max items limit
	config := DefaultConfig()
	config.MaxArrayItems = 5

	c, err := New(config)
	if err != nil {
		t.Fatalf("failed to create connector: %v", err)
	}

	inputs := map[string]interface{}{
		"data": []interface{}{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
	}

	_, err = c.sort(context.Background(), inputs)
	if err == nil {
		t.Error("sort() expected error for exceeding max items, got nil")
		return
	}

	if !strings.Contains(err.Error(), "exceeds maximum") {
		t.Errorf("sort() error = %v, want error containing 'exceeds maximum'", err)
	}
}

func TestGroup(t *testing.T) {
	c, err := New(nil)
	if err != nil {
		t.Fatalf("failed to create connector: %v", err)
	}

	tests := []struct {
		name    string
		inputs  map[string]interface{}
		want    []interface{}
		wantErr bool
	}{
		{
			name: "group by category",
			inputs: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{"name": "apple", "category": "fruit"},
					map[string]interface{}{"name": "carrot", "category": "vegetable"},
					map[string]interface{}{"name": "banana", "category": "fruit"},
					map[string]interface{}{"name": "broccoli", "category": "vegetable"},
				},
				"expr": ".category",
			},
			want: []interface{}{
				[]interface{}{
					map[string]interface{}{"name": "apple", "category": "fruit"},
					map[string]interface{}{"name": "banana", "category": "fruit"},
				},
				[]interface{}{
					map[string]interface{}{"name": "carrot", "category": "vegetable"},
					map[string]interface{}{"name": "broccoli", "category": "vegetable"},
				},
			},
		},
		{
			name: "group by numeric value",
			inputs: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{"id": 1.0, "status": 1.0},
					map[string]interface{}{"id": 2.0, "status": 2.0},
					map[string]interface{}{"id": 3.0, "status": 1.0},
					map[string]interface{}{"id": 4.0, "status": 2.0},
				},
				"expr": ".status",
			},
			want: []interface{}{
				[]interface{}{
					map[string]interface{}{"id": 1.0, "status": 1.0},
					map[string]interface{}{"id": 3.0, "status": 1.0},
				},
				[]interface{}{
					map[string]interface{}{"id": 2.0, "status": 2.0},
					map[string]interface{}{"id": 4.0, "status": 2.0},
				},
			},
		},
		{
			name: "group empty array",
			inputs: map[string]interface{}{
				"data": []interface{}{},
				"expr": ".field",
			},
			want: []interface{}{},
		},
		{
			name: "group single element",
			inputs: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{"value": 1.0},
				},
				"expr": ".value",
			},
			want: []interface{}{
				[]interface{}{
					map[string]interface{}{"value": 1.0},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := c.group(context.Background(), tt.inputs)
			if (err != nil) != tt.wantErr {
				t.Errorf("group() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				got := result.Response.([]interface{})
				if !deepEqual(got, tt.want) {
					t.Errorf("group() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestGroup_Errors(t *testing.T) {
	c, err := New(nil)
	if err != nil {
		t.Fatalf("failed to create connector: %v", err)
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
				"expr": ".field",
			},
			wantError: "input must be an array",
		},
		{
			name: "missing expr",
			inputs: map[string]interface{}{
				"data": []interface{}{1, 2, 3},
			},
			wantError: "missing required parameter: expr",
		},
		{
			name: "expr not a string",
			inputs: map[string]interface{}{
				"data": []interface{}{1, 2, 3},
				"expr": 123,
			},
			wantError: "expr must be a string",
		},
		{
			name: "empty expr",
			inputs: map[string]interface{}{
				"data": []interface{}{1, 2, 3},
				"expr": "",
			},
			wantError: "expr cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.group(context.Background(), tt.inputs)
			if err == nil {
				t.Errorf("group() expected error containing %q, got nil", tt.wantError)
				return
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Errorf("group() error = %v, want error containing %q", err, tt.wantError)
			}
		})
	}
}

func TestGroup_MaxItems(t *testing.T) {
	// Create connector with small max items limit
	config := DefaultConfig()
	config.MaxArrayItems = 5

	c, err := New(config)
	if err != nil {
		t.Fatalf("failed to create connector: %v", err)
	}

	inputs := map[string]interface{}{
		"data": []interface{}{
			map[string]interface{}{"id": 1},
			map[string]interface{}{"id": 2},
			map[string]interface{}{"id": 3},
			map[string]interface{}{"id": 4},
			map[string]interface{}{"id": 5},
			map[string]interface{}{"id": 6},
		},
		"expr": ".id",
	}

	_, err = c.group(context.Background(), inputs)
	if err == nil {
		t.Error("group() expected error for exceeding max items, got nil")
		return
	}

	if !strings.Contains(err.Error(), "exceeds maximum") {
		t.Errorf("group() error = %v, want error containing 'exceeds maximum'", err)
	}
}
