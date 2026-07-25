package auth_setting

import "testing"

func TestParseDashboardSessionLifetimeDays(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    int
		wantErr bool
	}{
		{name: "minimum", raw: "1", want: 1},
		{name: "default", raw: "30", want: 30},
		{name: "maximum", raw: "3650", want: 3650},
		{name: "zero", raw: "0", wantErr: true},
		{name: "negative", raw: "-1", wantErr: true},
		{name: "too large", raw: "3651", wantErr: true},
		{name: "non numeric", raw: "forever", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDashboardSessionLifetimeDays(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseDashboardSessionLifetimeDays(%q) unexpectedly succeeded with %d", tt.raw, got)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("ParseDashboardSessionLifetimeDays(%q) = (%d, %v), want (%d, nil)", tt.raw, got, err, tt.want)
			}
		})
	}
}
