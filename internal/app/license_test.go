// Copyright (c) 2026 Iqbal Uddin Khan. See LICENSE.

package app

import (
	"os"
	"testing"
)

// The LICENSE at the repository root is canonical; the embedded and packaged copies must match it.
func TestLicenseCopiesAreInSync(t *testing.T) {
	root, err := os.ReadFile("../../LICENSE")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"license.txt", "../../packaging/COPYING.txt"} {
		b, err := os.ReadFile(p)
		if err != nil || string(b) != string(root) {
			t.Errorf("%s differs from LICENSE (copy LICENSE over it)", p)
		}
	}
	if string(root) == "" || licenseText != string(root) {
		t.Error("embedded license text differs from LICENSE")
	}
}
