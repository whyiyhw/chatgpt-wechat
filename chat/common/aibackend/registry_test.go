package aibackend

import (
	"context"
	"errors"
	"testing"
)

type fakeBackend struct {
	name string
}

func (f *fakeBackend) Name() string { return f.name }
func (f *fakeBackend) Type() string { return "fake" }
func (f *fakeBackend) OpenConversation(context.Context, OpenConversationReq) (OpenConversationResp, error) {
	return OpenConversationResp{}, nil
}
func (f *fakeBackend) Chat(context.Context, ChatReq) (ChatResp, error) {
	return ChatResp{}, nil
}
func (f *fakeBackend) ChatStream(context.Context, ChatReq) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk)
	close(ch)
	return ch, nil
}
func (f *fakeBackend) UploadFile(context.Context, UploadFileReq) (UploadFileResp, error) {
	return UploadFileResp{}, nil
}
func (f *fakeBackend) Health(context.Context) error { return nil }

func TestRegisterAndNew(t *testing.T) {
	const typeName = "fake-test-registry"

	Register(typeName, func(cfg map[string]any) (AIBackend, error) {
		name, _ := cfg["name"].(string)
		return &fakeBackend{name: name}, nil
	})

	b, err := New(typeName, map[string]any{"name": "demo"})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if b.Name() != "demo" {
		t.Fatalf("Name = %q, want %q", b.Name(), "demo")
	}
	if b.Type() != "fake" {
		t.Fatalf("Type = %q, want %q", b.Type(), "fake")
	}
}

func TestNewUnknownType(t *testing.T) {
	if _, err := New("does-not-exist", nil); err == nil {
		t.Fatal("expected error for unknown type, got nil")
	}
}

func TestIsRetryable(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{ErrBackendUnavailable, true},
		{ErrRateLimited, true},
		{ErrBadRequest, false},
		{ErrUnauthorized, false},
		{ErrConversationNotFound, false},
		{errors.New("other"), false},
	}
	for _, c := range cases {
		if got := IsRetryable(c.err); got != c.want {
			t.Errorf("IsRetryable(%v) = %v, want %v", c.err, got, c.want)
		}
	}
}
