package validation

import "testing"

func TestRouter_FanOutAndUnregister(t *testing.T) {
	var a, b []ValidationViolation
	ua := RegisterSink(func(v ValidationViolation) { a = append(a, v) })
	ub := RegisterSink(func(v ValidationViolation) { b = append(b, v) })
	defer ub()

	LogViolation(ValidationViolation{Surface: SurfaceInvariants, Name: "X"})
	if len(a) != 1 || len(b) != 1 {
		t.Fatalf("both sinks must observe: a=%d b=%d", len(a), len(b))
	}

	ua()
	LogViolation(ValidationViolation{Surface: SurfaceLegality, Name: "Y"})
	if len(a) != 1 {
		t.Errorf("unregistered sink must stop observing; got %d", len(a))
	}
	if len(b) != 2 {
		t.Errorf("remaining sink must keep observing; got %d", len(b))
	}
}

func TestRouter_PanickingSinkIsContained(t *testing.T) {
	u1 := RegisterSink(func(ValidationViolation) { panic("broken observer") })
	defer u1()
	var got int
	u2 := RegisterSink(func(ValidationViolation) { got++ })
	defer u2()

	LogViolation(ValidationViolation{Name: "Z"}) // must not panic
	if got != 1 {
		t.Errorf("sink after the panicking one must still run; got %d", got)
	}
}

func TestRouter_NoSinksIsNoop(t *testing.T) {
	LogViolation(ValidationViolation{Name: "lonely"}) // must not panic
	LogViolations([]ValidationViolation{{Name: "a"}, {Name: "b"}})
}
