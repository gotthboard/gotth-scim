package scim

import "testing"

func FuzzDecodeDocument(fuzzer *testing.F) {
	for _, seed := range [][]byte{[]byte(`{}`), []byte(`{"userName":"member"}`), []byte(`{"a":1,"A":2}`), nil} {
		fuzzer.Add(seed)
	}
	fuzzer.Fuzz(func(t *testing.T, raw []byte) {
		document, err := DecodeDocument(raw)
		if err != nil {
			return
		}
		if document == nil {
			t.Fatal("successful decode returned nil")
		}
		if _, err := canonicalDocument(document); err != nil {
			t.Fatalf("successful document could not be canonicalized: %v", err)
		}
	})
}

func FuzzDecodePatch(fuzzer *testing.F) {
	fuzzer.Add([]byte(`{"schemas":["` + PatchSchema + `"],"Operations":[{"op":"replace","path":"active","value":true}]}`))
	fuzzer.Add([]byte(`{}`))
	fuzzer.Fuzz(func(t *testing.T, raw []byte) {
		request, err := DecodePatch(raw, 100)
		if err == nil && (len(request.Operations) == 0 || len(request.Operations) > 100) {
			t.Fatalf("successful PATCH crossed operation boundary: %d", len(request.Operations))
		}
	})
}

func FuzzDecodeBulk(fuzzer *testing.F) {
	fuzzer.Add([]byte(`{"schemas":["` + BulkRequestSchema + `"],"Operations":[{"method":"POST","bulkId":"one","path":"/Users","data":{}}]}`))
	fuzzer.Add([]byte(`{}`))
	fuzzer.Fuzz(func(t *testing.T, raw []byte) {
		request, err := DecodeBulk(raw)
		if err == nil && (len(request.Operations) == 0 || len(request.Operations) > MaximumBulkOperations) {
			t.Fatalf("successful Bulk crossed operation boundary: %d", len(request.Operations))
		}
	})
}

func FuzzParseFilter(fuzzer *testing.F) {
	for _, seed := range []string{`userName eq "member"`, `emails[type eq "work" and value co "@"]`, `not (active eq false)`, ""} {
		fuzzer.Add(seed)
	}
	definition := DefaultDefinitions()[0]
	fuzzer.Fuzz(func(t *testing.T, raw string) {
		expression, err := ParseFilter(raw, definition)
		if err != nil {
			return
		}
		if _, err := MatchFilter(expression, Document{"schemas": []any{UserSchema}, "userName": "member"}); err != nil {
			t.Fatalf("validated filter failed evaluation: %v", err)
		}
	})
}

func FuzzDecodeSearchRequest(fuzzer *testing.F) {
	fuzzer.Add([]byte(`{"schemas":["` + SearchRequestSchema + `"],"filter":"userName pr"}`))
	fuzzer.Add([]byte(`{}`))
	fuzzer.Fuzz(func(t *testing.T, raw []byte) { _, _ = decodeSearchRequest(raw) })
}
