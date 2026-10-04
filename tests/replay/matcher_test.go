package replay_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestMatcherNegativeControls(t *testing.T) {
	t.Parallel()

	expected := requestExpectation{
		Method: "POST", Origin: "https://example.invalid", Path: "/items/a%2Fb",
		Query:   url.Values{"tag": {"one", "two"}},
		Headers: http.Header{"Authorization": {"Bearer synthetic-token"}, "Content-Type": {"application/json"}},
		Body:    `{"name":"synthetic"}`,
	}

	cases := []struct{ name, target, method, body, authorization string }{
		{"method", "https://example.invalid/items/a%2Fb?tag=one&tag=two", "GET", expected.Body, "Bearer synthetic-token"},
		{"origin", "https://unlisted.invalid/items/a%2Fb?tag=one&tag=two", "POST", expected.Body, "Bearer synthetic-token"},
		{"escaped path",
			"https://example.invalid/items/a/b?tag=one&tag=two",
			"POST",
			expected.Body,
			"Bearer synthetic-token"},
		{"missing repeated query",
			"https://example.invalid/items/a%2Fb?tag=one",
			"POST",
			expected.Body,
			"Bearer synthetic-token"},
		{"extra query",
			"https://example.invalid/items/a%2Fb?tag=one&tag=two&extra=yes",
			"POST",
			expected.Body,
			"Bearer synthetic-token"},
		{"header", "https://example.invalid/items/a%2Fb?tag=one&tag=two", "POST", expected.Body, "Bearer other-token"},
		{"body", "https://example.invalid/items/a%2Fb?tag=one&tag=two", "POST",
			`{"name":"changed"}`, "Bearer synthetic-token"},
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

	expected := requestExpectation{Method: "GET",
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
