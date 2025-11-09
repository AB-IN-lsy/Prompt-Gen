package prompt

import "testing"

func TestExtractShareEncodedSegment(t *testing.T) {
	svc := &Service{sharePrefix: "PGSHARE", shareMaxEncodedLen: 64}
	encoded := "QUJDREVGRw"
	cases := []struct {
		name    string
		payload string
		expect  string
		wantErr error
	}{
		{name: "plain", payload: "PGSHARE-" + encoded, expect: encoded},
		{name: "with sentence", payload: "复制这段 PGSHARE-" + encoded + " 即可导入", expect: encoded},
		{name: "trim spaces", payload: "  PGSHARE-" + encoded + "  ", expect: encoded},
	}
	for _, cs := range cases {
		cs := cs
		t.Run(cs.name, func(t *testing.T) {
			got, err := svc.extractShareEncodedSegment(cs.payload)
			if cs.wantErr != nil {
				if err == nil || err != cs.wantErr {
					t.Fatalf("expected error %v, got %v", cs.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != cs.expect {
				t.Fatalf("expected %s, got %s", cs.expect, got)
			}
		})
	}

	limitSvc := &Service{sharePrefix: "PGSHARE", shareMaxEncodedLen: 4}
	if _, err := limitSvc.extractShareEncodedSegment("PGSHARE-QUJDREVGRw"); err != ErrSharePayloadTooLarge {
		t.Fatalf("expected ErrSharePayloadTooLarge, got %v", err)
	}
}
