package provider

import (
	"strings"
	"testing"
)

func TestRedactClientSecret_FormEncoded(t *testing.T) {
	in := "client_id=abc&client_secret=verysecretvalue&grant_type=client_credentials"
	out := redactClientSecret(in)
	if strings.Contains(out, "verysecretvalue") {
		t.Errorf("redactClientSecret leaked secret: %s", out)
	}
	if !strings.Contains(out, "[REDACTED]") {
		t.Errorf("redactClientSecret should mark redacted value: %s", out)
	}
	// Other fields preserved.
	if !strings.Contains(out, "client_id=abc") {
		t.Errorf("redactClientSecret should preserve client_id: %s", out)
	}
	if !strings.Contains(out, "grant_type=client_credentials") {
		t.Errorf("redactClientSecret should preserve grant_type: %s", out)
	}
}

func TestRedactClientSecret_JSON(t *testing.T) {
	in := `{"error":"invalid_client","error_description":"AADSTS7000215: Invalid client secret","client_secret":"abc123def"}`
	out := redactClientSecret(in)
	if strings.Contains(out, "abc123def") {
		t.Errorf("redactClientSecret leaked JSON secret: %s", out)
	}
}

func TestRedactClientSecret_MultipleSecretFields(t *testing.T) {
	in := `access_token=foo&refresh_token=bar&api_key=baz&password=qux`
	out := redactClientSecret(in)
	for _, leaked := range []string{"foo", "bar", "baz", "qux"} {
		if strings.Contains(out, "="+leaked) {
			t.Errorf("redactClientSecret leaked %q in %s", leaked, out)
		}
	}
}

func TestRedactClientSecret_CaseInsensitive(t *testing.T) {
	in := `Client_Secret=ABCDEF`
	out := redactClientSecret(in)
	if strings.Contains(out, "ABCDEF") {
		t.Errorf("redactClientSecret should be case-insensitive: %s", out)
	}
}

func TestRedactClientSecret_NoSecretPreserved(t *testing.T) {
	in := `error="invalid_grant" not a real secret here`
	out := redactClientSecret(in)
	if out != in {
		t.Errorf("redactClientSecret altered safe string: %q -> %q", in, out)
	}
}

func TestRedactClientSecret_EmptyValue(t *testing.T) {
	in := "client_secret="
	out := redactClientSecret(in)
	// An empty value still gets the [REDACTED] marker — harmless.
	if !strings.Contains(out, "client_secret") {
		t.Errorf("redactClientSecret should preserve field name: %s", out)
	}
}
