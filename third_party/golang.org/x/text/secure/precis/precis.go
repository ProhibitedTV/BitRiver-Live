package precis

// Profile represents a text preparation profile. The implementation here is a
// minimal stub that treats inputs as opaque strings.
type Profile struct{}

// OpaqueString approximates the behaviour of the real precis profile used by
// SCRAM authentication by returning the supplied input without modification.
var OpaqueString Profile

// Bytes copies the provided byte slice and returns it unchanged.
func (Profile) Bytes(b []byte) ([]byte, error) {
	out := make([]byte, len(b))
	copy(out, b)
	return out, nil
}

// String returns the supplied string without modification.
func (Profile) String(s string) (string, error) {
	return s, nil
}
