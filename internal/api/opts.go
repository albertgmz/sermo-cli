package api

// PageOpts select one page of a list. A zero field is not sent.
// Cursor has priority over Page. With Limit 0 the server uses its default of 20.
type PageOpts struct {
	Page   int
	Cursor string
	Limit  int
}

func optInt(v int) *int {
	if v == 0 {
		return nil
	}
	return &v
}

func optString(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func optBool(v bool) *bool {
	if !v {
		return nil
	}
	return &v
}
