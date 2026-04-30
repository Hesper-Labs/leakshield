package provider

import (
	"context"
	"testing"
)

type fakeProvider struct{ name string }

func (f *fakeProvider) Name() string                                    { return f.name }
func (f *fakeProvider) Routes() []Route                                 { return nil }
func (f *fakeProvider) SupportedModels() []string                       { return nil }
func (f *fakeProvider) Forward(context.Context, *PassthroughRequest, *ProviderKey) (*PassthroughResponse, error) {
	return nil, nil
}
func (f *fakeProvider) Stream(context.Context, *PassthroughRequest, *ProviderKey) (StreamReader, error) {
	return nil, nil
}
func (f *fakeProvider) ExtractMessages(*PassthroughRequest) ([]Message, error)  { return nil, nil }
func (f *fakeProvider) InjectMessages(*PassthroughRequest, []Message) error     { return nil }
func (f *fakeProvider) CountTokens(*PassthroughRequest) (int, error)            { return 0, nil }

func TestRegisterAndLookup(t *testing.T) {
	defer reset()
	reset()

	Register("fake", func() (Provider, error) {
		return &fakeProvider{name: "fake"}, nil
	})

	p, err := Lookup("fake")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if p.Name() != "fake" {
		t.Fatalf("got %q, want %q", p.Name(), "fake")
	}

	if _, err := Lookup("nope"); err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestFromPath(t *testing.T) {
	defer reset()
	reset()

	Register("openai", func() (Provider, error) {
		return &fakeProvider{name: "openai"}, nil
	})
	Register("anthropic", func() (Provider, error) {
		return &fakeProvider{name: "anthropic"}, nil
	})

	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"/openai/v1/chat/completions", "openai", false},
		{"/anthropic/v1/messages", "anthropic", false},
		{"/unknown/foo", "", true},
		{"/", "", true},
		{"", "", true},
	}
	for _, c := range cases {
		got, err := FromPath(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("FromPath(%q): expected error", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("FromPath(%q): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("FromPath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRegisterDuplicatePanics(t *testing.T) {
	defer reset()
	reset()

	Register("dup", func() (Provider, error) { return &fakeProvider{name: "dup"}, nil })

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()
	Register("dup", func() (Provider, error) { return &fakeProvider{name: "dup"}, nil })
}
