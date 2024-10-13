// Code generated by "enumer -type=Type -text -transform=snake -output=./stats_type.go"; DO NOT EDIT.

package stats

import (
	"fmt"
	"strings"
)

const _TypeName = "traversedmatchedformattedchanged"

var _TypeIndex = [...]uint8{0, 9, 16, 25, 32}

const _TypeLowerName = "traversedmatchedformattedchanged"

func (i Type) String() string {
	if i < 0 || i >= Type(len(_TypeIndex)-1) {
		return fmt.Sprintf("Type(%d)", i)
	}
	return _TypeName[_TypeIndex[i]:_TypeIndex[i+1]]
}

// An "invalid array index" compiler error signifies that the constant values have changed.
// Re-run the stringer command to generate them again.
func _TypeNoOp() {
	var x [1]struct{}
	_ = x[Traversed-(0)]
	_ = x[Matched-(1)]
	_ = x[Formatted-(2)]
	_ = x[Changed-(3)]
}

var _TypeValues = []Type{Traversed, Matched, Formatted, Changed}

var _TypeNameToValueMap = map[string]Type{
	_TypeName[0:9]:        Traversed,
	_TypeLowerName[0:9]:   Traversed,
	_TypeName[9:16]:       Matched,
	_TypeLowerName[9:16]:  Matched,
	_TypeName[16:25]:      Formatted,
	_TypeLowerName[16:25]: Formatted,
	_TypeName[25:32]:      Changed,
	_TypeLowerName[25:32]: Changed,
}

var _TypeNames = []string{
	_TypeName[0:9],
	_TypeName[9:16],
	_TypeName[16:25],
	_TypeName[25:32],
}

// TypeString retrieves an enum value from the enum constants string name.
// Throws an error if the param is not part of the enum.
func TypeString(s string) (Type, error) {
	if val, ok := _TypeNameToValueMap[s]; ok {
		return val, nil
	}

	if val, ok := _TypeNameToValueMap[strings.ToLower(s)]; ok {
		return val, nil
	}
	return 0, fmt.Errorf("%s does not belong to Type values", s)
}

// TypeValues returns all values of the enum
func TypeValues() []Type {
	return _TypeValues
}

// TypeStrings returns a slice of all String values of the enum
func TypeStrings() []string {
	strs := make([]string, len(_TypeNames))
	copy(strs, _TypeNames)
	return strs
}

// IsAType returns "true" if the value is listed in the enum definition. "false" otherwise
func (i Type) IsAType() bool {
	for _, v := range _TypeValues {
		if i == v {
			return true
		}
	}
	return false
}

// MarshalText implements the encoding.TextMarshaler interface for Type
func (i Type) MarshalText() ([]byte, error) {
	return []byte(i.String()), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface for Type
func (i *Type) UnmarshalText(text []byte) error {
	var err error
	*i, err = TypeString(string(text))
	return err
}
