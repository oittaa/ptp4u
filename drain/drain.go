/*
Copyright (c) Facebook, Inc. and its affiliates.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package drain

import (
	"os"
)

// Drain is a drain check interface
type Drain interface {
	// Check returns true if the service needs to be drained
	Check() bool
}

// Undrain checks the existence of a undrain file
func Undrain(filename string) bool {
	if _, err := os.Stat(filename); err == nil {
		return true
	}

	return false
}
