package version

import (
	"fmt"
	"strconv"
	"strings"
)

type versionOperator string

const (
	opEqual              versionOperator = "="
	opGreaterThan        versionOperator = ">"
	opGreaterThanOrEqual versionOperator = ">="
	opLessThan           versionOperator = "<"
	opLessThanOrEqual    versionOperator = "<="
)

func Satisfies(version, constraint string) (bool, error) {
	constraint = strings.TrimSpace(constraint)
	if constraint == "" {
		return true, nil
	}

	operator, expected, err := parseConstraint(constraint)
	if err != nil {
		return false, err
	}

	comparison, err := compareVersion(version, expected)
	if err != nil {
		return false, err
	}

	switch operator {
	case opEqual:
		return comparison == 0, nil
	case opGreaterThan:
		return comparison > 0, nil
	case opGreaterThanOrEqual:
		return comparison >= 0, nil
	case opLessThan:
		return comparison < 0, nil
	case opLessThanOrEqual:
		return comparison <= 0, nil
	default:
		return false, fmt.Errorf("unsupported operator %q", operator)
	}
}

func ValidateConstraint(constraint string) error {
	if strings.TrimSpace(constraint) == "" {
		return nil
	}
	_, expected, err := parseConstraint(constraint)
	if err != nil {
		return err
	}
	_, err = parseVersion(expected)
	return err
}

func parseConstraint(input string) (versionOperator, string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return opEqual, "", fmt.Errorf("empty version constraint")
	}

	for _, candidate := range []versionOperator{opGreaterThanOrEqual, opLessThanOrEqual, opGreaterThan, opLessThan, opEqual} {
		prefix := string(candidate)
		if strings.HasPrefix(input, prefix) {
			version := strings.TrimSpace(strings.TrimPrefix(input, prefix))
			if version == "" {
				return opEqual, "", fmt.Errorf("missing version after %q", prefix)
			}
			return candidate, version, nil
		}
	}

	return opEqual, input, nil
}

func compareVersion(left, right string) (int, error) {
	leftParts, err := parseVersion(left)
	if err != nil {
		return 0, err
	}
	rightParts, err := parseVersion(right)
	if err != nil {
		return 0, err
	}

	maxLen := len(leftParts)
	if len(rightParts) > maxLen {
		maxLen = len(rightParts)
	}

	for i := 0; i < maxLen; i++ {
		leftValue := 0
		if i < len(leftParts) {
			leftValue = leftParts[i]
		}
		rightValue := 0
		if i < len(rightParts) {
			rightValue = rightParts[i]
		}
		if leftValue > rightValue {
			return 1, nil
		}
		if leftValue < rightValue {
			return -1, nil
		}
	}
	return 0, nil
}

func parseVersion(version string) ([]int, error) {
	version = strings.TrimSpace(strings.TrimPrefix(version, "v"))
	if version == "" {
		return nil, fmt.Errorf("empty version")
	}

	main := strings.Split(version, "-")[0]
	segments := strings.Split(main, ".")
	parts := make([]int, 0, len(segments))
	for _, segment := range segments {
		if segment == "" {
			return nil, fmt.Errorf("invalid version %q", version)
		}
		value, err := strconv.Atoi(segment)
		if err != nil {
			return nil, fmt.Errorf("invalid version segment %q: %w", segment, err)
		}
		parts = append(parts, value)
	}
	return parts, nil
}
