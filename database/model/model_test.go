package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestInboundMarshalJSONNestsObjectFields(t *testing.T) {
	in := Inbound{
		Id:             7,
		Protocol:       VLESS,
		Port:           443,
		Settings:       `{"clients":[],"decryption":"none"}`,
		StreamSettings: `{"network":"tcp"}`,
		Sniffing:       `{"enabled":true}`,
	}
	out, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	for _, field := range []string{"settings", "streamSettings", "sniffing"} {
		if _, ok := parsed[field].(map[string]any); !ok {
			t.Errorf("expected %s to marshal as a JSON object, got %T", field, parsed[field])
		}
	}
	if strings.Contains(string(out), `"settings":"`) {
		t.Errorf("settings should not be emitted as a JSON string: %s", out)
	}
}

func TestInboundMarshalJSONEmptyFieldsBecomeNull(t *testing.T) {
	in := Inbound{Id: 1, Protocol: VLESS}
	out, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	for _, field := range []string{"settings", "streamSettings", "sniffing"} {
		if parsed[field] != nil {
			t.Errorf("expected %s to be null, got %v", field, parsed[field])
		}
	}
}

func TestInboundUnmarshalJSONAcceptsBothShapes(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{
			name: "nested objects (modern)",
			body: `{"id":1,"settings":{"clients":[],"decryption":"none"},"streamSettings":{"network":"tcp"},"sniffing":{"enabled":true}}`,
		},
		{
			name: "JSON-encoded strings (legacy)",
			body: `{"id":1,"settings":"{\"clients\":[],\"decryption\":\"none\"}","streamSettings":"{\"network\":\"tcp\"}","sniffing":"{\"enabled\":true}"}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var in Inbound
			if err := json.Unmarshal([]byte(tc.body), &in); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
			if !strings.Contains(in.Settings, `"decryption":"none"`) {
				t.Errorf("Settings not normalised: %q", in.Settings)
			}
			if !strings.Contains(in.StreamSettings, `"network":"tcp"`) {
				t.Errorf("StreamSettings not normalised: %q", in.StreamSettings)
			}
			if !strings.Contains(in.Sniffing, `"enabled":true`) {
				t.Errorf("Sniffing not normalised: %q", in.Sniffing)
			}
		})
	}
}

func TestInboundMarshalJSONInvalidTextFallsBackToString(t *testing.T) {
	in := Inbound{Id: 1, Settings: "not json at all"}
	out, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if !strings.Contains(string(out), `"settings":"not json at all"`) {
		t.Errorf("expected invalid settings text to be wrapped as a JSON string, got %s", out)
	}
}

func TestClientRecordMarshalJSONNestsReverse(t *testing.T) {
	rec := ClientRecord{Id: 1, Email: "alice@example.com", Reverse: `{"tag":"vless-in"}`}
	out, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	obj, ok := parsed["reverse"].(map[string]any)
	if !ok {
		t.Fatalf("expected reverse to marshal as a JSON object, got %T", parsed["reverse"])
	}
	if obj["tag"] != "vless-in" {
		t.Errorf("expected tag to be preserved, got %v", obj["tag"])
	}
}

func TestClientRecordMarshalJSONEmptyReverseIsNull(t *testing.T) {
	rec := ClientRecord{Id: 1, Email: "alice@example.com"}
	out, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if parsed["reverse"] != nil {
		t.Errorf("expected reverse to be null, got %v", parsed["reverse"])
	}
}

func TestClientRecordUnmarshalJSONAcceptsBothShapes(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "nested object", body: `{"id":1,"reverse":{"tag":"vless-in"}}`},
		{name: "legacy string", body: `{"id":1,"reverse":"{\"tag\":\"vless-in\"}"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var rec ClientRecord
			if err := json.Unmarshal([]byte(tc.body), &rec); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
			if !strings.Contains(rec.Reverse, `"tag":"vless-in"`) {
				t.Errorf("Reverse not normalised: %q", rec.Reverse)
			}
		})
	}
}

func TestInboundClientIpsMarshalJSONNestsArray(t *testing.T) {
	row := InboundClientIps{Id: 1, ClientEmail: "alice@example.com", Ips: `[{"ip":"1.2.3.4","timestamp":1700000000}]`}
	out, err := json.Marshal(row)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	arr, ok := parsed["ips"].([]any)
	if !ok {
		t.Fatalf("expected ips to marshal as a JSON array, got %T", parsed["ips"])
	}
	if len(arr) != 1 {
		t.Errorf("expected 1 entry, got %d", len(arr))
	}
}

