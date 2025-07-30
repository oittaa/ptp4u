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
	"testing"
)

func TestCheck(t *testing.T) {
	file, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	check := &FileDrain{FileName: file.Name()}
	if !check.Check() {
		t.Fatal("expected check.Check() to return true, but it returned false")
	}

	os.Remove(file.Name())
	if check.Check() {
		t.Fatal("expected check.Check() to return false after removal, but it returned true")
	}
}

func TestUndrain(t *testing.T) {
	file, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	if !Undrain(file.Name()) {
		t.Fatal("expected Undrain() to return true, but it returned false")
	}

	os.Remove(file.Name())
	if Undrain(file.Name()) {
		t.Fatal("expected Undrain() to return false after removal, but it returned true")
	}
}
