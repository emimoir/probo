// Copyright (c) 2026 Probo Inc <hello@getprobo.com>.
//
// Permission to use, copy, modify, and/or distribute this software for any
// purpose with or without fee is hereby granted, provided that the above
// copyright notice and this permission notice appear in all copies.
//
// THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES WITH
// REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF MERCHANTABILITY
// AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY SPECIAL, DIRECT,
// INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES WHATSOEVER RESULTING FROM
// LOSS OF USE, DATA OR PROFITS, WHETHER IN AN ACTION OF CONTRACT, NEGLIGENCE OR
// OTHER TORTIOUS ACTION, ARISING OUT OF OR IN CONNECTION WITH THE USE OR
// PERFORMANCE OF THIS SOFTWARE.

package coredata

import "fmt"

type (
	DocumentVersionContentSource string
)

const (
	DocumentVersionContentSourceAuthored  DocumentVersionContentSource = "AUTHORED"
	DocumentVersionContentSourceGenerated DocumentVersionContentSource = "GENERATED"
)

func (e DocumentVersionContentSource) IsValid() bool {
	switch e {
	case DocumentVersionContentSourceAuthored, DocumentVersionContentSourceGenerated:
		return true
	}
	return false
}

func (e DocumentVersionContentSource) String() string { return string(e) }

func (e *DocumentVersionContentSource) UnmarshalText(text []byte) error {
	*e = DocumentVersionContentSource(text)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid DocumentVersionContentSource", string(text))
	}
	return nil
}

func (e DocumentVersionContentSource) MarshalText() ([]byte, error) {
	return []byte(e.String()), nil
}