func TestInboundClientIpsUnmarshalJSONAcceptsBothShapes(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "nested array", body: `{"id":1,"ips":[{"ip":"1.2.3.4","timestamp":1}]}`},
		{name: "legacy string", body: `{"id":1,"ips":"[{\"ip\":\"1.2.3.4\",\"timestamp\":1}]"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var row InboundClientIps
			if err := json.Unmarshal([]byte(tc.body), &row); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
			if !strings.Contains(row.Ips, `"ip":"1.2.3.4"`) {
				t.Errorf("Ips not normalised: %q", row.Ips)
			}
		})
	}
}

func TestHealHysteriaALPNRewritesTcpDefaults(t *testing.T) {
	// The shape a hysteria inbound got when the preset used the TCP-TLS TLS
	// defaults: an ALPN no QUIC client can negotiate.
	stream := `{"network":"hysteria","security":"tls","tlsSettings":{"serverName":"a.example.com","alpn":["h2","http/1.1"]}}`

	healed, changed := HealHysteriaALPN(stream)
	if !changed {
		t.Fatalf("expected the h2/http1.1 default to be rewritten, got changed=false")
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(healed), &parsed); err != nil {
		t.Fatalf("healed stream is not valid JSON: %v", err)
	}
	tls, _ := parsed["tlsSettings"].(map[string]any)
	alpn, _ := tls["alpn"].([]any)
	if len(alpn) != 1 || alpn[0] != "h3" {
		t.Fatalf("alpn = %v, want [h3]", alpn)
	}
	// Everything else must survive — serverName in particular, since losing it
	// would break SNI for the very inbound we are trying to repair.
	if tls["serverName"] != "a.example.com" {
		t.Fatalf("serverName = %v, want a.example.com", tls["serverName"])
	}
	if parsed["network"] != "hysteria" || parsed["security"] != "tls" {
		t.Fatalf("stream fields lost: %v", parsed)
	}
}

func TestHealHysteriaALPNLeavesCorrectConfigUntouched(t *testing.T) {
	// Returning changed=false for an already-correct inbound keeps the config
	// bytes stable, so XrayService doesn't see a phantom diff and restart Xray.
	stream := `{"network":"hysteria","security":"tls","tlsSettings":{"alpn":["h3"]}}`
	healed, changed := HealHysteriaALPN(stream)
	if changed {
		t.Fatalf("expected no change for an already-h3 inbound")
	}
	if healed != stream {
		t.Fatalf("stream was rewritten despite changed=false: got %q, want %q", healed, stream)
	}
}

func TestHealHysteriaALPNIgnoresUnusableInput(t *testing.T) {
	cases := map[string]string{
		"empty":           "",
		"invalid json":    "{not json",
		"no tlsSettings":  `{"network":"hysteria","security":"none"}`,
		"tlsSettings nil": `{"tlsSettings":null}`,
	}
	for name, stream := range cases {
		t.Run(name, func(t *testing.T) {
			healed, changed := HealHysteriaALPN(stream)
			if changed {
				t.Fatalf("expected changed=false, got true (healed=%q)", healed)
			}
			if healed != stream {
				t.Fatalf("input was modified: got %q, want %q", healed, stream)
			}
		})
	}
}

func TestGenXrayInboundConfigHealsHysteriaALPN(t *testing.T) {
	// End to end through the actual config generator: an inbound saved before
	// the fix must come out with h3 without anyone editing it.
	in := Inbound{
		Protocol:       Hysteria,
		Port:           36715,
		Settings:       `{"version":2,"clients":[{"auth":"pw","email":"a@b.c"}]}`,
		StreamSettings: `{"network":"hysteria","security":"tls","tlsSettings":{"alpn":["h2","http/1.1"]}}`,
	}
	cfg := in.GenXrayInboundConfig()
	if !strings.Contains(string(cfg.StreamSettings), `"h3"`) {
		t.Fatalf("generated stream settings still lack h3: %s", cfg.StreamSettings)
	}
	if strings.Contains(string(cfg.StreamSettings), "http/1.1") {
		t.Fatalf("generated stream settings still carry the TCP-TLS alpn: %s", cfg.StreamSettings)
	}
}

func TestGenXrayInboundConfigLeavesOtherProtocolsAlpnAlone(t *testing.T) {
	// The heal is hysteria-only: h2/http1.1 is correct for TCP-TLS protocols
	// and rewriting it there would break them.
	in := Inbound{
		Protocol:       VLESS,
		Port:           443,
		Settings:       `{"clients":[]}`,
		StreamSettings: `{"network":"tcp","security":"tls","tlsSettings":{"alpn":["h2","http/1.1"]}}`,
	}
	cfg := in.GenXrayInboundConfig()
	if !strings.Contains(string(cfg.StreamSettings), "http/1.1") {
		t.Fatalf("vless alpn was rewritten: %s", cfg.StreamSettings)
	}
}
