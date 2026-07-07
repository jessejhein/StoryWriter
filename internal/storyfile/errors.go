package storyfile

import "errors"

// ErrInvalidCanonicalState reports malformed or relationship-invalid canonical
// story files.
var ErrInvalidCanonicalState = errors.New("invalid canonical state")

func invalidCanonical(err error) error {
	if err == nil {
		return nil
	}
	return errors.Join(ErrInvalidCanonicalState, err)
}
