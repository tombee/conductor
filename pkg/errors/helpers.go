// Copyright 2025 Tom Barlow
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package errors

import (
	"errors"
	"fmt"
)

// Wrap creates a new error that wraps the given error with additional context.
// If err is nil, returns nil.
//
// Usage:
//
//	if err := doSomething(); err != nil {
//	    return errors.Wrap(err, "doing something")
//	}
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

// Wrapf creates a new error that wraps the given error with formatted context.
// If err is nil, returns nil.
//
// Usage:
//
//	if err := loadFile(path); err != nil {
//	    return errors.Wrapf(err, "loading file %s", path)
//	}
func Wrapf(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	message := fmt.Sprintf(format, args...)
	return fmt.Errorf("%s: %w", message, err)
}

// Is reports whether any error in err's tree matches target.
// This is a convenience wrapper around errors.Is from the standard library.
//
// Usage:
//
//	if errors.Is(err, &NotFoundError{}) {
//	    // handle not found
//	}
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// As finds the first error in err's tree that matches target type,
// and if one is found, sets target to that error value and returns true.
// This is a convenience wrapper around errors.As from the standard library.
//
// Usage:
//
//	var configErr *ConfigError
//	if errors.As(err, &configErr) {
//	    log.Printf("Config error at key: %s", configErr.Key)
//	}
func As(err error, target interface{}) bool {
	return errors.As(err, target)
}

// Unwrap returns the result of calling the Unwrap method on err,
// if err's type contains an Unwrap method returning error.
// This is a convenience wrapper around errors.Unwrap from the standard library.
func Unwrap(err error) error {
	return errors.Unwrap(err)
}

// New creates a new error with the given message.
// This is a convenience wrapper around errors.New from the standard library.
func New(message string) error {
	return errors.New(message)
}
