package aibackend

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
)

const (
	routerTestType = "fake-router-test"
	routerBadType  = "fake-router-bad"
)

var routerTestRegisterOnce sync.Once

func registerRouterTestFactories(t *testing.T) {
	t.Helper()
	routerTestRegisterOnce.Do(func() {
		Register(routerTestType, func(cfg map[string]any) (AIBackend, error) {
			name, _ := cfg["Name"].(string)
			return &routerFake{name: name, t: routerTestType}, nil
		})
		Register(routerBadType, func(cfg map[string]any) (AIBackend, error) {
			return nil, errFakeBoom
		})
	})
}

type routerFake struct {
	name, t string
}

func (f *routerFake) Name() string { return f.name }
func (f *routerFake) Type() string { return f.t }
func (f *routerFake) OpenConversation(context.Context, OpenConversationReq) (OpenConversationResp, error) {
	return OpenConversationResp{}, nil
}
func (f *routerFake) Chat(context.Context, ChatReq) (ChatResp, error) {
	return ChatResp{}, nil
}
func (f *routerFake) ChatStream(context.Context, ChatReq) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk)
	close(ch)
	return ch, nil
}
func (f *routerFake) UploadFile(context.Context, UploadFileReq) (UploadFileResp, error) {
	return UploadFileResp{}, nil
}
func (f *routerFake) Health(context.Context) error { return nil }

type fakeError string

func (e fakeError) Error() string { return string(e) }

const errFakeBoom = fakeError("router-test: factory boom")

func TestNewRouterEmpty(t *testing.T) {
	registerRouterTestFactories(t)
	r, err := NewRouter(RouterConfig{})
	if err != nil {
		t.Fatalf("NewRouter empty: %v", err)
	}
	if r.Default() != nil {
		t.Errorf("empty Default = %v, want nil", r.Default())
	}
	if r.Get("anything") != nil {
		t.Errorf("empty Get = %v, want nil", r.Get("anything"))
	}
	if names := r.Names(); len(names) != 0 {
		t.Errorf("empty Names = %v, want []", names)
	}
}

func TestNewRouterHappyPath(t *testing.T) {
	registerRouterTestFactories(t)
	r, err := NewRouter(RouterConfig{
		Default: "main",
		Backends: []BackendInstanceConfig{
			{Name: "main", Type: routerTestType, BaseURL: "https://a"},
			{Name: "backup", Type: routerTestType, BaseURL: "https://b"},
		},
	})
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	if got := r.Default(); got == nil || got.Name() != "main" {
		t.Errorf("Default = %v, want backend named main", got)
	}
	if got := r.Get("backup"); got == nil || got.Name() != "backup" {
		t.Errorf("Get(backup) = %v, want backend named backup", got)
	}
	if got := r.Get("missing"); got != nil {
		t.Errorf("Get(missing) = %v, want nil", got)
	}
	names := r.Names()
	sort.Strings(names)
	if got, want := strings.Join(names, ","), "backup,main"; got != want {
		t.Errorf("Names = %q, want %q", got, want)
	}
}

func TestNewRouterValidation(t *testing.T) {
	registerRouterTestFactories(t)
	cases := []struct {
		name   string
		cfg    RouterConfig
		errSub string
	}{
		{
			name: "missing-Name",
			cfg: RouterConfig{Backends: []BackendInstanceConfig{
				{Type: routerTestType},
			}},
			errSub: "missing Name",
		},
		{
			name: "missing-Type",
			cfg: RouterConfig{Backends: []BackendInstanceConfig{
				{Name: "x"},
			}},
			errSub: "missing Type",
		},
		{
			name: "duplicate-name",
			cfg: RouterConfig{Backends: []BackendInstanceConfig{
				{Name: "a", Type: routerTestType},
				{Name: "a", Type: routerTestType},
			}},
			errSub: "duplicate backend name",
		},
		{
			name: "default-missing",
			cfg: RouterConfig{
				Default: "ghost",
				Backends: []BackendInstanceConfig{
					{Name: "a", Type: routerTestType},
				},
			},
			errSub: "default backend",
		},
		{
			name: "factory-error",
			cfg: RouterConfig{Backends: []BackendInstanceConfig{
				{Name: "a", Type: routerBadType},
			}},
			errSub: "factory boom",
		},
		{
			name: "unknown-type",
			cfg: RouterConfig{Backends: []BackendInstanceConfig{
				{Name: "a", Type: "never-registered-xyz"},
			}},
			errSub: "unknown backend type",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := NewRouter(c.cfg)
			if err == nil {
				t.Fatalf("want error containing %q, got nil", c.errSub)
			}
			if !strings.Contains(err.Error(), c.errSub) {
				t.Errorf("err = %v, want substring %q", err, c.errSub)
			}
		})
	}
}

