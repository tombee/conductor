package util

import "testing"

func TestContains(t *testing.T) {
	tests := []struct {
		name  string
		slice interface{}
		item  interface{}
		want  bool
	}{
		// String slice tests
		{
			name:  "string slice - found",
			slice: []string{"apple", "banana", "cherry"},
			item:  "banana",
			want:  true,
		},
		{
			name:  "string slice - not found",
			slice: []string{"apple", "banana", "cherry"},
			item:  "orange",
			want:  false,
		},
		{
			name:  "string slice - empty",
			slice: []string{},
			item:  "apple",
			want:  false,
		},
		{
			name:  "string slice - single element found",
			slice: []string{"only"},
			item:  "only",
			want:  true,
		},
		{
			name:  "string slice - single element not found",
			slice: []string{"only"},
			item:  "nope",
			want:  false,
		},
		// Integer slice tests
		{
			name:  "int slice - found",
			slice: []int{1, 2, 3, 4, 5},
			item:  3,
			want:  true,
		},
		{
			name:  "int slice - not found",
			slice: []int{1, 2, 3, 4, 5},
			item:  10,
			want:  false,
		},
		{
			name:  "int slice - empty",
			slice: []int{},
			item:  1,
			want:  false,
		},
		{
			name:  "int slice - single element found",
			slice: []int{42},
			item:  42,
			want:  true,
		},
		{
			name:  "int slice - zero value",
			slice: []int{0, 1, 2},
			item:  0,
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got bool
			switch slice := tt.slice.(type) {
			case []string:
				got = Contains(slice, tt.item.(string))
			case []int:
				got = Contains(slice, tt.item.(int))
			default:
				t.Fatalf("unsupported type: %T", slice)
			}

			if got != tt.want {
				t.Errorf("Contains() = %v, want %v", got, tt.want)
			}
		})
	}
}
