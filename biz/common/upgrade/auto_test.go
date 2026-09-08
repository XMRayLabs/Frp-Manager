package upgrade

import "testing"

func TestIsMacAppBundlePath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: "/Applications/frp-manager Client.app/Contents/Resources/frpp", want: true},
		{path: "/applications/FRP.APP/contents/MacOS/frp-manager", want: true},
		{path: "/usr/local/libexec/frp-manager/frpp", want: false},
	}
	for _, test := range tests {
		if got := isMacAppBundlePath(test.path); got != test.want {
			t.Fatalf("isMacAppBundlePath(%q) = %v, want %v", test.path, got, test.want)
		}
	}
}
