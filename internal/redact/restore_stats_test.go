package redact

import "testing"

func TestRestorationDiagnostics(t *testing.T) {
	ctx := NewContext(10)
	if ctx.RestoreStatus() != "no_sensitive_data" {
		t.Fatal(ctx.RestoreStatus())
	}
	token, err := ctx.RedactText("alice@example.com", DetectorFlags{Email: true})
	if err != nil {
		t.Fatal(err)
	}
	ctx.RestoreText("Example: {{RG_TYPE_TOKEN}}")
	if ctx.RestoreStatus() != "no_placeholders" || ctx.UnresolvedCount() != 0 {
		t.Fatal("illustrative syntax counted as an unresolved token")
	}
	for i := 0; i < 3; i++ {
		ctx.RestoreText(token)
	}
	if ctx.RestoreCount() != 3 || ctx.RestoreUniqueCount() != 1 || ctx.RestoreStatus() != "restored" {
		t.Fatalf("counts = %d / %d, status = %s", ctx.RestoreCount(), ctx.RestoreUniqueCount(), ctx.RestoreStatus())
	}
	unknown := "{{RG_EMAIL_ABCDEFGHIJKLMNOP}}"
	if unknown == token {
		unknown = "{{RG_EMAIL_QRSTUVWXYZABCDEF}}"
	}
	if ctx.RestoreText(unknown) != unknown || ctx.UnresolvedCount() != 1 || ctx.RestoreStatus() != "partial" {
		t.Fatal("unknown token was not reported")
	}
	other := NewContext(10)
	other.RestoreText(unknown)
	if other.RestoreStatus() != "unresolved" || other.RestoreCount() != 0 {
		t.Fatal("unknown-only response was not reported")
	}
}
