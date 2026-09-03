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
