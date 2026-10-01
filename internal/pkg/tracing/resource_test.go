package tracing

import "testing"

// The resource merge fails when the SDK's default resource and Parashift's
// attributes use different semantic-conventions schemas; Init would then
// refuse to start tracing. Keep the semconv import in step with the SDK.
func TestNewResource_MergesWithTheSDKDefault(t *testing.T) {
	res, err := newResource(Config{ServiceName: "parashift-backend", ServiceVersion: "v1.2.3", Environment: "production"})
	if err != nil {
		t.Fatalf("resource merge failed (semconv schema out of step with the SDK?): %v", err)
	}
	got := map[string]string{}
	for _, kv := range res.Attributes() {
		got[string(kv.Key)] = kv.Value.String()
	}
	for k, want := range map[string]string{
		"service.name":                "parashift-backend",
		"service.version":             "v1.2.3",
		"deployment.environment.name": "production",
	} {
		if got[k] != want {
			t.Errorf("%s = %q, want %q", k, got[k], want)
		}
	}
}
