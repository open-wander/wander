package clientspartials

import "testing"

func TestBytesPercentageAvoidsOverflow(t *testing.T) {
	maxUint64 := ^uint64(0)
	for _, test := range []struct {
		used  uint64
		total uint64
		want  uint64
	}{
		{used: 0, total: 0, want: 0},
		{used: 1, total: 4, want: 25},
		{used: maxUint64 - 1, total: maxUint64, want: 99},
		{used: maxUint64, total: maxUint64, want: 100},
	} {
		if got := bytesPercentage(test.used, test.total); got != test.want {
			t.Fatalf("bytesPercentage(%d, %d) = %d, want %d", test.used, test.total, got, test.want)
		}
	}
}
