package config

import "testing"

func TestOffensiveModeParsing(t *testing.T) {
	cases := []struct {
		env  string
		want bool
	}{
		{"", false},
		{"default", false},
		{"offensive", true},
		{"OFFENSIVE", true},
		{" offensive ", true},
		{"1", false},
	}
	for _, c := range cases {
		t.Setenv(ModeEnv, c.env)
		if got := Offensive(); got != c.want {
			t.Errorf("Offensive() with %s=%q = %v, want %v", ModeEnv, c.env, got, c.want)
		}
	}
}
