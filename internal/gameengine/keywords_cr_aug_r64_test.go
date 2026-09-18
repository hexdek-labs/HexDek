package gameengine

import "testing"

func TestHealDamage_701_69a(t *testing.T) {
	p := &Permanent{Card: &Card{Name: "X"}, MarkedDamage: 5}
	HealDamage(p, 2)
	if p.MarkedDamage != 3 {
		t.Errorf("heal 2 of 5 marked → %d, want 3", p.MarkedDamage)
	}
	HealDamage(p, 0) // 0 = heal all
	if p.MarkedDamage != 0 {
		t.Errorf("heal all → %d, want 0", p.MarkedDamage)
	}
	p.MarkedDamage = 4
	HealDamage(p, 10) // more than marked → floors at 0
	if p.MarkedDamage != 0 {
		t.Errorf("heal 10 of 4 → %d, want 0", p.MarkedDamage)
	}
	HealDamage(nil, 1) // nil-safe, must not panic
}

func TestIsWorthy_700_16(t *testing.T) {
	cases := []struct {
		name string
		card *Card
		want bool
	}{
		{"legendary red creature", &Card{Types: []string{"legendary", "creature"}, Colors: []string{"R"}}, true},
		{"legendary white creature", &Card{Types: []string{"legendary", "creature"}, Colors: []string{"W"}}, true},
		{"legendary RW creature", &Card{Types: []string{"legendary", "creature"}, Colors: []string{"R", "W"}}, true},
		{"non-legendary red creature", &Card{Types: []string{"creature"}, Colors: []string{"R"}}, false},
		{"legendary blue creature (not R/W)", &Card{Types: []string{"legendary", "creature"}, Colors: []string{"U"}}, false},
		{"legendary red artifact (not a creature)", &Card{Types: []string{"legendary", "artifact"}, Colors: []string{"R"}}, false},
		{"nil", nil, false},
	}
	for _, tc := range cases {
		if got := IsWorthy(tc.card); got != tc.want {
			t.Errorf("%s: IsWorthy = %v, want %v", tc.name, got, tc.want)
		}
	}
}
