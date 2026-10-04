package replay_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

const (
	matcherAuthorization = "Bearer synthetic-token"
	matcherTarget        = "https://example.invalid/items/a%2Fb?tag=one&tag=two"
)

func TestMatcherNegativeControls(t *testing.T) {
	t.Parallel()

	expected := requestExpectation{
		Method: http.MethodPost, Origin: "https://example.invalid", Path: "/items/a%2Fb",
		Query:   url.Values{"tag": {"one", "two"}},
		Headers: http.Header{"Authorization": {matcherAuthorization}, "Content-Type": {"application/json"}},
		Body:    `{"name":"synthetic"}`,
	}

	cases := []struct{ name, target, method, body, authorization string }{
		{"method", matcherTarget, http.MethodGet, expected.Body, matcherAuthorization},
		{"origin", "https://unlisted.invalid/items/a%2Fb?tag=one&tag=two",
			http.MethodPost, expected.Body, matcherAuthorization},
		{"escaped path",
			"https://example.invalid/items/a/b?tag=one&tag=two",
			http.MethodPost,
			expected.Body,
			matcherAuthorization},
		{"missing repeated query",
			"https://example.invalid/items/a%2Fb?tag=one",
			http.MethodPost,
			expected.Body,
			matcherAuthorization},
		{"extra query",
			"https://example.invalid/items/a%2Fb?tag=one&tag=two&extra=yes",
			http.MethodPost,
			expected.Body,
			matcherAuthorization},
		{"header", matcherTarget, http.MethodPost, expected.Body, "Bearer other-token"},
		{"body", matcherTarget, http.MethodPost,
			`{"name":"changed"}`, matcherAuthorization},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			request, err := http.NewRequestWithContext(t.Context(), test.method, test.target, strings.NewReader(test.body))
			if err != nil {
				t.Fatal(err)
			}

			request.Header.Set("Authorization", test.authorization)
			request.Header.Set("Content-Type", "application/json")

			transport := newPairedTransport([]exchange{{Request: expected,
				Response: responseExpectation{Status: 200,
					Headers: nil,
					Body:    "{}"}}})

			response, err := transport.RoundTrip(request)

			if response != nil {
				closeErr := response.Body.Close()
				if closeErr != nil {
					t.Error(closeErr)
				}
			}

			if err == nil || response != nil || transport.next != 0 {
				t.Fatal("mismatch supplied fallback response or consumed pair")
			}

			if transport.complete() == nil {
				t.Fatal("unconsumed exchange accepted")
			}
		})
	}
}

func TestMatcherRejectsDuplicate(t *testing.T) {
	t.Parallel()

	expected := requestExpectation{Method: http.MethodGet,
		Origin:  "https://example.invalid",
		Path:    "/items",
		Query:   nil,
		Headers: nil,
		Body:    ""}
	transport := newPairedTransport([]exchange{{Request: expected,
		Response: responseExpectation{Status: 200,
			Headers: nil,
			Body:    "{}"}}})

	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.invalid/items", nil)
	if err != nil {
		t.Fatal(err)
	}

	response, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatal(err)
	}

	err = response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = transport.complete()
	if err != nil {
		t.Fatal(err)
	}

	response, err = transport.RoundTrip(request)

	if response != nil {
		closeErr := response.Body.Close()
		if closeErr != nil {
			t.Error(closeErr)
		}
	}

	if err == nil || response != nil {
		t.Fatal("duplicate received response")
	}
}
