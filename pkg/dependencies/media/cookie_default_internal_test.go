package media

import (
	"net/http"
	"net/http/cookiejar"
	"testing"
)

//nolint:paralleltest // Mutates the process-global default client to verify SDK isolation, then restores it.
func TestDefaultIgnoresGlobalCookieJar(t *testing.T) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}

	original := http.DefaultClient

	var global http.Client

	global.Jar = jar
	http.DefaultClient = &global

	t.Cleanup(func() { http.DefaultClient = original })

	client, err := New()
	if err != nil {
		t.Fatal(err)
	}

	standard, ok := client.httpClient.(*http.Client)
	if !ok || standard.Jar != nil {
		t.Fatal("media inherited the global cookie jar")
	}
}
