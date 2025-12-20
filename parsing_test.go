package main

import (
	"testing"
)

func TestParsePackages_Apt(t *testing.T) {
	output := `Listing...
adduser/noble,noble,now 3.137ubuntu1 all [installed]
apt/noble,now 2.7.14build2 amd64 [installed]
bash/noble,now 5.2.21-2ubuntu4 amd64 [installed]
`
	expected := []Package{
		{Name: "adduser", Manager: "apt", Version: "3.137ubuntu1"},
		{Name: "apt", Manager: "apt", Version: "2.7.14build2"},
		{Name: "bash", Manager: "apt", Version: "5.2.21-2ubuntu4"},
	}

	result := parsePackages("apt", output)

	if len(result) != len(expected) {
		t.Fatalf("Expected %d packages, got %d", len(expected), len(result))
	}

	for i, pkg := range result {
		if pkg != expected[i] {
			t.Errorf("Package %d mismatch:\nExpected: %+v\nGot:      %+v", i, expected[i], pkg)
		}
	}
}

func TestParsePackages_Empty(t *testing.T) {
	output := ""
	result := parsePackages("apt", output)
	if len(result) != 0 {
		t.Errorf("Expected 0 packages for empty output, got %d", len(result))
	}
}
