package util

// Contains checks if a slice contains a specific item.
// It works with any comparable type (strings, ints, etc.).
func Contains[T comparable](slice []T, item T) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
