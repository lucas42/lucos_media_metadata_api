package predicateconfig

import (
	"os"
	"testing"
)

func TestValidateURIOriginAcceptsEolasURI(t *testing.T) {
	os.Setenv("EOLAS_ORIGIN", "https://eolas.l42.eu")
	t.Cleanup(func() { os.Unsetenv("EOLAS_ORIGIN") })
	c := Config{AllowedOrigins: []string{OriginEolas}}
	if msg := c.ValidateURIOrigin("https://eolas.l42.eu/metadata/language/en/"); msg != "" {
		t.Errorf("expected no error for valid eolas URI, got: %q", msg)
	}
}

func TestValidateURIOriginRejectsWrongOrigin(t *testing.T) {
	os.Setenv("EOLAS_ORIGIN", "https://eolas.l42.eu")
	t.Cleanup(func() { os.Unsetenv("EOLAS_ORIGIN") })
	c := Config{AllowedOrigins: []string{OriginEolas}}
	if msg := c.ValidateURIOrigin("https://example.com/metadata/language/en/"); msg == "" {
		t.Error("expected error for URI with wrong origin, got empty string")
	}
}

func TestValidateURIOriginRejectsNonHttpsScheme(t *testing.T) {
	os.Setenv("EOLAS_ORIGIN", "https://eolas.l42.eu")
	t.Cleanup(func() { os.Unsetenv("EOLAS_ORIGIN") })
	c := Config{AllowedOrigins: []string{OriginEolas}}
	// http:// does not match https:// eolas origin.
	if msg := c.ValidateURIOrigin("http://eolas.l42.eu/metadata/language/en/"); msg == "" {
		t.Error("expected error for http:// URI when origin is https://, got empty string")
	}
}

func TestValidateURIOriginSkipsWhenNoAllowlist(t *testing.T) {
	c := Config{AllowedOrigins: nil}
	if msg := c.ValidateURIOrigin("http://anything.example.com/entity/1"); msg != "" {
		t.Errorf("expected no error for config with nil AllowedOrigins, got: %q", msg)
	}
}

func TestValidateURIOriginSkipsWhenAllowlistResolvesToEmpty(t *testing.T) {
	// MEDIA_METADATA_MANAGER_ORIGIN unset → empty → filtered out → validation skipped.
	os.Unsetenv("MEDIA_METADATA_MANAGER_ORIGIN")
	c := Config{AllowedOrigins: []string{OriginMediaMetadataManager}}
	if msg := c.ValidateURIOrigin("/albums/1"); msg != "" {
		t.Errorf("expected no error when all origins resolve to empty, got: %q", msg)
	}
}

func TestRequiresURITrueForURIObject(t *testing.T) {
	c := Config{ValueShape: ValueShapeURIObject}
	if !c.RequiresURI() {
		t.Error("expected RequiresURI() true for ValueShapeURIObject")
	}
}

func TestRequiresURIFalseForLiteral(t *testing.T) {
	c := Config{ValueShape: ValueShapeLiteral}
	if c.RequiresURI() {
		t.Error("expected RequiresURI() false for ValueShapeLiteral")
	}
}

func TestRequiresURIFalseForOmit(t *testing.T) {
	c := Config{ValueShape: ValueShapeOmit}
	if c.RequiresURI() {
		t.Error("expected RequiresURI() false for ValueShapeOmit")
	}
}

func TestRequiresURIFalseForZeroValue(t *testing.T) {
	c := Config{}
	if c.RequiresURI() {
		t.Error("expected RequiresURI() false for zero-value Config (ValueShapeOmit is zero)")
	}
}
