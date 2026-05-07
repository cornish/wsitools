package codec

import (
	"errors"
	"testing"
)

func TestRegistryRegisterAndLookup(t *testing.T) {
	resetRegistryForTesting()

	fac := &fakeFactory{name: "fake"}
	Register(fac)

	got, err := Lookup("fake")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if got != fac {
		t.Errorf("Lookup returned different factory")
	}
}

func TestLookupUnknown(t *testing.T) {
	resetRegistryForTesting()
	_, err := Lookup("nope")
	if !errors.Is(err, ErrUnknownCodec) {
		t.Errorf("err: got %v, want ErrUnknownCodec", err)
	}
}

type fakeFactory struct{ name string }

func (f *fakeFactory) Name() string { return f.name }
func (f *fakeFactory) NewEncoder(LevelGeometry, Quality) (Encoder, error) {
	return nil, errors.New("not implemented")
}
