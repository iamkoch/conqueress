package guid

import (
	"testing"
)

func TestFromStringFails(t *testing.T) {
	testData := []struct {
		input         string
		expectedError string
	}{
		{"", "invalid guid provided to FromString: xid: invalid ID"},
		{"abck2", "invalid guid provided to FromString: xid: invalid ID"},
	}

	for _, td := range testData {
		if _, err := FromString(td.input); err.Error() != td.expectedError {
			t.Errorf("expected error %s, got %s", td.expectedError, err.Error())
		}
	}
}

func TestMustFromString(t *testing.T) {
	testDate := []struct {
		input                 string
		shouldPanic           bool
		errorMessageWhenPanic string
	}{
		{"", true, "invalid guid provided to FromString"},
		{"abck2", true, "invalid guid provided to FromString"},
		{"cfqd9lq87d5j0f24jh30", false, ""},
	}

	for _, td := range testDate {
		if td.shouldPanic {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("expected panic, but did not")
				}
			}()
			MustFromString(td.input)
		} else {
			result := MustFromString(td.input)
			if result.String() != td.input {
				t.Errorf("expected %s, got %s", td.input, result.String())
			}
		}
	}
}
