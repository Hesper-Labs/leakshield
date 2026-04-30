package openai

// SetTestBaseURL points every Provider returned by the registry factory
// at base instead of api.openai.com. It is a no-op in production code;
// the hook exists so cross-package handler tests can run against an
// httptest.Server without depending on internal constructors.
//
// Tests should reset this with SetTestBaseURL("") in t.Cleanup.
func SetTestBaseURL(base string) {
	testBaseURL = base
}

var testBaseURL string
