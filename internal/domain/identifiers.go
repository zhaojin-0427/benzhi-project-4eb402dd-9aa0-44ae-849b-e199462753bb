package domain

import "regexp"

var identifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

func ValidateIdentifier(field, value string) error {
	if !identifierPattern.MatchString(value) {
		return FieldError(field, "标识只能包含字母、数字、点、下划线或连字符，且长度不超过 128")
	}
	return nil
}