func TestShouldUseRouterBoundaries(t *testing.T) {
	registerRouterTestFactories(t)

	// nil receiver
	var nilR *Router
	if nilR.ShouldUseRouter("u1") {
		t.Errorf("nil router should never route")
	}

	// 0% → always false
	r0, _ := NewRouter(RouterConfig{
		RolloutPercent: 0,
		Backends:       []BackendInstanceConfig{{Name: "a", Type: routerTestType}},
	})
	for _, k := range []string{"", "u1", "u2", "verylongkey-xyz"} {
		if r0.ShouldUseRouter(k) {
			t.Errorf("0%% but ShouldUseRouter(%q)=true", k)
		}
	}

	// 100% → always true
	r100, _ := NewRouter(RouterConfig{
		RolloutPercent: 100,
		Backends:       []BackendInstanceConfig{{Name: "a", Type: routerTestType}},
	})
	for _, k := range []string{"", "u1", "u2"} {
		if !r100.ShouldUseRouter(k) {
			t.Errorf("100%% but ShouldUseRouter(%q)=false", k)
		}
	}
}

func TestShouldUseRouterStable(t *testing.T) {
	registerRouterTestFactories(t)
	r, _ := NewRouter(RouterConfig{
		RolloutPercent: 50,
		Backends:       []BackendInstanceConfig{{Name: "a", Type: routerTestType}},
	})
	// 同一 key 多次调用必须稳定（同侧）
	for _, k := range []string{"u1", "u2", "user-xyz-007", "bot-1001-user-42"} {
		first := r.ShouldUseRouter(k)
		for i := 0; i < 20; i++ {
			if r.ShouldUseRouter(k) != first {
				t.Errorf("ShouldUseRouter(%q) flapped", k)
				break
			}
		}
	}
	// 空 key 在中间百分比时保守 false
	if r.ShouldUseRouter("") {
		t.Errorf("empty key should be false in mid-percent")
	}
}

func TestShouldUseRouterDistribution(t *testing.T) {
	registerRouterTestFactories(t)
	r, _ := NewRouter(RouterConfig{
		RolloutPercent: 30,
		Backends:       []BackendInstanceConfig{{Name: "a", Type: routerTestType}},
	})
	// 大量随机 key 命中率应大致接近 30%（容差 ±10%，避免 flaky）
	hits, total := 0, 5000
	for i := 0; i < total; i++ {
		if r.ShouldUseRouter(fmt.Sprintf("user-%d", i)) {
			hits++
		}
	}
	rate := float64(hits) / float64(total) * 100
	if rate < 20 || rate > 40 {
		t.Errorf("rollout 30%% landed %.1f%% (want roughly 20-40%%)", rate)
	}
}

func TestNewRouterRejectsBadPercent(t *testing.T) {
	for _, p := range []int{-1, 101, 1000} {
		if _, err := NewRouter(RouterConfig{RolloutPercent: p}); err == nil {
			t.Errorf("RolloutPercent=%d should fail, got nil", p)
		}
	}
}

func TestToFactoryMapDropsEmpty(t *testing.T) {
	m, err := toFactoryMap(BackendInstanceConfig{
		Name:    "n",
		Type:    "t",
		BaseURL: "",
		APIKey:  "k",
	})
	if err != nil {
		t.Fatalf("toFactoryMap: %v", err)
	}
	if _, has := m["BaseURL"]; has {
		t.Errorf("BaseURL should have been dropped (empty)")
	}
	if _, has := m["APIKey"]; !has {
		t.Errorf("APIKey should have been kept")
	}
}
