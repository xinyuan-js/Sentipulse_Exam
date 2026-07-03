package version

import "testing"

func TestSatisfiesVersion(t *testing.T) {
	tests := []struct {
		name       string
		version    string
		constraint string
		want       bool
	}{
		{name: "empty constraint", version: "1.2.0", constraint: "", want: true},
		{name: "implicit equality", version: "1.2.0", constraint: "1.2", want: true},
		{name: "greater than or equal", version: "1.2.1", constraint: ">=1.2.0", want: true},
		{name: "less than", version: "1.9.0", constraint: "<2.0.0", want: true},
		{name: "not satisfied", version: "1.9.0", constraint: ">=2.0.0", want: false},
		{name: "v prefix", version: "v2.0.0", constraint: ">=1.9.0", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Satisfies(tt.version, tt.constraint)
			if err != nil {
				t.Fatalf("Satisfies returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("Satisfies(%q, %q) = %v, want %v", tt.version, tt.constraint, got, tt.want)
			}
		})
	}
}

func TestSatisfiesVersionRejectsInvalidConstraint(t *testing.T) {
	if _, err := Satisfies("1.0.0", ">="); err == nil {
		t.Fatal("expected invalid constraint error")
	}
}

func TestValidateConstraintRejectsInvalidVersion(t *testing.T) {
	if err := ValidateConstraint(">=banana"); err == nil {
		t.Fatal("expected invalid version error")
	}
}
