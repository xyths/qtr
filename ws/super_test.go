package ws

import "testing"

func Test_getClientOrderId(t *testing.T) {
	var tests = []struct {
		sep, prefix         string
		short, long, unique int64
		result              string
	}{
		{"", "p", 1, 2, 3, "p123"},
		{"-", "p", 1, 2, 3, "p-1-2-3"},
		{" ", "bbbb", 2, 3, 100000000, "bbbb 2 3 100000000"},
	}
	for i, tt := range tests {
		got := getClientOrderId(tt.sep, tt.prefix, tt.short, tt.long, tt.unique)
		if tt.result != got {
			t.Errorf("[%d] want %s, got %s", i, tt.result, got)
		}
	}
}
