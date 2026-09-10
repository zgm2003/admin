package dictionary

import "errors"

var ErrConflict = errors.New("dictionary value conflicts with an existing record")
