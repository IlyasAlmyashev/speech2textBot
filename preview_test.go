package main

import "testing"

func TestCreatePreview(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
		ok   bool
	}{
		{
			name: "instagram keeps url details",
			text: "https://www.instagram.com/xxx/?utm_source=test#post",
			want: "here's your preview : https://oginstagram.com/xxx/?utm_source=test#post",
			ok:   true,
		},
		{
			name: "x and twitter",
			text: "X: https://www.x.com/x and Twitter: https://twitter.com/user/status/1",
			want: "here's your preview : X: https://www.fixupx.com/x and Twitter: https://www.fixupx.com/user/status/1",
			ok:   true,
		},
		{name: "unsupported link", text: "https://example.com/post"},
		{
			name: "domain at the end of the text",
			text: "see https://x.com",
			want: "here's your preview : see https://www.fixupx.com",
			ok:   true,
		},
		{
			name: "case insensitive",
			text: "HTTP://WWW.INSTAGRAM.COM/p/1",
			want: "here's your preview : https://oginstagram.com/p/1",
			ok:   true,
		},
		{name: "lookalike domain", text: "https://x.com.evil.org/a https://instagram.community/b"},
		{name: "subdomain of other site", text: "https://notx.com/a"},
		{name: "plain text", text: "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := createPreview(tt.text)
			if ok != tt.ok || got != tt.want {
				t.Errorf("createPreview(%q) = %q, %v; want %q, %v", tt.text, got, ok, tt.want, tt.ok)
			}
		})
	}
}
