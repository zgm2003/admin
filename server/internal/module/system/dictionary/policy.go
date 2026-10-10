package dictionary

import (
	"fmt"

	"admin/server/internal/shared/yesno"
)

const fixedGenderDictionaryCode = "user.gender"

var errFixedDictionaryValue = fmt.Errorf("%w: dictionary value is fixed by the business protocol", ErrConflict)

func isFixedValueDictionary(value Dictionary) bool {
	return value.IsBuiltin == yesno.Yes && value.Code == fixedGenderDictionaryCode
}

func validateFixedValueCreate(parent Dictionary, value string) error {
	if !isFixedValueDictionary(parent) {
		return nil
	}
	if value != "0" && value != "1" && value != "2" {
		return fmt.Errorf("%w: %s", errFixedDictionaryValue, fixedGenderDictionaryCode)
	}
	return nil
}

func validateFixedDictionaryStatus(parent Dictionary, status yesno.Value) error {
	if isFixedValueDictionary(parent) && status == yesno.No {
		return fmt.Errorf("%w: cannot disable %s", errFixedDictionaryValue, parent.Code)
	}
	return nil
}

func validateFixedValueStatus(parent Dictionary, item Item, status yesno.Value) error {
	if isFixedValueDictionary(parent) && status == yesno.No {
		return fmt.Errorf("%w: cannot disable %s", errFixedDictionaryValue, item.Value)
	}
	return nil
}

func validateFixedValueDelete(parent Dictionary, item Item) error {
	if isFixedValueDictionary(parent) {
		return fmt.Errorf("%w: cannot delete %s", errFixedDictionaryValue, item.Value)
	}
	return nil
}
