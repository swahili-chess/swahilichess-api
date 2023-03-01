package validator

import (
	"regexp"
)

// email regex
var (
	EmailRX = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+\\/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")
)



type Validator struct {
	Errors map[string]string
}


// Creates a new validator
func New() *Validator {
	return &Validator{Errors: make(map[string]string)}
}


// Checks if there is no errors  Validator Errors field
func (v *Validator) Valid() bool {
	return len(v.Errors) == 0
}


// Adds error if it doesn't current exists in error field in Validator struct
func (v *Validator) AddError(key, message string) {
	if _, exists := v.Errors[key]; !exists {
		v.Errors[key] = message
	}
}


// Uses AddError to add error when the condition is false.
func (v *Validator) Check(ok bool, key, message string) {
	if !ok {
		v.AddError(key, message)
	}
}


// Check the value if its in the List.
func In(value string, list ...string) bool {
	for i := range list {
		if value == list[i] {
			return true
		}
	}
	return false

}


// Matches the value with regex
func Matches(value string, rx *regexp.Regexp) bool {
	return rx.MatchString(value)
}
