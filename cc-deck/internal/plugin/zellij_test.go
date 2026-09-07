package plugin

import "testing"

func TestCheckCompatibility(t *testing.T) {
	cases := []struct {
		version string
		want    string
	}{
		{"0.44.3", "incompatible"}, // below the minimum: could load the controller twice
		{"0.40.0", "incompatible"},
		{"0.45.0", "compatible"},
		{"0.45.1", "compatible"},
		{"0.46.0", "untested"}, // newer than anything verified
		{"1.0.0", "untested"},
		{"garbage", "untested"},
		{"", "untested"},
	}
	for _, c := range cases {
		if got := CheckCompatibility(c.version, MaxTestedZellijVersion); got != c.want {
			t.Errorf("CheckCompatibility(%q) = %q, want %q", c.version, got, c.want)
		}
	}
}

func TestEmbeddedPluginVersionBounds(t *testing.T) {
	info := EmbeddedPlugin()
	if info.MinZellij != MinZellijVersion {
		t.Errorf("MinZellij = %q, want %q", info.MinZellij, MinZellijVersion)
	}
	if info.MaxTested != MaxTestedZellijVersion {
		t.Errorf("MaxTested = %q, want %q", info.MaxTested, MaxTestedZellijVersion)
	}
	if CheckCompatibility(info.MinZellij+".0", info.MaxTested) != "compatible" {
		t.Errorf("the minimum version must itself be compatible")
	}
}
