package httpserver

import "testing"

func TestSingleRange(t *testing.T) {
	tests := []struct {
		value            string
		size, start, end int64
		ok               bool
	}{{"bytes=0-9", 100, 0, 9, true}, {"bytes=0-", 100, 0, 99, true}, {"bytes=90-200", 100, 90, 99, true}, {"bytes=-10", 100, 90, 99, true}, {"bytes=-200", 100, 0, 99, true}, {"bytes=99-99", 100, 99, 99, true}, {"bytes=100-", 100, 0, 0, false}, {"bytes=8-2", 100, 0, 0, false}, {"bytes=0-1,4-5", 100, 0, 0, false}, {"bytes=-0", 100, 0, 0, false}, {"bytes=0-0", 0, 0, 0, false}, {"nope", 100, 0, 0, false}}
	for _, x := range tests {
		t.Run(x.value, func(t *testing.T) {
			a, b, ok := singleRange(x.value, x.size)
			if a != x.start || b != x.end || ok != x.ok {
				t.Fatalf("got %d %d %v", a, b, ok)
			}
		})
	}
}

func TestSignedHLSChildrenIncludeMediaToken(t *testing.T) {
	if got := appendMediaToken("segment-000001.m4s", "a+b"); got != "segment-000001.m4s?token=a%2Bb" {
		t.Fatalf("segment URI = %q", got)
	}
	if got := appendHLSAttributeToken(`#EXT-X-MAP:URI="init.mp4"`, "a+b"); got != `#EXT-X-MAP:URI="init.mp4?token=a%2Bb"` {
		t.Fatalf("map URI = %q", got)
	}
}
