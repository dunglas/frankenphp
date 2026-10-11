package caddy

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/caddyauth"
	_ "github.com/caddyserver/caddy/v2/modules/caddyhttp/encode/gzip"
	_ "github.com/caddyserver/caddy/v2/modules/caddyhttp/encode/zstd"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/fileserver"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/headers"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/intercept"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/rewrite"
	_ "github.com/caddyserver/caddy/v2/modules/caddyhttp/templates"
	_ "github.com/caddyserver/caddy/v2/modules/caddyhttp/timeouts"
	_ "github.com/caddyserver/caddy/v2/modules/caddyhttp/tracing"
	_ "github.com/dunglas/caddy-cbrotli"
	"github.com/dunglas/frankenphp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type reloadShutdownBarrier struct {
	entered chan struct{}
	release chan struct{}
	held    sync.Once
	freed   sync.Once
}

func (b *reloadShutdownBarrier) shutdown(int) {
	b.held.Do(func() {
		close(b.entered)
		<-b.release
	})
}

func (b *reloadShutdownBarrier) unblock() { b.freed.Do(func() { close(b.release) }) }

func reloadTestRequest(path, body string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "http://example.com"+path, strings.NewReader(body))
	original := r.Clone(r.Context())
	ctx := context.WithValue(r.Context(), caddy.ReplacerCtxKey, caddy.NewReplacer())
	ctx = context.WithValue(ctx, caddyhttp.OriginalRequestCtxKey, *original)
	r = r.WithContext(ctx)
	r.URL.Path = "/echo.php"
	return r
}

func setReloadTestRoutes(t *testing.T, app *FrankenPHPApp, paths []string, reverseScopeOrder bool) {
	t.Helper()
	hosts := caddyhttp.MatchHost{"example.com", "alias.example.com"}
	listeners := []string{"127.0.0.1:9080", "127.0.0.1:9081"}
	if reverseScopeOrder {
		hosts = caddyhttp.MatchHost{"alias.example.com", "example.com"}
		listeners = []string{"127.0.0.1:9081", "127.0.0.1:9080"}
	}
	var routes caddyhttp.RouteList
	for i, module := range app.modules {
		path := caddyhttp.MatchPath{paths[i] + "/*"}
		rw := &rewrite.Rewrite{URI: "/echo.php"}
		require.NoError(t, rw.Provision(caddy.Context{}))
		inner := caddyhttp.RouteList{
			{Handlers: []caddyhttp.MiddlewareHandler{rw}},
			{Handlers: []caddyhttp.MiddlewareHandler{module}},
		}
		require.NoError(t, inner.ProvisionHandlers(caddy.Context{}, nil))
		routes = append(routes, caddyhttp.Route{
			MatcherSets: caddyhttp.MatcherSets{{hosts, path}},
			Handlers:    []caddyhttp.MiddlewareHandler{&caddyhttp.Subroute{Routes: inner}},
		})
	}
	require.NoError(t, routes.ProvisionHandlers(caddy.Context{}, nil))
	app.httpApp = &caddyhttp.App{Servers: map[string]*caddyhttp.Server{
		"srv0": {Listen: listeners, Routes: routes},
	}}
}

// Exercise the real PHP shutdown/init and Caddy module handoff without using
// sockets: keeping old modules models requests accepted by the old HTTP config.
func TestReloadHandoffPreservesConcurrentBodiesAndServerScope(t *testing.T) {
	if testing.Short() {
		t.Skip("requires the PHP runtime")
	}
	for _, workerMode := range []bool{false, true} {
		t.Run(fmt.Sprintf("worker=%t", workerMode), func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			require.NoError(t, err)
			script := `<?php
$handler = static function (): void {
    echo $_SERVER['SITE'] . ':' . $_SERVER['REQUEST_URI'] . ':' . file_get_contents('php://input');
};
if (getenv('FRANKENPHP_WORKER')) {
    while (frankenphp_handle_request($handler)) {}
} else {
    $handler();
}

`
			require.NoError(t, os.WriteFile(filepath.Join(root, "echo.php"), []byte(script), 0600))
			require.NoError(t, os.WriteFile(filepath.Join(root, "barrier.php"), []byte(script), 0600))
			var barriers []*reloadShutdownBarrier
			var reloads sync.WaitGroup
			t.Cleanup(func() {
				for _, b := range barriers {
					b.unblock()
				}
				reloads.Wait()
				frankenphp.Shutdown()
				activeApp.Store(nil)
			})
			newApp := func(order []string) (*FrankenPHPApp, *reloadShutdownBarrier) {
				b := &reloadShutdownBarrier{entered: make(chan struct{}), release: make(chan struct{})}
				barriers = append(barriers, b)
				app := &FrankenPHPApp{
					NumThreads: 1, MaxThreads: 3, ctx: context.Background(), logger: slog.Default(),
					metrics: frankenphp.NewPrometheusMetrics(prometheus.NewRegistry()), started: make(chan any),
				}
				for i, site := range order {
					module := &FrankenPHPModule{
						Root: root, Name: "example.com", ServerIndex: i + 1, SplitPath: []string{".php"},
						Env: map[string]string{"SITE": site}, resolvedEnv: map[string]string{"SITE": site},
						resolvedDocumentRoot: root, logger: slog.Default(), app: app,
					}
					if workerMode {
						module.Workers = []workerConfig{{
							Name: site, FileName: filepath.Join(root, "echo.php"), Num: 1,
							options: []frankenphp.WorkerOption{frankenphp.WithWorkerOnShutdown(b.shutdown)},
						}}
					}
					app.modules = append(app.modules, module)
				}
				if !workerMode {
					app.Workers = []workerConfig{{
						Name: "barrier", FileName: filepath.Join(root, "barrier.php"), Num: 1,
						options: []frankenphp.WorkerOption{frankenphp.WithWorkerOnShutdown(b.shutdown)},
					}}
				}
				paths := make([]string, len(order))
				for i, site := range order {
					paths[i] = "/" + site
				}
				// Reordered hosts and listeners describe the same request scope.
				setReloadTestRoutes(t, app, paths, order[0] == "green")
				return app, b
			}
			current, barrier := newApp([]string{"blue", "green"})
			require.NoError(t, current.Start())
			for _, order := range [][]string{{"blue", "green"}, {"green", "blue"}} {
				next, nextBarrier := newApp(order)
				// A newly provisioned module can receive traffic before its app's
				// Start: it must wait for its own runtime rather than use the old app.
				future := next.modules[0]
				futureWriter := httptest.NewRecorder()
				futureDone := make(chan error, 1)
				go func() {
					futureDone <- future.ServeHTTP(futureWriter, reloadTestRequest("/"+order[0]+"/new", "new body"), nil)
				}()
				futureReturned := false
				select {
				case err := <-futureDone:
					futureReturned = true
					t.Errorf("new module returned before its own app started: %v", err)
				case <-time.After(20 * time.Millisecond):
				}
				loaded := make(chan error, 1)
				reloads.Add(1)
				go func() {
					defer reloads.Done()
					loaded <- next.Start()
				}()
				select {
				case <-barrier.entered:
				case <-time.After(5 * time.Second):
					t.Fatal("reload never entered PHP shutdown")
				}
				r := reloadTestRequest("/"+current.modules[0].Env["SITE"]+"/canceled", "")
				rctx, cancelRequest := context.WithCancel(r.Context())
				cancelRequest()
				r = r.WithContext(rctx)
				require.ErrorIs(t, current.modules[0].ServeHTTP(httptest.NewRecorder(), r, nil), context.Canceled)
				type result struct {
					err      error
					body     string
					expected string
				}
				results := make(chan result, 8)
				for _, module := range current.modules {
					for n := range 4 {
						payload := fmt.Sprintf("request-%s-%d", module.Env["SITE"], n)
						go func() {
							path := "/" + module.Env["SITE"] + "/original/path"
							r := reloadTestRequest(path, payload)
							w := httptest.NewRecorder()
							err := module.ServeHTTP(w, r, nil)
							results <- result{err, w.Body.String(), module.Env["SITE"] + ":" + path + ":" + payload}
						}()
					}
				}
				select {
				case result := <-results:
					t.Fatalf("request returned while PHP was stopped: %v, body %q", result.err, result.body)
				case <-time.After(20 * time.Millisecond):
				}
				barrier.unblock()
				select {
				case err := <-loaded:
					require.NoError(t, err)
				case <-time.After(10 * time.Second):
					t.Fatal("reload failed to finish after shutdown was released")
				}
				if !futureReturned {
					select {
					case err := <-futureDone:
						require.NoError(t, err)
						require.Equal(t, order[0]+":/"+order[0]+"/new:new body", futureWriter.Body.String())
					case <-time.After(5 * time.Second):
						t.Fatal("new module did not start serving after its app started")
					}
				}
				for range 8 {
					select {
					case result := <-results:
						require.NoError(t, result.err)
						require.Equal(t, result.expected, result.body)
					case <-time.After(5 * time.Second):
						t.Fatal("request was not handed over after the reload")
					}
				}
				current, barrier = next, nextBarrier
			}
		})
	}
}

// Both handlers have identical PHP configuration, but each route has its own
// worker state. Reordering routes must not merge their queued requests.
func TestReloadHandoffKeepsIdenticalWorkersInTheirRoutes(t *testing.T) {
	if testing.Short() {
		t.Skip("requires the PHP runtime")
	}
	root, rootErr := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, rootErr)
	script := `<?php
$count = 0;
while (frankenphp_handle_request(static function () use (&$count): void { echo ++$count; })) {}
`
	require.NoError(t, os.WriteFile(filepath.Join(root, "echo.php"), []byte(script), 0600))
	var barriers []*reloadShutdownBarrier
	var reloads sync.WaitGroup
	t.Cleanup(func() {
		for _, barrier := range barriers {
			barrier.unblock()
		}
		reloads.Wait()
		frankenphp.Shutdown()
		activeApp.Store(nil)
	})
	newApp := func(paths []string) (*FrankenPHPApp, *reloadShutdownBarrier) {
		barrier := &reloadShutdownBarrier{entered: make(chan struct{}), release: make(chan struct{})}
		barriers = append(barriers, barrier)
		app := &FrankenPHPApp{
			NumThreads: 1, MaxThreads: 3, ctx: context.Background(), logger: slog.Default(),
			metrics: frankenphp.NewPrometheusMetrics(prometheus.NewRegistry()), started: make(chan any),
		}
		for i := range paths {
			app.modules = append(app.modules, &FrankenPHPModule{
				Root: root, Name: "example.com", ServerIndex: i + 1, SplitPath: []string{".php"},
				resolvedDocumentRoot: root, logger: slog.Default(), app: app,
				Workers: []workerConfig{{
					FileName: filepath.Join(root, "echo.php"), Num: 1,
					options: []frankenphp.WorkerOption{frankenphp.WithWorkerOnShutdown(barrier.shutdown)},
				}},
			})
		}
		setReloadTestRoutes(t, app, paths, paths[0] == "/protected")
		return app, barrier
	}
	old, oldBarrier := newApp([]string{"/public", "/protected"})
	require.NoError(t, old.Start())
	next, nextBarrier := newApp([]string{"/protected", "/public"})
	loaded := make(chan error, 1)
	reloads.Add(1)
	go func() {
		defer reloads.Done()
		loaded <- next.Start()
	}()
	select {
	case <-oldBarrier.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("reload never entered PHP shutdown")
	}
	type result struct {
		err  error
		body string
	}
	results := make(chan result, 2)
	for i, module := range old.modules {
		path := []string{"/public/request", "/protected/request"}[i]
		go func() {
			w := httptest.NewRecorder()
			err := module.ServeHTTP(w, reloadTestRequest(path, ""), nil)
			results <- result{err, w.Body.String()}
		}()
	}
	oldBarrier.unblock()
	select {
	case err := <-loaded:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("reload did not finish")
	}
	for range 2 {
		select {
		case result := <-results:
			require.NoError(t, result.err)
			require.Equal(t, "1", result.body, "each route must dispatch to its own worker")
		case <-time.After(5 * time.Second):
			t.Fatal("retired route request never returned")
		}
	}
	for i, module := range next.modules {
		w := httptest.NewRecorder()
		path := []string{"/protected/request", "/public/request"}[i]
		require.NoError(t, module.ServeHTTP(w, reloadTestRequest(path, ""), nil))
		require.Equal(t, "2", w.Body.String())
	}

	// Removing a public route must not send its accepted requests to a
	// protected route merely because that route has the same PHP config.
	removedPublic := next.modules[1]
	protected, _ := newApp([]string{"/protected"})
	nextBarrier.unblock()
	require.NoError(t, protected.Start())
	w := httptest.NewRecorder()
	err := removedPublic.ServeHTTP(w, reloadTestRequest("/public/request", ""), nil)
	var handlerError caddyhttp.HandlerError
	require.ErrorAs(t, err, &handlerError)
	require.Equal(t, http.StatusServiceUnavailable, handlerError.StatusCode)
	require.Empty(t, w.Body.String())
	w = httptest.NewRecorder()
	require.NoError(t, protected.modules[0].ServeHTTP(w, reloadTestRequest("/protected/request", ""), nil))
	require.Equal(t, "1", w.Body.String(), "removed public request must not execute the protected worker")
}

func TestReloadRouteKeyKeepsMatcherScope(t *testing.T) {
	key := func(sets caddyhttp.MatcherSets) string {
		module := &FrankenPHPModule{}
		value := reloadRouteKey(caddyhttp.RouteList{{
			MatcherSets: sets,
			Handlers:    []caddyhttp.MiddlewareHandler{module},
		}}, module)
		require.NotEmpty(t, value)
		return value
	}

	t.Run("provisioned negations retain their nested scope", func(t *testing.T) {
		public := &caddyhttp.MatchNot{MatcherSets: []caddyhttp.MatcherSet{{caddyhttp.MatchPath{"/public/*"}}}}
		protected := &caddyhttp.MatchNot{MatcherSets: []caddyhttp.MatcherSet{{caddyhttp.MatchPath{"/protected/*"}}}}
		// Caddy clears the raw module config during provisioning. Its runtime
		// nested matchers still distinguish these opposite request scopes.
		require.Nil(t, public.MatcherSetsRaw)
		require.Nil(t, protected.MatcherSetsRaw)
		require.NotEqual(t, key(caddyhttp.MatcherSets{{public}}), key(caddyhttp.MatcherSets{{protected}}))
	})

	t.Run("unordered matchers in one set keep the same scope", func(t *testing.T) {
		host := caddyhttp.MatchHost{"example.com"}
		path := caddyhttp.MatchPath{"/public/*"}
		require.Equal(t, key(caddyhttp.MatcherSets{{&host, path}}), key(caddyhttp.MatcherSets{{path, &host}}))
	})

	t.Run("matcher module kinds distinguish identical JSON values", func(t *testing.T) {
		host := caddyhttp.MatchHost{"same-value"}
		path := caddyhttp.MatchPath{"same-value"}
		require.NotEqual(t, key(caddyhttp.MatcherSets{{&host}}), key(caddyhttp.MatcherSets{{path}}))
	})

	for _, pointer := range []bool{false, true} {
		name := "host value order does not change scope"
		if pointer {
			name = "host pointer order does not change scope"
		}
		t.Run(name, func(t *testing.T) {
			first := caddyhttp.MatchHost{"z.example.com", "a.example.com"}
			permuted := caddyhttp.MatchHost{"a.example.com", "z.example.com"}
			changed := caddyhttp.MatchHost{"a.example.com", "other.example.com"}
			asSet := func(host caddyhttp.MatchHost) caddyhttp.MatcherSets {
				if pointer {
					return caddyhttp.MatcherSets{{&host}}
				}
				return caddyhttp.MatcherSets{{host}}
			}
			require.Equal(t, key(asSet(first)), key(asSet(permuted)))
			require.NotEqual(t, key(asSet(first)), key(asSet(changed)))
			require.Equal(t, caddyhttp.MatchHost{"z.example.com", "a.example.com"}, first, "keying must preserve provisioned host order")
		})
	}

	t.Run("nested negated host order does not change scope", func(t *testing.T) {
		first := caddyhttp.MatchHost{"z.example.com", "a.example.com"}
		permuted := caddyhttp.MatchHost{"a.example.com", "z.example.com"}
		changed := caddyhttp.MatchHost{"a.example.com", "other.example.com"}
		negated := func(host *caddyhttp.MatchHost) caddyhttp.MatcherSets {
			return caddyhttp.MatcherSets{{&caddyhttp.MatchNot{MatcherSets: []caddyhttp.MatcherSet{{
				&caddyhttp.MatchNot{MatcherSets: []caddyhttp.MatcherSet{{host}}},
			}}}}}
		}
		require.Equal(t, key(negated(&first)), key(negated(&permuted)))
		require.NotEqual(t, key(negated(&first)), key(negated(&changed)))
		require.Equal(t, caddyhttp.MatchHost{"z.example.com", "a.example.com"}, first)
	})
}

func TestReloadKeyIgnoresListenerOrder(t *testing.T) {
	key := func(listeners []string) string {
		app := &FrankenPHPApp{}
		module := &FrankenPHPModule{app: app}
		app.httpApp = &caddyhttp.App{Servers: map[string]*caddyhttp.Server{
			"srv0": {
				Listen: listeners,
				Routes: caddyhttp.RouteList{{Handlers: []caddyhttp.MiddlewareHandler{module}}},
			},
		}}
		value, _ := module.reloadKey()
		require.NotEmpty(t, value)
		return value
	}
	first := []string{"127.0.0.1:9081", "127.0.0.1:9080"}
	permuted := []string{"127.0.0.1:9080", "127.0.0.1:9081"}
	changed := []string{"127.0.0.1:9080", "127.0.0.1:9082"}
	require.Equal(t, key(first), key(permuted))
	require.NotEqual(t, key(first), key(changed))
	require.Equal(t, []string{"127.0.0.1:9081", "127.0.0.1:9080"}, first, "keying must preserve listener order")
}

func TestReloadRouteKeyNormalizesOnlyPHPServerDocumentRoot(t *testing.T) {
	key := func(root, guard, filename string, generated bool) string {
		module := &FrankenPHPModule{Root: root, ServerIndex: 1, GeneratedRewrite: generated}
		return reloadRouteKey(caddyhttp.RouteList{
			{MatcherSets: caddyhttp.MatcherSets{{&fileserver.MatchFile{Root: guard, TryFiles: []string{filename}}}},
				Handlers: []caddyhttp.MiddlewareHandler{&rewrite.Rewrite{URI: "{http.matchers.file.relative}{http.matchers.file.remainder}"}}},
			{Handlers: []caddyhttp.MiddlewareHandler{module}},
		}, module)
	}
	// JSON escaping in directory names must not prevent normalization.
	require.Equal(t, key("/releases/old<&", "/releases/old<&", "index.php", true), key("/releases/new<&", "/releases/new<&", "index.php", true))
	require.NotEqual(t, key("/app", "/guards/public", "index.php", true), key("/app", "/guards/protected", "index.php", true))
	require.NotEqual(t, key("/app", "/app", "public.php", true), key("/app", "/app", "protected.php", true))
	// Explicit file-based rewrite scopes before raw PHP remain independent.
	require.NotEqual(t, key("/first", "/first", "index.php", false), key("/second", "/second", "index.php", false))
}

func TestReloadRouteKeyDoesNotMatchRewriteOnlySubroutes(t *testing.T) {
	routes := caddyhttp.RouteList{{Handlers: []caddyhttp.MiddlewareHandler{&caddyhttp.Subroute{
		Routes: caddyhttp.RouteList{{Handlers: []caddyhttp.MiddlewareHandler{&rewrite.Rewrite{URI: "/index.php"}}}},
	}}}}
	for range 32 {
		routes = caddyhttp.RouteList{{Handlers: []caddyhttp.MiddlewareHandler{&caddyhttp.Subroute{Routes: routes}}}}
	}
	require.Empty(t, reloadRouteKey(routes, &FrankenPHPModule{}), "rewrite context alone must not identify a PHP handler")
}

func reloadRouteKey(routes caddyhttp.RouteList, target *FrankenPHPModule) string {
	keys := make(map[*FrankenPHPModule][2]string)
	reloadRouteKeys(routes, "", "", keys, true, nil)
	return keys[target][0]
}

func TestReloadKeyRejectsAmbiguousDispatchScopes(t *testing.T) {
	app := &FrankenPHPApp{}
	first, second := &FrankenPHPModule{app: app}, &FrankenPHPModule{app: app}
	app.modules = []*FrankenPHPModule{first, second}
	app.httpApp = &caddyhttp.App{Servers: map[string]*caddyhttp.Server{
		"srv0": {Routes: caddyhttp.RouteList{
			{Handlers: []caddyhttp.MiddlewareHandler{first}},
			{Handlers: []caddyhttp.MiddlewareHandler{second}},
		}},
	}}
	for _, module := range []*FrankenPHPModule{first, second} {
		key, _ := module.reloadKey()
		require.Empty(t, key)
	}
}

// Two sequential route blocks can select separate PHP roots even though both
// PHP handlers match the same rewritten path. Retired HTTP handlers must retain
// that selection when PHP dispatch resumes against a replacement config.
func TestReloadHandoffKeepsSequentialRewriteRoots(t *testing.T) {
	t.Run("raw php", func(t *testing.T) { testReloadRewriteRoots(t, false) })
	t.Run("php_server", func(t *testing.T) { testReloadRewriteRoots(t, true) })
	t.Run("nested rewrite", func(t *testing.T) { testReloadRewriteRoots(t, false, "nested") })
	t.Run("conditional nested php", func(t *testing.T) { testReloadRewriteRoots(t, false, "conditional") })
	t.Run("PHP-only subroutes", func(t *testing.T) { testReloadRewriteRoots(t, false, "php-only") })
	t.Run("duplicate addition", func(t *testing.T) { testReloadRewriteRoots(t, false, "add") })
	t.Run("duplicate removal", func(t *testing.T) { testReloadRewriteRoots(t, false, "remove") })
}

func testReloadRewriteRoots(t *testing.T, phpServer bool, options ...string) {
	mode := ""
	if len(options) != 0 {
		mode = options[0]
	}
	nested, adding, removing := mode != "", mode == "add", mode == "remove"
	if testing.Short() {
		t.Skip("requires the PHP runtime")
	}
	root := t.TempDir()
	for _, site := range []string{"foo", "bar"} {
		for _, generation := range []string{"old", "new"} {
			directory := filepath.Join(root, generation, site)
			require.NoError(t, os.MkdirAll(directory, 0700))
			require.NoError(t, os.WriteFile(filepath.Join(directory, "index.php"), []byte("<?php echo '"+generation+":"+site+"';"), 0600))
		}
	}
	t.Cleanup(func() {
		require.NoError(t, caddy.Stop())
		frankenphp.Shutdown()
		activeApp.Store(nil)
	})
	load := func(generation string, reverse bool) *FrankenPHPApp {
		sites := []string{"foo", "bar"}
		if adding && generation == "old" || removing && generation == "new" {
			sites = sites[:1]
		}
		if reverse {
			sites[0], sites[1] = sites[1], sites[0]
		}
		route := func(site string) string {
			if nested {
				if mode == "php-only" || adding || removing {
					return fmt.Sprintf("route {\nrewrite /%s /index.php\n}\nroute {\nphp /index.php {\nroot %s\n}\n}", site, filepath.Join(root, generation, site))
				}
				if mode == "conditional" {
					return fmt.Sprintf("route {\nrewrite /%s /index.php\nphp /only-worker.php {\nroot %s\n}\n}\nphp /index.php {\nroot %s\n}", site, filepath.Join(root, generation, site), filepath.Join(root, generation, site))
				}
				return fmt.Sprintf("route {\nrewrite /%s /index.php\n}\nphp /index.php {\nroot %s\n}", site, filepath.Join(root, generation, site))
			}
			if phpServer {
				return fmt.Sprintf("route /%s {\nphp_server {\nroot %s\n}\n}", site, filepath.Join(root, generation, site))
			}
			return fmt.Sprintf("route {\nrewrite /%s /index.php\nphp /index.php {\nroot %s\n}\n}", site, filepath.Join(root, generation, site))
		}
		routes := route(sites[0])
		for _, site := range sites[1:] {
			routes += "\n" + route(site)
		}
		if nested {
			routes = "route {\n" + routes + "\n}"
		}
		config := fmt.Sprintf(`{
    admin off
    persist_config off
    auto_https off
    frankenphp {
        num_threads 2
    }
}
http://example.com:0 {
    bind 127.0.0.1
    %s
}`, routes)
		adapted, _, err := caddyconfig.GetAdapter("caddyfile").Adapt([]byte(config), nil)
		require.NoError(t, err)
		require.NoError(t, caddy.Load(adapted, true))
		return activeApp.Load()
	}
	serve := func(server *caddyhttp.Server, site, expected string) {
		w := httptest.NewRecorder()
		server.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "http://example.com/"+site, nil))
		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, expected, w.Body.String())
	}
	old := load("old", false)
	var oldServer *caddyhttp.Server
	for _, server := range old.httpApp.Servers {
		oldServer = server
	}
	require.NotNil(t, oldServer)
	serve(oldServer, "foo", "old:foo")
	if !adding {
		serve(oldServer, "bar", "old:bar")
	}
	next := load("new", !nested)
	require.NotSame(t, old, next)
	for _, server := range next.httpApp.Servers {
		serve(server, "foo", "new:foo")
		if !removing {
			serve(server, "bar", "new:bar")
		}
	}
	serve(oldServer, "foo", "new:foo")
	if !adding {
		if removing {
			w := httptest.NewRecorder()
			oldServer.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "http://example.com/bar", nil))
			require.Equal(t, http.StatusServiceUnavailable, w.Code, "removed ambiguous scope must not dispatch to surviving root")
		} else {
			serve(oldServer, "bar", "new:bar")
		}
	}
}

func TestReloadHandoffKeepsManualFileMatcherScopes(t *testing.T) {
	if testing.Short() {
		t.Skip("requires the PHP runtime")
	}
	root := t.TempDir()
	first, second := filepath.Join(root, "first"), filepath.Join(root, "second")
	for directory, filename := range map[string]string{first: "foo.php", second: "index.php"} {
		require.NoError(t, os.MkdirAll(directory, 0700))
		script := "<?php echo $_SERVER['GENERATION'] . ':" + filepath.Base(directory) + "';"
		require.NoError(t, os.WriteFile(filepath.Join(directory, filename), []byte(script), 0600))
	}
	t.Cleanup(func() {
		require.NoError(t, caddy.Stop())
		frankenphp.Shutdown()
		activeApp.Store(nil)
	})
	load := func(generation string) *caddyhttp.Server {
		config := fmt.Sprintf(`{
    admin off
    persist_config off
    auto_https off
    frankenphp {
        num_threads 2
    }
}
http://example.com:0 {
    bind 127.0.0.1
    @php path *.php
    @first file {
        root %s
        try_files {path}.php
    }
    @second file {
        root %s
        try_files {path}.php
    }
    route {
        rewrite @first {http.matchers.file.relative}{http.matchers.file.remainder}
        php @php {
            root %s
            env GENERATION %s
        }
    }
    route {
        rewrite @second {http.matchers.file.relative}{http.matchers.file.remainder}
        php @php {
            root %s
            env GENERATION %s
        }
    }
}`, first, second, first, generation, second, generation)
		adapted, _, err := caddyconfig.GetAdapter("caddyfile").Adapt([]byte(config), nil)
		require.NoError(t, err)
		require.NoError(t, caddy.Load(adapted, true))
		for _, server := range activeApp.Load().httpApp.Servers {
			return server
		}
		t.Fatal("no provisioned HTTP server")
		return nil
	}
	serve := func(server *caddyhttp.Server, path, expected string) {
		w := httptest.NewRecorder()
		server.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "http://example.com/"+path, nil))
		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, expected, w.Body.String())
	}
	old := load("old")
	serve(old, "foo", "old:first")
	serve(old, "index", "old:second")
	next := load("new")
	serve(next, "foo", "new:first")
	serve(next, "index", "new:second")
	serve(old, "foo", "new:first")
	serve(old, "index", "new:second")
}

func TestReloadHandoffRejectsChangedRewriteGuard(t *testing.T) {
	if testing.Short() {
		t.Skip("requires the PHP runtime")
	}
	for _, mixed := range []bool{false, true} {
		t.Run(fmt.Sprintf("mixed=%t", mixed), func(t *testing.T) {
			root := t.TempDir()
			for _, generation := range []string{"old", "new"} {
				directory := filepath.Join(root, generation)
				require.NoError(t, os.MkdirAll(directory, 0700))
				for _, filename := range []string{"index.php", "public.php"} {
					require.NoError(t, os.WriteFile(filepath.Join(directory, filename), []byte("<?php echo '"+generation+"';"), 0600))
				}
			}
			t.Cleanup(func() {
				require.NoError(t, caddy.Stop())
				frankenphp.Shutdown()
				activeApp.Store(nil)
			})
			load := func(generation string) *caddyhttp.Server {
				directory := filepath.Join(root, generation)
				path := "public"
				if generation == "new" && !mixed {
					path = "protected"
				}
				routes := fmt.Sprintf("route {\nroute {\nrewrite /%s /index.php\n}\nroute {\nphp /index.php {\nroot %s\n}\n}\n}", path, directory)
				if mixed {
					routes = fmt.Sprintf(`@guard file {
    root %s
    try_files {path}.php
}
route {
    rewrite @guard {http.matchers.file.relative}{http.matchers.file.remainder}
    php /only-worker.php {
        root %s
    }
    php_server {
        root %s
    }
}`, directory, directory, directory)
				}
				config := `{
    admin off
    persist_config off
    auto_https off
    frankenphp {
        num_threads 2
    }
}
http://example.com:0 {
    bind 127.0.0.1
` + routes + "\n}"
				adapted, _, err := caddyconfig.GetAdapter("caddyfile").Adapt([]byte(config), nil)
				require.NoError(t, err)
				require.NoError(t, caddy.Load(adapted, true))
				for _, server := range activeApp.Load().httpApp.Servers {
					return server
				}
				t.Fatal("no provisioned HTTP server")
				return nil
			}
			serve := func(server *caddyhttp.Server, path string, status int, body string) {
				w := httptest.NewRecorder()
				server.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "http://example.com/"+path, nil))
				require.Equal(t, status, w.Code)
				require.Equal(t, body, w.Body.String())
			}
			old := load("old")
			serve(old, "public", http.StatusOK, "old")
			next := load("new")
			path := "protected"
			if mixed {
				path = "public"
			}
			serve(next, path, http.StatusOK, "new")
			serve(old, "public", http.StatusServiceUnavailable, "")
		})
	}
}

func reloadSecurityServerConfig(t *testing.T, routes string, configure func(*caddyhttp.Server)) []byte {
	t.Helper()
	var config caddy.Config
	require.NoError(t, json.Unmarshal(reloadSecurityConfig(t, routes), &config))
	var httpApp caddyhttp.App
	require.NoError(t, json.Unmarshal(config.AppsRaw["http"], &httpApp))
	for _, server := range httpApp.Servers {
		configure(server)
	}
	config.AppsRaw["http"] = caddyconfig.JSON(httpApp, nil)
	return caddyconfig.JSON(config, nil)
}

func TestReloadKeyIncludesServerSecurity(t *testing.T) {
	for name, change := range map[string]func(*caddyhttp.Server){
		"underscore": func(s *caddyhttp.Server) { s.ExpectedUnderscoreHeaders = []string{"X_Trusted"} },
		"dot":        func(s *caddyhttp.Server) { s.ExpectedDotHeaders = []string{"X.Trusted"} },
		"SNI":        func(s *caddyhttp.Server) { s.StrictSNIHost = new(true) },
		"proxies": func(s *caddyhttp.Server) {
			s.TrustedProxiesRaw = json.RawMessage(`{"source":"static","ranges":["127.0.0.1/32"]}`)
		},
		"IP headers":     func(s *caddyhttp.Server) { s.ClientIPHeaders = []string{"X-Real-IP"} },
		"strict proxies": func(s *caddyhttp.Server) { s.TrustedProxiesStrict = 1 },
		"unix proxies":   func(s *caddyhttp.Server) { s.TrustedProxiesUnix = true },
	} {
		t.Run(name, func(t *testing.T) {
			app := &FrankenPHPApp{}
			module := &FrankenPHPModule{app: app}
			server := &caddyhttp.Server{Routes: caddyhttp.RouteList{{Handlers: []caddyhttp.MiddlewareHandler{module}}}}
			app.httpApp = &caddyhttp.App{Servers: map[string]*caddyhttp.Server{"srv0": server}}
			before, _ := module.reloadKey()
			change(server)
			after, _ := module.reloadKey()
			require.NotEqual(t, before, after)
			unchanged, _ := module.reloadKey()
			require.Equal(t, after, unchanged)
		})
	}
}

func TestReloadHandoffRejectsChangedHeaderHygiene(t *testing.T) {
	for _, header := range []string{"X_Trusted", "X.Trusted"} {
		for _, tighten := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/tighten=%t", header, tighten), func(t *testing.T) {
				oldRoot, newRoot := reloadSecurityRoots(t)
				for _, root := range []string{oldRoot, newRoot} {
					require.NoError(t, os.WriteFile(filepath.Join(root, "index.php"), []byte(`<?php echo $_SERVER['HTTP_X_TRUSTED'] ?? 'missing';`), 0600))
				}
				routes := func(root string) string { return fmt.Sprintf("php /index.php {\nroot %s\n}", root) }
				configure := func(allow bool) func(*caddyhttp.Server) {
					return func(s *caddyhttp.Server) {
						if allow && header == "X_Trusted" {
							s.ExpectedUnderscoreHeaders = []string{header}
						}
						if allow && header == "X.Trusted" {
							s.ExpectedDotHeaders = []string{header}
						}
					}
				}
				require.NoError(t, caddy.Load(reloadSecurityServerConfig(t, routes(oldRoot), configure(true)), true))
				old := activeApp.Load()
				oldServer := old.httpApp.Servers["srv0"]
				require.NoError(t, caddy.Load(reloadSecurityServerConfig(t, routes(newRoot), configure(!tighten)), true))
				next := activeApp.Load()
				request := func() *http.Request {
					r := httptest.NewRequest(http.MethodGet, "http://example.com/index.php", nil)
					r.Header.Set(header, "spoofed")
					return r
				}
				direct := reloadSecurityServe(next.httpApp.Servers["srv0"], request())
				require.Equal(t, http.StatusOK, direct.Code)
				handoff := reloadSecurityServe(oldServer, request())
				if tighten {
					require.Equal(t, "missing", direct.Body.String())
					require.Equal(t, http.StatusServiceUnavailable, handoff.Code)
					require.Empty(t, handoff.Body.String())
				} else {
					require.Equal(t, "spoofed", direct.Body.String())
					require.Equal(t, http.StatusOK, handoff.Code)
					require.Equal(t, "spoofed", handoff.Body.String())
				}
			})
		}
	}
}

func TestReloadHandoffRejectsTightenedSNI(t *testing.T) {
	oldRoot, newRoot := reloadSecurityRoots(t)
	routes := func(root string) string { return fmt.Sprintf("php /index.php {\nroot %s\n}", root) }
	_, oldServer := reloadSecurityLoad(t, routes(oldRoot))
	require.NoError(t, caddy.Load(reloadSecurityServerConfig(t, routes(newRoot), func(s *caddyhttp.Server) { s.StrictSNIHost = new(true) }), true))
	request := func() *http.Request {
		r := httptest.NewRequest(http.MethodGet, "https://example.com/index.php", nil)
		r.TLS = &tls.ConnectionState{ServerName: "other.example.com"}
		return r
	}
	require.Equal(t, http.StatusMisdirectedRequest, reloadSecurityServe(activeApp.Load().httpApp.Servers["srv0"], request()).Code)
	w := reloadSecurityServe(oldServer, request())
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	require.NotContains(t, w.Body.String(), "new:")
}

func TestReloadHandoffEnforcesAddedDenyRoute(t *testing.T) {
	oldRoot, newRoot := reloadSecurityRoots(t)
	routes := func(root, deny string) string {
		return fmt.Sprintf("route /index.php {\n%sphp {\nroot %s\n}\n}", deny, root)
	}
	_, oldServer := reloadSecurityLoad(t, routes(oldRoot, ""))
	_, newServer := reloadSecurityLoad(t, routes(newRoot, "respond NEW-DENY-POLICY 403\n"))
	for _, server := range []*caddyhttp.Server{newServer, oldServer} {
		w := reloadSecurityServe(server, httptest.NewRequest(http.MethodGet, "http://example.com/index.php", nil))
		require.Equal(t, http.StatusForbidden, w.Code)
		require.Equal(t, "NEW-DENY-POLICY", w.Body.String())
	}
}

func TestReloadPrepareRequestRederivesClientIP(t *testing.T) {
	oldRoot, newRoot := reloadSecurityRoots(t)
	routes := func(root string) string { return fmt.Sprintf("php /index.php {\nroot %s\n}", root) }
	_, oldServer := reloadSecurityLoad(t, routes(oldRoot))
	require.NoError(t, caddy.Load(reloadSecurityServerConfig(t, routes(newRoot), func(s *caddyhttp.Server) {
		s.TrustedProxiesRaw = json.RawMessage(`{"source":"static","ranges":["127.0.0.1/32"]}`)
	}), true))
	newServer := activeApp.Load().httpApp.Servers["srv0"]
	r := httptest.NewRequest(http.MethodGet, "http://example.com/index.php", nil)
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("X-Forwarded-For", "192.0.2.1")
	w := httptest.NewRecorder()
	r = caddyhttp.PrepareRequest(r, caddy.NewReplacer(), w, oldServer)
	require.Equal(t, "127.0.0.1", caddyhttp.GetVar(r.Context(), caddyhttp.ClientIPVarKey))
	r = caddyhttp.PrepareRequest(r, caddy.NewReplacer(), w, newServer)
	require.Equal(t, "192.0.2.1", caddyhttp.GetVar(r.Context(), caddyhttp.ClientIPVarKey))
	require.Equal(t, true, caddyhttp.GetVar(r.Context(), caddyhttp.TrustedProxyVarKey))
}

func TestReloadHandoffKeepsUnchangedMiddleware(t *testing.T) {
	for _, middleware := range []string{"encode zstd br gzip\n"} {
		t.Run(middleware, func(t *testing.T) {
			oldRoot, newRoot := reloadSecurityRoots(t)
			hash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
			require.NoError(t, err)
			routes := func(root string) string {
				return fmt.Sprintf("route /index.php {\n%sbasic_auth {\nalice %s\n}\nphp_server {\nroot %s\n}\n}", middleware, hash, root)
			}
			_, oldServer := reloadSecurityLoad(t, routes(oldRoot))
			for range 2 {
				_, _ = reloadSecurityLoad(t, routes(newRoot))
				request := httptest.NewRequest(http.MethodGet, "http://example.com/index.php", nil)
				request.SetBasicAuth("alice", "password")
				w := reloadSecurityServe(oldServer, request)
				require.Equal(t, http.StatusOK, w.Code)
				require.Equal(t, "new:/index.php:", w.Body.String())
			}
		})
	}
}

func TestReloadHandoffRefusesUnchangedMiddleware(t *testing.T) {
	for _, middleware := range []string{"request_header X-Unchanged yes\n"} {
		t.Run(middleware, func(t *testing.T) {
			oldRoot, newRoot := reloadSecurityRoots(t)
			hash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
			require.NoError(t, err)
			routes := func(root string) string {
				return fmt.Sprintf("route /index.php {\n%sbasic_auth {\nalice %s\n}\nphp_server {\nroot %s\n}\n}", middleware, hash, root)
			}
			_, oldServer := reloadSecurityLoad(t, routes(oldRoot))
			for range 2 {
				_, _ = reloadSecurityLoad(t, routes(newRoot))
				request := httptest.NewRequest(http.MethodGet, "http://example.com/index.php", nil)
				request.SetBasicAuth("alice", "password")
				w := reloadSecurityServe(oldServer, request)
				require.Equal(t, http.StatusServiceUnavailable, w.Code)
				require.Empty(t, w.Body.String())
			}
		})
	}
}

func TestReloadHandoffWorkerMatchUsesReplacementRoot(t *testing.T) {
	for _, changeRoot := range []bool{false, true} {
		t.Run(fmt.Sprintf("change_root=%t", changeRoot), func(t *testing.T) {
			oldRoot, newRoot := reloadSecurityRoots(t)
			for _, root := range []string{oldRoot, newRoot} {
				generation := filepath.Base(root)
				script := fmt.Sprintf(`<?php while (frankenphp_handle_request(static function () { echo '%s-worker:' . $_SERVER['REQUEST_URI']; })) {}`, generation)
				require.NoError(t, os.WriteFile(filepath.Join(root, "worker.php"), []byte(script), 0600))
				require.NoError(t, os.WriteFile(filepath.Join(root, "asset.txt"), []byte(generation+"-static"), 0600))
			}
			if !changeRoot {
				newRoot = oldRoot
			}
			routes := func(root string) string {
				return fmt.Sprintf("php_server {\nroot %s\nworker worker.php {\nnum 1\nmatch /worker/*\n}\n}", root)
			}
			old, oldServer := reloadSecurityLoad(t, routes(oldRoot))
			next, newServer := reloadSecurityLoad(t, routes(newRoot))
			require.Len(t, old.modules, 2)
			for i, module := range old.modules {
				oldKey, _ := module.reloadKey()
				newKey, _ := next.modules[i].reloadKey()
				require.NotEmpty(t, oldKey)
				require.Equal(t, oldKey, newKey)
			}
			for _, path := range []string{"/worker/request", "/index.php"} {
				direct := reloadSecurityServe(newServer, httptest.NewRequest(http.MethodGet, "http://example.com"+path, nil))
				handoff := reloadSecurityServe(oldServer, httptest.NewRequest(http.MethodGet, "http://example.com"+path, nil))
				require.Equal(t, http.StatusOK, direct.Code)
				require.Equal(t, http.StatusOK, handoff.Code)
				require.Equal(t, direct.Body.String(), handoff.Body.String())
				require.Contains(t, handoff.Body.String(), filepath.Base(newRoot))
			}
			// The new generated file guard must still serve existing static files.
			direct := reloadSecurityServe(newServer, httptest.NewRequest(http.MethodGet, "http://example.com/asset.txt", nil))
			require.Equal(t, filepath.Base(newRoot)+"-static", direct.Body.String())
		})
	}
}

type reloadCancelMatcher struct{ cancel context.CancelFunc }

func (*reloadCancelMatcher) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.matchers.reload_cancel",
		New: func() caddy.Module { return new(reloadCancelMatcher) },
	}
}

func (*reloadCancelMatcher) Match(*http.Request) bool { return true }

func (m *reloadCancelMatcher) MarshalJSON() ([]byte, error) {
	if m.cancel != nil {
		m.cancel()
	}
	return []byte(`{}`), nil
}

func TestReloadHandoffRejectsCancellationDuringKeyWalk(t *testing.T) {
	oldRoot, newRoot := reloadSecurityRoots(t)
	routes := func(root string) string { return fmt.Sprintf("php /index.php {\nroot %s\n}", root) }
	_, oldServer := reloadSecurityLoad(t, routes(oldRoot))
	next, newServer := reloadSecurityLoad(t, routes(newRoot))
	ctx, cancel := context.WithCancel(next.ctx)
	t.Cleanup(cancel)
	next.ctx = ctx
	for _, server := range []*caddyhttp.Server{oldServer, newServer} {
		server.Routes[0].MatcherSets[0] = append(server.Routes[0].MatcherSets[0], &reloadCancelMatcher{cancel: cancel})
	}
	require.NoError(t, next.ctx.Err())
	w := reloadSecurityServe(oldServer, httptest.NewRequest(http.MethodGet, "http://example.com/index.php", nil))
	require.ErrorIs(t, next.ctx.Err(), context.Canceled)
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	require.NotContains(t, w.Body.String(), "new:")
}

func TestReloadHandoffContainsReplayErrors(t *testing.T) {
	oldRoot, newRoot := reloadSecurityRoots(t)
	routes := func(root, auth string) string {
		return fmt.Sprintf("route /index.php {\n%sphp {\nroot %s\n}\n}", auth, root)
	}
	_, oldServer := reloadSecurityLoad(t, routes(oldRoot, "")+"\nhandle_errors {\nrespond RETIRED-ERROR-POLICY 403\n}")
	hash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	require.NoError(t, err)
	_, newServer := reloadSecurityLoad(t, routes(newRoot, fmt.Sprintf("basic_auth {\nalice %s\n}\n", hash)))
	request := func() *http.Request { return httptest.NewRequest(http.MethodGet, "http://example.com/index.php", nil) }
	require.Equal(t, http.StatusUnauthorized, reloadSecurityServe(newServer, request()).Code)
	w := reloadSecurityServe(oldServer, request())
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	require.Empty(t, w.Body.String())
}

type reloadUnknownHandler struct {
	Value any `json:"value"`
}

func (*reloadUnknownHandler) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.reload_unknown",
		New: func() caddy.Module { return new(reloadUnknownHandler) },
	}
}

func (*reloadUnknownHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	return next.ServeHTTP(w, r)
}

func TestReloadKeyUnknownMiddlewareContentAndMarshalFailure(t *testing.T) {
	key := func(value any, app *FrankenPHPApp) string {
		module := &FrankenPHPModule{app: app}
		app.httpApp = &caddyhttp.App{Servers: map[string]*caddyhttp.Server{"srv0": {Routes: caddyhttp.RouteList{
			{Handlers: []caddyhttp.MiddlewareHandler{&reloadUnknownHandler{Value: value}}},
			{Handlers: []caddyhttp.MiddlewareHandler{module}},
		}}}}
		result, _ := module.reloadKey()
		return result
	}
	require.Equal(t, key("same", new(FrankenPHPApp)), key("same", new(FrankenPHPApp)))
	require.NotEqual(t, key("old", new(FrankenPHPApp)), key("new", new(FrankenPHPApp)))
	require.NotEqual(t, key(make(chan int), new(FrankenPHPApp)), key(make(chan int), new(FrankenPHPApp)))
}

func TestReloadWorkerFileGuardKeepsIndependentRoots(t *testing.T) {
	key := func(guard string) string {
		module := &FrankenPHPModule{Root: "/app", ServerIndex: 1, Workers: []workerConfig{{MatchPath: []string{"/worker/*"}}}}
		return reloadRouteKey(caddyhttp.RouteList{
			{MatcherSets: caddyhttp.MatcherSets{{&fileserver.MatchFile{Root: guard, TryFiles: []string{"{http.request.uri.path}"}}}},
				Handlers: []caddyhttp.MiddlewareHandler{&fileserver.FileServer{Root: "/app"}}},
			{Handlers: []caddyhttp.MiddlewareHandler{module}},
		}, module)
	}
	require.NotEqual(t, key("/guards/public"), key("/guards/private"))
}

func TestReloadHandoffIsolatesRetiredResponsePolicies(t *testing.T) {
	for _, policy := range []string{"replace_status", "handle_response"} {
		for _, deny := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/deny=%t", policy, deny), func(t *testing.T) {
				oldRoot, newRoot := reloadSecurityRoots(t)
				interceptor := "intercept {\nreplace_status 200\n}\n"
				if policy == "handle_response" {
					interceptor = "intercept {\nhandle_response {\nfile_server\n}\n}\n"
				}
				routes := func(root, control string) string {
					return fmt.Sprintf("route /index.php {\nroot * %s\n%s%sphp {\nroot %s\n}\n}", root, interceptor, control, root)
				}
				old, oldServer := reloadSecurityLoad(t, routes(oldRoot, ""))
				hash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
				require.NoError(t, err)
				control := fmt.Sprintf("basic_auth {\nalice %s\n}\n", hash)
				status, body := http.StatusServiceUnavailable, ""
				if deny {
					// Keep the new policy outside the new interceptor so only
					// the retired response policy could rewrite its response.
					control = "respond NEW-DENY 403\n"
					status, body = http.StatusForbidden, "NEW-DENY"
				}
				nextRoutes := fmt.Sprintf("route /index.php {\n%sroot * %s\n%sphp {\nroot %s\n}\n}", control, newRoot, interceptor, newRoot)
				next, newServer := reloadSecurityLoad(t, nextRoutes)
				oldKey, _ := old.modules[0].reloadKey()
				newKey, _ := next.modules[0].reloadKey()
				require.NotEmpty(t, oldKey)
				if policy == "handle_response" {
					require.NotEqual(t, oldKey, newKey, "response routes must refuse handoff")
					status, body = http.StatusServiceUnavailable, ""
				} else {
					require.Equal(t, oldKey, newKey, "replace_status-only must keep replay")
				}
				request := func() *http.Request { return httptest.NewRequest(http.MethodGet, "http://example.com/index.php", nil) }
				direct := reloadSecurityServe(newServer, request())
				if deny {
					require.Equal(t, http.StatusForbidden, direct.Code)
					require.Equal(t, "NEW-DENY", direct.Body.String())
				} else {
					require.Equal(t, http.StatusUnauthorized, direct.Code)
				}
				w := reloadSecurityServe(oldServer, request())
				t.Logf("retired response policy returned %d, body %q", w.Code, w.Body.String())
				require.Equal(t, status, w.Code)
				require.Equal(t, body, w.Body.String())
				require.NotContains(t, w.Body.String(), "<?php", "retired file_server must never expose old PHP source")
			})
		}
	}
}

func TestReloadHandoffRunsReplacementResponsePolicy(t *testing.T) {
	oldRoot, newRoot := reloadSecurityRoots(t)
	routes := func(root, generation string) string {
		return fmt.Sprintf(`route /index.php {
    vars generation %s
    intercept {
        replace_status {http.vars.generation}
    }
    php {
        root %s
    }
}`, generation, root)
	}
	_, oldServer := reloadSecurityLoad(t, routes(oldRoot, "201"))
	initial := reloadSecurityServe(oldServer, httptest.NewRequest(http.MethodGet, "http://example.com/index.php", nil))
	require.Equal(t, http.StatusCreated, initial.Code)
	require.Equal(t, "old:/index.php:", initial.Body.String())
	_, newServer := reloadSecurityLoad(t, routes(newRoot, "202"))
	for _, server := range []*caddyhttp.Server{newServer, oldServer} {
		w := reloadSecurityServe(server, httptest.NewRequest(http.MethodGet, "http://example.com/index.php", nil))
		require.Equal(t, http.StatusAccepted, w.Code)
		require.Equal(t, "new:/index.php:", w.Body.String(), "only the replacement response policy may run")
	}
}

type reloadWrittenErrorHandler struct{}

func (*reloadWrittenErrorHandler) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.reload_written_error",
		New: func() caddy.Module { return new(reloadWrittenErrorHandler) },
	}
}

func (*reloadWrittenErrorHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request, _ caddyhttp.Handler) error {
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte("NEW-DENY"))
	return errors.New("error after committing a denial")
}

type reloadCountingWriter struct {
	*httptest.ResponseRecorder
	statuses []int
}

func (w *reloadCountingWriter) WriteHeader(status int) {
	w.statuses = append(w.statuses, status)
	w.ResponseRecorder.WriteHeader(status)
}

func TestReloadHandoffDoesNotOverwriteCommittedDenial(t *testing.T) {
	makeApp := func() (*FrankenPHPApp, *FrankenPHPModule, *caddyhttp.Server) {
		app := &FrankenPHPApp{ctx: context.Background(), started: make(chan any)}
		app.hasStarted.Store(true)
		close(app.started)
		module := &FrankenPHPModule{app: app}
		app.modules = []*FrankenPHPModule{module}
		routes := caddyhttp.RouteList{{Handlers: []caddyhttp.MiddlewareHandler{new(reloadWrittenErrorHandler), module}}}
		require.NoError(t, routes.ProvisionHandlers(caddy.Context{}, nil))
		server := &caddyhttp.Server{Routes: routes}
		app.httpApp = &caddyhttp.App{Servers: map[string]*caddyhttp.Server{"srv0": server}}
		return app, module, server
	}
	_, old, oldServer := makeApp()
	next, _, _ := makeApp()
	w := &reloadCountingWriter{ResponseRecorder: httptest.NewRecorder()}
	wrapped := &caddyhttp.ResponseWriterWrapper{ResponseWriter: &caddyhttp.ResponseWriterWrapper{ResponseWriter: w}}
	r := caddyhttp.PrepareRequest(httptest.NewRequest(http.MethodGet, "http://example.com/index.php", nil), caddy.NewReplacer(), wrapped, oldServer)
	require.NoError(t, old.requestReload(wrapped, r, next))
	require.Equal(t, []int{http.StatusForbidden}, w.statuses, "an already-written denial must not trigger a superfluous 503")
	require.Equal(t, "NEW-DENY", w.Body.String())
}

func TestReloadHandoffRejectsForwardAuthStaleAuthorization(t *testing.T) {
	oldRoot, newRoot := reloadSecurityRoots(t)
	var calls atomic.Int32
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Authorization", "Basic YWxpY2U6cGFzc3dvcmQ=")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(authServer.Close)
	hash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	require.NoError(t, err)
	routes := func(root, copiedHeader, auth string) string {
		return fmt.Sprintf("route /index.php {\nforward_auth %s {\nuri /auth\ncopy_headers %s\n}\n%sphp {\nroot %s\n}\n}", authServer.URL, copiedHeader, auth, root)
	}
	old, oldServer := reloadSecurityLoad(t, routes(oldRoot, "Authorization", ""))
	request := func() *http.Request { return httptest.NewRequest(http.MethodGet, "http://example.com/index.php", nil) }
	require.Equal(t, "old:/index.php:", reloadSecurityServe(oldServer, request()).Body.String())
	next, newServer := reloadSecurityLoad(t, routes(newRoot, "X-User", fmt.Sprintf("basic_auth {\nalice %s\n}\n", hash)))
	proxy := func(server *caddyhttp.Server) *reverseproxy.Handler {
		var found *reverseproxy.Handler
		var walk func(caddyhttp.RouteList)
		walk = func(routes caddyhttp.RouteList) {
			for _, route := range routes {
				for _, h := range route.Handlers {
					switch h := h.(type) {
					case *reverseproxy.Handler:
						found = h
					case *caddyhttp.Subroute:
						walk(h.Routes)
					}
				}
			}
		}
		walk(server.Routes)
		require.NotNil(t, found)
		return found
	}
	oldJSON, err := json.Marshal(proxy(oldServer))
	require.NoError(t, err)
	newJSON, err := json.Marshal(proxy(newServer))
	require.NoError(t, err)
	require.JSONEq(t, string(oldJSON), string(newJSON), "provisioned copy_headers changes are invisible to JSON keying")
	require.Equal(t, http.StatusUnauthorized, reloadSecurityServe(newServer, request()).Code)
	w := reloadSecurityServe(oldServer, request())
	t.Logf("stale Authorization returned %d, body %q", w.Code, w.Body.String())
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	require.Empty(t, w.Body.String())
	require.EqualValues(t, 3, calls.Load(), "the refused handoff must not replay forward_auth")
	oldKey, _ := old.modules[0].reloadKey()
	newKey, _ := next.modules[0].reloadKey()
	require.NotEqual(t, oldKey, newKey)
}

func TestReloadHandoffKeepsOrdinaryReverseProxy(t *testing.T) {
	oldRoot, newRoot := reloadSecurityRoots(t)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("API")) }))
	t.Cleanup(api.Close)
	routes := func(root string) string {
		return fmt.Sprintf("route {\nreverse_proxy /api/* %s\nphp /index.php {\nroot %s\n}\n}", api.URL, root)
	}
	old, oldServer := reloadSecurityLoad(t, routes(oldRoot))
	next, newServer := reloadSecurityLoad(t, routes(newRoot))
	oldKey, _ := old.modules[0].reloadKey()
	newKey, _ := next.modules[0].reloadKey()
	require.NotEmpty(t, oldKey)
	require.Equal(t, oldKey, newKey)
	for _, server := range []*caddyhttp.Server{newServer, oldServer} {
		w := reloadSecurityServe(server, httptest.NewRequest(http.MethodGet, "http://example.com/index.php", nil))
		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, "new:/index.php:", w.Body.String())
		w = reloadSecurityServe(server, httptest.NewRequest(http.MethodGet, "http://example.com/api/request", nil))
		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, "API", w.Body.String())
	}
}

func TestReloadHandoffRejectsProvisionedTrustChange(t *testing.T) {
	oldRoot, newRoot := reloadSecurityRoots(t)
	routes := func(root, deny string) string {
		return fmt.Sprintf(`@external not client_ip 127.0.0.1/32
route /index.php {
    %s
    php {
        root %s
    }
}`, deny, root)
	}
	require.NoError(t, caddy.Load(reloadSecurityServerConfig(t, routes(oldRoot, ""), func(s *caddyhttp.Server) {
		s.ClientIPHeaders = []string{"CF-Connecting-IP"}
	}), true))
	old := activeApp.Load()
	oldServer := old.httpApp.Servers["srv0"]
	newConfig := reloadSecurityServerConfig(t, routes(newRoot, "respond @external EXTERNAL-DENY 403"), func(s *caddyhttp.Server) {
		s.TrustedProxiesRaw = json.RawMessage(`{"source":"static","ranges":["127.0.0.1/32"]}`)
		s.ClientIPHeaders = []string{"CF-Connecting-IP"}
	})
	require.NoError(t, caddy.Load(newConfig, true))
	next := activeApp.Load()
	newServer := next.httpApp.Servers["srv0"]
	require.Empty(t, oldServer.TrustedProxiesRaw)
	require.Empty(t, newServer.TrustedProxiesRaw, "provisioning must clear the raw trust field")
	request := func() *http.Request {
		r := httptest.NewRequest(http.MethodGet, "http://example.com/index.php", nil)
		r.RemoteAddr = "127.0.0.1:1234"
		r.Header.Set("CF-Connecting-IP", "192.0.2.1")
		return r
	}
	direct := reloadSecurityServe(newServer, request())
	require.Equal(t, http.StatusForbidden, direct.Code)
	require.Equal(t, "EXTERNAL-DENY", direct.Body.String())
	w := reloadSecurityServe(oldServer, request())
	t.Logf("changed trust returned %d, body %q", w.Code, w.Body.String())
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	require.Empty(t, w.Body.String())
	oldKey, _ := old.modules[0].reloadKey()
	newKey, _ := next.modules[0].reloadKey()
	require.NotEqual(t, oldKey, newKey)
	// Static trust fingerprints must remain stable on an equivalent reload.
	require.NoError(t, caddy.Load(newConfig, true))
	unchangedKey, _ := activeApp.Load().modules[0].reloadKey()
	require.Equal(t, newKey, unchangedKey)
	w = reloadSecurityServe(newServer, request())
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Equal(t, "EXTERNAL-DENY", w.Body.String())
}

func TestReloadHandoffRejectsUnchangedAuthorizationInjector(t *testing.T) {
	oldRoot, newRoot := reloadSecurityRoots(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	require.NoError(t, err)
	routes := func(root, auth string) string {
		return fmt.Sprintf("route /index.php {\n%srequest_header Authorization \"Basic YWxpY2U6cGFzc3dvcmQ=\"\nphp {\nroot %s\n}\n}", auth, root)
	}
	old, oldServer := reloadSecurityLoad(t, routes(oldRoot, ""))
	request := func() *http.Request { return httptest.NewRequest(http.MethodGet, "http://example.com/index.php", nil) }
	require.Equal(t, "old:/index.php:", reloadSecurityServe(oldServer, request()).Body.String())
	next, newServer := reloadSecurityLoad(t, routes(newRoot, fmt.Sprintf("basic_auth {\nalice %s\n}\n", hash)))
	require.Equal(t, http.StatusUnauthorized, reloadSecurityServe(newServer, request()).Code)
	w := reloadSecurityServe(oldServer, request())
	t.Logf("unchanged Authorization injector returned %d, body %q", w.Code, w.Body.String())
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	require.Empty(t, w.Body.String())
	oldKey, _ := old.modules[0].reloadKey()
	newKey, _ := next.modules[0].reloadKey()
	require.NotEqual(t, oldKey, newKey)
}

type reloadInjectAuthenticator struct{ calls atomic.Int32 }

func (*reloadInjectAuthenticator) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.authentication.providers.reload_inject",
		New: func() caddy.Module { return new(reloadInjectAuthenticator) },
	}
}

func (a *reloadInjectAuthenticator) Authenticate(_ http.ResponseWriter, r *http.Request) (caddyauth.User, bool, error) {
	a.calls.Add(1)
	r.SetBasicAuth("alice", "password")
	return caddyauth.User{ID: "alice"}, true, nil
}

func init() { caddy.RegisterModule(new(reloadInjectAuthenticator)) }

func TestReloadHandoffRejectsThirdPartyAuthorizationInjector(t *testing.T) {
	oldRoot, newRoot := reloadSecurityRoots(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	require.NoError(t, err)
	routes := func(root string) string {
		return fmt.Sprintf("route /index.php {\nbasic_auth {\nalice %s\n}\nphp {\nroot %s\n}\n}", hash, root)
	}
	var config any
	require.NoError(t, json.Unmarshal(reloadSecurityConfig(t, routes(oldRoot)), &config))
	var inject func(any)
	inject = func(value any) {
		switch value := value.(type) {
		case map[string]any:
			if value["handler"] == "authentication" {
				value["providers"] = map[string]any{"reload_inject": map[string]any{}}
			}
			for _, child := range value {
				inject(child)
			}
		case []any:
			for _, child := range value {
				inject(child)
			}
		}
	}
	inject(config)
	require.NoError(t, caddy.Load(caddyconfig.JSON(config, nil), true))
	old := activeApp.Load()
	oldServer := old.httpApp.Servers["srv0"]
	var provider *reloadInjectAuthenticator
	var find func(caddyhttp.RouteList)
	find = func(routes caddyhttp.RouteList) {
		for _, route := range routes {
			for _, handler := range route.Handlers {
				switch handler := handler.(type) {
				case *caddyauth.Authentication:
					provider, _ = handler.Providers["reload_inject"].(*reloadInjectAuthenticator)
				case *caddyhttp.Subroute:
					find(handler.Routes)
				}
			}
		}
	}
	find(oldServer.Routes)
	require.NotNil(t, provider)
	request := func() *http.Request { return httptest.NewRequest(http.MethodGet, "http://example.com/index.php", nil) }
	require.Equal(t, "old:/index.php:", reloadSecurityServe(oldServer, request()).Body.String())
	next, newServer := reloadSecurityLoad(t, routes(newRoot))
	require.Equal(t, http.StatusUnauthorized, reloadSecurityServe(newServer, request()).Code)
	w := reloadSecurityServe(oldServer, request())
	t.Logf("third-party Authorization injector returned %d, body %q", w.Code, w.Body.String())
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	require.Empty(t, w.Body.String())
	require.EqualValues(t, 2, provider.calls.Load(), "the injector must never replay")
	oldKey, _ := old.modules[0].reloadKey()
	newKey, _ := next.modules[0].reloadKey()
	require.NotEqual(t, oldKey, newKey)
}

func TestReloadHandoffEarlyHintsRefusal(t *testing.T) {
	for _, retiredIntercept := range []bool{false, true} {
		t.Run(fmt.Sprintf("retired_intercept=%t", retiredIntercept), func(t *testing.T) {
			oldRoot, newRoot := reloadSecurityRoots(t)
			hash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
			require.NoError(t, err)
			routes := func(root, auth string) string {
				intercept := ""
				if retiredIntercept {
					intercept = "intercept {\nreplace_status 200\n}\n"
				}
				// Hints precede the interceptor in the retired chain. Its recorder
				// sees no status when replay bypasses it, reproducing WriteHeader(0).
				return fmt.Sprintf("route /index.php {\nrespond 103\n%s%sphp {\nroot %s\n}\n}", intercept, auth, root)
			}
			old, oldServer := reloadSecurityLoad(t, routes(oldRoot, ""))
			next, newServer := reloadSecurityLoad(t, routes(newRoot, fmt.Sprintf("basic_auth {\nalice %s\n}\n", hash)))
			oldKey, _ := old.modules[0].reloadKey()
			newKey, _ := next.modules[0].reloadKey()
			require.NotEmpty(t, oldKey)
			require.Equal(t, oldKey, newKey, "this must exercise replay, not key refusal")
			for name, fixture := range map[string]struct {
				server *caddyhttp.Server
				status int
				hints  int
			}{"direct": {newServer, http.StatusUnauthorized, 1}, "retired": {oldServer, http.StatusServiceUnavailable, 2}} {
				t.Run(name, func(t *testing.T) {
					wire := httptest.NewServer(fixture.server)
					defer wire.Close()
					client := wire.Client()
					client.Timeout = 5 * time.Second
					var hints []int
					trace := &httptrace.ClientTrace{Got1xxResponse: func(code int, _ textproto.MIMEHeader) error {
						hints = append(hints, code)
						return nil
					}}
					request, err := http.NewRequestWithContext(httptrace.WithClientTrace(context.Background(), trace), http.MethodGet, wire.URL+"/index.php", nil)
					require.NoError(t, err)
					request.Host = "example.com"
					response, err := client.Do(request)
					require.NoError(t, err, "a retired interceptor must not panic and abort the connection")
					defer func() { require.NoError(t, response.Body.Close()) }()
					body, err := io.ReadAll(response.Body)
					require.NoError(t, err)
					t.Logf("wire statuses: %v then %d, body %q", hints, response.StatusCode, body)
					require.Len(t, hints, fixture.hints)
					for _, status := range hints {
						require.Equal(t, http.StatusEarlyHints, status)
					}
					require.Equal(t, fixture.status, response.StatusCode)
					require.Empty(t, body)
				})
			}
		})
	}
}

func TestReloadKeyAuthorizationPoisonScope(t *testing.T) {
	key := func(h caddyhttp.MiddlewareHandler) string {
		app := new(FrankenPHPApp)
		module := &FrankenPHPModule{app: app}
		app.httpApp = &caddyhttp.App{Servers: map[string]*caddyhttp.Server{"srv0": {Routes: caddyhttp.RouteList{{Handlers: []caddyhttp.MiddlewareHandler{h, module}}}}}}
		result, _ := module.reloadKey()
		return result
	}
	for name, ops := range map[string]*headers.HeaderOps{
		"add":              {Add: http.Header{"aUtHoRiZaTiOn": {"injected"}}},
		"set":              {Set: http.Header{"AUTHORIZATION": {"injected"}}},
		"replace":          {Replace: map[string][]headers.Replacement{"authorization": {{Search: "old", Replace: "new"}}}},
		"delete":           {Delete: []string{"aUtHoRiZaTiOn"}},
		"forwarded delete": {Delete: []string{"X-Forwarded-For"}},
		"ordinary header":  {Set: http.Header{"X-Unchanged": {"yes"}}},
	} {
		t.Run(name, func(t *testing.T) {
			h := &headers.Handler{Request: ops}
			require.NotEqual(t, key(h), key(h), "request header mutations must poison even unchanged configuration")
		})
	}
	for name, h := range map[string]caddyhttp.MiddlewareHandler{
		"response Authorization": &headers.Handler{Response: &headers.RespHeaderOps{HeaderOps: &headers.HeaderOps{Set: http.Header{"Authorization": {"response"}}}}},
		"builtin basic":          &caddyauth.Authentication{Providers: map[string]caddyauth.Authenticator{"http_basic": new(caddyauth.HTTPBasicAuth)}},
	} {
		t.Run(name, func(t *testing.T) { require.Equal(t, key(h), key(h)) })
	}
	for _, providers := range []map[string]caddyauth.Authenticator{
		{"reload_inject": new(reloadInjectAuthenticator)},
		{"http_basic": new(caddyauth.HTTPBasicAuth), "reload_inject": new(reloadInjectAuthenticator)},
	} {
		h := &caddyauth.Authentication{Providers: providers}
		require.NotEqual(t, key(h), key(h), "every non-http_basic provider must poison")
	}
}

type reloadDeadlineWriter struct {
	*httptest.ResponseRecorder
	chunks    []int
	deadlines []time.Time
}

func (w *reloadDeadlineWriter) Write(p []byte) (int, error) {
	w.chunks = append(w.chunks, len(p))
	return w.ResponseRecorder.Write(p)
}

func (w *reloadDeadlineWriter) SetWriteDeadline(deadline time.Time) error {
	w.deadlines = append(w.deadlines, deadline)
	return nil
}

// Maintainer-accepted (m)-family residual: handed-off writes bypass the retired
// WriteIdleTimeout, WriteMinRate, and MaxWriteChunk enforcement. Unwrapping to
// the base writer also removes response processors below route-level timeouts.
func TestReloadHandoffBypassesRetiredWriteIdleEnforcement(t *testing.T) {
	makeApp := func() (*FrankenPHPApp, *FrankenPHPModule, *caddyhttp.Server) {
		app := &FrankenPHPApp{ctx: context.Background(), started: make(chan any)}
		app.hasStarted.Store(true)
		close(app.started)
		module := &FrankenPHPModule{app: app}
		app.modules = []*FrankenPHPModule{module}
		routes := caddyhttp.RouteList{{Handlers: []caddyhttp.MiddlewareHandler{new(reloadWrittenErrorHandler), module}}}
		require.NoError(t, routes.ProvisionHandlers(caddy.Context{}, nil))
		server := &caddyhttp.Server{Routes: routes}
		app.httpApp = &caddyhttp.App{Servers: map[string]*caddyhttp.Server{"srv0": server}}
		return app, module, server
	}
	_, old, oldServer := makeApp()
	next, _, _ := makeApp()
	w := &reloadDeadlineWriter{ResponseRecorder: httptest.NewRecorder()}
	start := time.Now()
	idle := &caddyhttp.IdleTimeoutWriter{
		ResponseWriterWrapper: &caddyhttp.ResponseWriterWrapper{ResponseWriter: w},
		Ctrl:                  http.NewResponseController(w),
		Deadline:              caddyhttp.IdleDeadline{Start: start, Timeout: time.Second, MinRate: 2},
		MaxChunk:              2,
		Logger:                zap.NewNop(),
	}
	wrapped := &caddyhttp.ResponseWriterWrapper{ResponseWriter: caddyhttp.NewResponseRecorder(idle, nil, nil)}
	r := caddyhttp.PrepareRequest(httptest.NewRequest(http.MethodGet, "http://example.com/index.php", nil), caddy.NewReplacer(), wrapped, oldServer)
	require.NoError(t, old.requestReload(wrapped, r, next))
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Equal(t, "NEW-DENY", w.Body.String())
	require.Equal(t, []int{len("NEW-DENY")}, w.chunks, "replayed writes bypass the retired MaxWriteChunk")
	require.Empty(t, w.deadlines, "replayed writes bypass the retired idle timeout and minimum rate")
}

func TestReloadHandoffRejectsDynamicAuthorizationInjector(t *testing.T) {
	for name, mutation := range map[string]string{
		"placeholder name": `request_header {http.request.header.x-key} "Basic YWxpY2U6cGFzc3dvcmQ="`,
		"wildcard replace": `request_header @all * "ping" "Basic YWxpY2U6cGFzc3dvcmQ="`,
	} {
		t.Run(name, func(t *testing.T) {
			oldRoot, newRoot := reloadSecurityRoots(t)
			hash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
			require.NoError(t, err)
			routes := func(root, auth string) string {
				return fmt.Sprintf("@all path *\nroute /index.php {\n%s%s\nphp {\nroot %s\n}\n}", auth, mutation, root)
			}
			request := func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "http://example.com/index.php", nil)
				r.Header.Set("X-Key", "Authorization")
				r.Header.Set("Authorization", "ping")
				return r
			}
			_, oldServer := reloadSecurityLoad(t, routes(oldRoot, ""))
			require.Equal(t, "old:/index.php:", reloadSecurityServe(oldServer, request()).Body.String())
			_, newServer := reloadSecurityLoad(t, routes(newRoot, fmt.Sprintf("basic_auth {\nalice %s\n}\n", hash)))
			require.Equal(t, http.StatusUnauthorized, reloadSecurityServe(newServer, request()).Code)
			w := reloadSecurityServe(oldServer, request())
			t.Logf("retired dynamic Authorization injector: %d, body %q", w.Code, w.Body.String())
			require.Equal(t, http.StatusServiceUnavailable, w.Code)
			require.Empty(t, w.Body.String())
			// A direct authenticated retry still reaches the current runtime.
			r := request()
			r.SetBasicAuth("alice", "password")
			require.Equal(t, "new:/index.php:", reloadSecurityServe(newServer, r).Body.String())
		})
	}
}

func TestReloadHandoffRejectsMutatedClientIP(t *testing.T) {
	for _, mutation := range []string{
		"request_header -X-Forwarded-For",
		"request_header -X-Forwarded-*",
		"request_header X-Forwarded-For 127.0.0.1",
		"request_header +X-Forwarded-For 127.0.0.1",
	} {
		t.Run(mutation, func(t *testing.T) {
			oldRoot, newRoot := reloadSecurityRoots(t)
			routes := func(root, deny string) string {
				return fmt.Sprintf("@external not client_ip 127.0.0.1/32\nroute /index.php {\n%s\n%sphp {\nroot %s\n}\n}", mutation, deny, root)
			}
			load := func(root, deny string) *caddyhttp.Server {
				config := reloadSecurityServerConfig(t, routes(root, deny), func(s *caddyhttp.Server) {
					s.TrustedProxiesRaw = json.RawMessage(`{"source":"static","ranges":["127.0.0.1/32"]}`)
				})
				require.NoError(t, caddy.Load(config, true))
				return activeApp.Load().httpApp.Servers["srv0"]
			}
			request := func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "http://example.com/index.php", nil)
				r.RemoteAddr = "127.0.0.1:1234"
				r.Header.Set("X-Forwarded-For", "192.0.2.1")
				return r
			}
			oldServer := load(oldRoot, "")
			require.Equal(t, "old:/index.php:", reloadSecurityServe(oldServer, request()).Body.String())
			newServer := load(newRoot, "respond @external EXTERNAL-DENY 403\n")
			direct := reloadSecurityServe(newServer, request())
			require.Equal(t, http.StatusForbidden, direct.Code)
			require.Equal(t, "EXTERNAL-DENY", direct.Body.String())
			w := reloadSecurityServe(oldServer, request())
			t.Logf("retired identity mutation: %d, body %q", w.Code, w.Body.String())
			require.Equal(t, http.StatusServiceUnavailable, w.Code)
			require.Empty(t, w.Body.String())
		})
	}
}

func TestReloadHandoffRejectsEarlyHintsInterceptDisclosure(t *testing.T) {
	oldRoot, newRoot := reloadSecurityRoots(t)
	oldSource, err := os.ReadFile(filepath.Join(oldRoot, "index.php"))
	require.NoError(t, err)
	hash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	require.NoError(t, err)
	// The retired interceptor selects its routes on the 103 and would resume
	// after replay to append this old-root PHP source to an already-written 503.
	responsePolicy := func(root string) string {
		return fmt.Sprintf(`intercept {
        handle_response {
            file_server {
                root %s
            }
        }
    }
`, root)
	}
	old, oldServer := reloadSecurityLoad(t, fmt.Sprintf("route /index.php {\n%srespond 103\nphp {\nroot %s\n}\n}", responsePolicy(oldRoot), oldRoot))
	next, newServer := reloadSecurityLoad(t, fmt.Sprintf("route /index.php {\nrespond 103\nbasic_auth {\nalice %s\n}\n%sphp {\nroot %s\n}\n}", hash, responsePolicy(newRoot), newRoot))
	oldKey, _ := old.modules[0].reloadKey()
	newKey, _ := next.modules[0].reloadKey()
	t.Logf("intercept handoff keys match: %t", oldKey == newKey)
	for name, fixture := range map[string]struct {
		server *caddyhttp.Server
		status int
	}{"direct": {newServer, http.StatusUnauthorized}, "retired": {oldServer, http.StatusServiceUnavailable}} {
		t.Run(name, func(t *testing.T) {
			wire := httptest.NewServer(fixture.server)
			defer wire.Close()
			client := wire.Client()
			client.Timeout = 5 * time.Second
			var hints []int
			trace := &httptrace.ClientTrace{Got1xxResponse: func(code int, _ textproto.MIMEHeader) error {
				hints = append(hints, code)
				return nil
			}}
			request, err := http.NewRequestWithContext(httptrace.WithClientTrace(context.Background(), trace), http.MethodGet, wire.URL+"/index.php", nil)
			require.NoError(t, err)
			request.Host = "example.com"
			response, err := client.Do(request)
			require.NoError(t, err)
			defer func() { require.NoError(t, response.Body.Close()) }()
			body, err := io.ReadAll(response.Body)
			require.NoError(t, err)
			t.Logf("wire hints %v, status %d, body %q", hints, response.StatusCode, body)
			require.Equal(t, fixture.status, response.StatusCode)
			require.Empty(t, body, "retired handle_response routes must never write bytes after refusal")
			require.NotContains(t, string(body), string(oldSource))
			require.Equal(t, []int{http.StatusEarlyHints}, hints)
		})
	}
	require.NotEqual(t, oldKey, newKey, "response routes must poison unchanged intercepts")
}

func TestReloadKeyIdentityAndResponsePoisonScope(t *testing.T) {
	key := func(h caddyhttp.MiddlewareHandler) string {
		app := new(FrankenPHPApp)
		module := &FrankenPHPModule{app: app}
		app.httpApp = &caddyhttp.App{Servers: map[string]*caddyhttp.Server{"srv0": {Routes: caddyhttp.RouteList{{Handlers: []caddyhttp.MiddlewareHandler{h, module}}}}}}
		result, _ := module.reloadKey()
		return result
	}
	for _, name := range []string{"authorization", "fOrWaRdEd", "x-forwarded-for", "X-Forwarded-Host", "X-Real-IP", "{http.request.header.x-key}", "*", "X-Forwarded-*"} {
		for operation, ops := range map[string]*headers.HeaderOps{
			"add":     {Add: http.Header{name: {"injected"}}},
			"set":     {Set: http.Header{name: {"injected"}}},
			"replace": {Replace: map[string][]headers.Replacement{name: {{Search: "old", Replace: "new"}}}},
			"delete":  {Delete: []string{name}},
		} {
			t.Run(name+"/"+operation, func(t *testing.T) {
				h := &headers.Handler{Request: ops}
				require.NotEqual(t, key(h), key(h), "request identity mutations must poison unchanged configuration")
			})
		}
	}
	for _, routes := range []caddyhttp.RouteList{nil, {}, {{}}} {
		h := &intercept.Intercept{HandleResponse: []caddyhttp.ResponseHandler{{StatusCode: "200"}, {Routes: routes}}}
		if routes == nil {
			require.Equal(t, key(h), key(h), "replace_status-only intercepts keep handoff")
		} else {
			require.NotEqual(t, key(h), key(h), "even empty non-nil response routes must poison")
		}
	}
	// Custom client identity headers must also refuse unchanged handoff.
	h := &headers.Handler{Request: &headers.HeaderOps{Delete: []string{"CF-Connecting-IP"}}}
	require.NotEqual(t, key(h), key(h))
}

func TestReloadHandoffRequestHeaderMutationsFailClosed(t *testing.T) {
	for _, mutation := range []struct {
		name, header, value, matcher string
	}{
		{"ordinary header", "X-Access", "allowed", "header X-Access denied"},
		{"Host field", "Host", "internal.example.com", "host example.com"},
	} {
		for _, addControl := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/added_control=%t", mutation.name, addControl), func(t *testing.T) {
				root, _ := reloadSecurityRoots(t)
				script := `<?php echo $_SERVER['HTTP_HOST'] . ':' . $_SERVER['HTTP_X_ACCESS'];`
				require.NoError(t, os.WriteFile(filepath.Join(root, "index.php"), []byte(script), 0600))
				phpResponse := "example.com:allowed"
				if mutation.header == "Host" {
					phpResponse = "internal.example.com:denied"
				}
				routes := func(control string) string {
					return fmt.Sprintf(`@denied %s
route /index.php {
    %s
    request_header %s %s
    php {
        root %s
    }
}`, mutation.matcher, control, mutation.header, mutation.value, root)
				}
				request := func() *http.Request {
					r := httptest.NewRequest(http.MethodGet, "http://example.com/index.php", nil)
					r.Header.Set("X-Access", "denied")
					return r
				}
				old, oldServer := reloadSecurityLoad(t, routes(""))
				initial := reloadSecurityServe(oldServer, request())
				require.Equal(t, http.StatusOK, initial.Code)
				require.Equal(t, phpResponse, initial.Body.String(), "PHP must observe the mutated header or r.Host")
				control := ""
				if addControl {
					// The control sees the original identity before the unchanged mutation.
					control = "route @denied {\nrespond NEW-DENY-POLICY 403\n}\n"
				}
				// Keep the PHP root and every other setting identical across reloads.
				next, newServer := reloadSecurityLoad(t, routes(control))
				oldKey, _ := old.modules[0].reloadKey()
				newKey, _ := next.modules[0].reloadKey()
				require.NotEmpty(t, oldKey)
				require.NotEmpty(t, newKey)
				require.NotEqual(t, oldKey, newKey, "even without a new control, unchanged request ops refuse handoff")
				retired := reloadSecurityServe(oldServer, request())
				require.Equal(t, http.StatusServiceUnavailable, retired.Code)
				require.Empty(t, retired.Body.String())
				// A fresh retry reaches the replacement server with its original identity.
				retry := reloadSecurityServe(newServer, request())
				if addControl {
					require.Equal(t, http.StatusForbidden, retry.Code)
					require.Equal(t, "NEW-DENY-POLICY", retry.Body.String())
				} else {
					require.Equal(t, http.StatusOK, retry.Code)
					require.Equal(t, phpResponse, retry.Body.String())
				}
			})
		}
	}
}

func TestReloadKeyRequestHeaderBoundary(t *testing.T) {
	key := func(h *headers.Handler) string {
		app := new(FrankenPHPApp)
		module := &FrankenPHPModule{app: app}
		app.httpApp = &caddyhttp.App{Servers: map[string]*caddyhttp.Server{"srv0": {Routes: caddyhttp.RouteList{{Handlers: []caddyhttp.MiddlewareHandler{h, module}}}}}}
		result, _ := module.reloadKey()
		return result
	}
	h := &headers.Handler{}
	require.Equal(t, key(h), key(h), "nil request ops remain equivalent")
	h.Request = &headers.HeaderOps{}
	require.NotEqual(t, key(h), key(h), "even empty non-nil request ops refuse equivalence")
	h.Request = nil
	h.Response = &headers.RespHeaderOps{HeaderOps: &headers.HeaderOps{Set: http.Header{"X-Response": {"old"}}}}
	before := key(h)
	require.Equal(t, before, key(h), "unchanged response-only ops remain equivalent")
	h.Response.Set.Set("X-Response", "new")
	require.NotEqual(t, before, key(h), "response-only ops remain keyed by content")
	h.Request = &headers.HeaderOps{}
	require.NotEqual(t, key(h), key(h), "response ops do not exempt non-nil request ops")
}

// Maintainer-accepted residuals in reloadRouteKeys:
//   - encode's If-None-Match normalization survives replay, but no realistic
//     access control keys on a client-supplied ETag; poisoning encode would
//     refuse handoff for the shipped default Caddyfile, an excluded tradeoff.
//   - Third-party request-mutating middleware remains content-keyed: stale
//     mutations can feed newly added respond-deny controls. Poisoning every
//     non-whitelisted handler would refuse unchanged third-party configs,
//     which the maintainer's availability guidance explicitly declines.
//   - reloadKey's map iteration is nondeterministic when a module is reachable
//     from multiple route lists (unreachable from Caddyfile/JSON configs).
//     Both outcomes are safe: correct replay or retryable refusal.

func TestReloadHandoffRequestBodySetFailsClosed(t *testing.T) {
	root, _ := reloadSecurityRoots(t)
	routes := func(control string) string {
		return fmt.Sprintf(`@denied vars {http.request.body} blocked
route /index.php {
    %s
    request_body {
        set allowed
    }
    php {
        root %s
    }
}`, control, root)
	}
	request := func() *http.Request {
		return httptest.NewRequest(http.MethodPost, "http://example.com/index.php", strings.NewReader("blocked"))
	}
	old, oldServer := reloadSecurityLoad(t, routes(""))
	initial := reloadSecurityServe(oldServer, request())
	require.Equal(t, http.StatusOK, initial.Code)
	require.Equal(t, "old:/index.php:allowed", initial.Body.String(), "the retired chain replaces the original body before PHP")
	// The replacement's new deny must see the original body before the
	// unchanged setter closes and discards it.
	next, newServer := reloadSecurityLoad(t, routes("route @denied {\nrespond NEW-DENY-POLICY 403\n}"))
	oldKey, _ := old.modules[0].reloadKey()
	newKey, _ := next.modules[0].reloadKey()
	require.NotEmpty(t, oldKey)
	require.NotEmpty(t, newKey)
	require.NotEqual(t, oldKey, newKey)
	retired := reloadSecurityServe(oldServer, request())
	require.Equal(t, http.StatusServiceUnavailable, retired.Code)
	require.Empty(t, retired.Body.String())
	retry := reloadSecurityServe(newServer, request())
	require.Equal(t, http.StatusForbidden, retry.Code)
	require.Equal(t, "NEW-DENY-POLICY", retry.Body.String())
}

func TestReloadHandoffRequestBodyMaxSizeKeepsHandoff(t *testing.T) {
	root, _ := reloadSecurityRoots(t)
	routes := fmt.Sprintf(`route /index.php {
    request_body {
        max_size 1KB
    }
    php {
        root %s
    }
}`, root)
	request := func() *http.Request {
		return httptest.NewRequest(http.MethodPost, "http://example.com/index.php", strings.NewReader("blocked"))
	}
	old, oldServer := reloadSecurityLoad(t, routes)
	next, newServer := reloadSecurityLoad(t, routes)
	oldKey, _ := old.modules[0].reloadKey()
	newKey, _ := next.modules[0].reloadKey()
	require.NotEmpty(t, oldKey)
	require.Equal(t, oldKey, newKey, "max_size only wraps the original reader")
	retired := reloadSecurityServe(oldServer, request())
	require.Equal(t, http.StatusOK, retired.Code)
	require.Equal(t, "old:/index.php:blocked", retired.Body.String())
	retry := reloadSecurityServe(newServer, request())
	require.Equal(t, http.StatusOK, retry.Code)
	require.Equal(t, retired.Body.String(), retry.Body.String())
}

func TestReloadHandoffTemplatesBelowRouteTimeouts(t *testing.T) {
	oldRoot, newRoot := reloadSecurityRoots(t)
	const secret = "round8-secret-must-not-reach-the-wire"
	t.Setenv("FRANKENPHP_RELOAD_ROUND8_SECRET", secret)
	for _, root := range []string{oldRoot, newRoot} {
		require.NoError(t, os.WriteFile(filepath.Join(root, "index.php"), []byte(`<?php header('Content-Type: text/html'); echo $_SERVER['HTTP_X_TEMPLATE'];`), 0600))
	}
	routes := func(root string) string {
		return fmt.Sprintf(`route /index.php {
    templates
    timeouts {
        write_timeout 30s
    }
    php {
        root %s
    }
}`, root)
	}
	// One evaluation emits a literal marker; a retired second evaluation would
	// expose the environment secret. Observe the response on a real connection.
	const payload = `{{"{{env \"FRANKENPHP_RELOAD_ROUND8_SECRET\"}}"}}`
	const harmless = `{{env "FRANKENPHP_RELOAD_ROUND8_SECRET"}}`
	wireResponse := func(server *caddyhttp.Server) string {
		t.Helper()
		wire := httptest.NewServer(server)
		defer wire.Close()
		client := wire.Client()
		client.Timeout = 5 * time.Second
		request, err := http.NewRequest(http.MethodGet, wire.URL+"/index.php", nil)
		require.NoError(t, err)
		request.Host = "example.com"
		request.Header.Set("X-Template", payload)
		response, err := client.Do(request)
		require.NoError(t, err)
		defer func() { require.NoError(t, response.Body.Close()) }()
		body, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)
		require.NotContains(t, string(body), secret)
		return string(body)
	}
	old, oldServer := reloadSecurityLoad(t, routes(oldRoot))
	require.Equal(t, harmless, wireResponse(oldServer), "the original response must evaluate the template exactly once")
	next, newServer := reloadSecurityLoad(t, routes(newRoot))
	oldKey, _ := old.modules[0].reloadKey()
	newKey, _ := next.modules[0].reloadKey()
	require.NotEmpty(t, oldKey)
	require.Equal(t, oldKey, newKey, "this must exercise replay through the replacement templates handler")
	direct := wireResponse(newServer)
	require.Equal(t, harmless, direct)
	require.Equal(t, direct, wireResponse(oldServer), "retired response processors below the timeout writer must not evaluate replayed output")
}

func TestReloadHandoffTracingBaggageFailsClosed(t *testing.T) {
	root, _ := reloadSecurityRoots(t)
	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	t.Setenv("OTEL_PROPAGATORS", "tracecontext,baggage")
	require.NoError(t, os.WriteFile(filepath.Join(root, "index.php"), []byte(`<?php echo $_SERVER['HTTP_BAGGAGE'];`), 0600))
	routes := func(control string) string {
		return fmt.Sprintf(`@denied header Baggage *access=denied*
route /index.php {
    %s
    tracing
    php {
        root %s
    }
}`, control, root)
	}
	request := func() *http.Request {
		r := httptest.NewRequest(http.MethodGet, "http://example.com/index.php", nil)
		r.Header.Set("Baggage", "access=denied,access=allowed")
		return r
	}
	old, oldServer := reloadSecurityLoad(t, routes(""))
	initial := reloadSecurityServe(oldServer, request())
	require.Equal(t, http.StatusOK, initial.Code)
	require.Equal(t, "access=allowed", initial.Body.String(), "tracing normalizes the original header using last-one-wins deduplication")
	// The new control runs before the unchanged tracing handler, and must see
	// the original baggage rather than the retired handler's normalized value.
	next, newServer := reloadSecurityLoad(t, routes("route @denied {\nrespond NEW-DENY-POLICY 403\n}"))
	oldKey, _ := old.modules[0].reloadKey()
	newKey, _ := next.modules[0].reloadKey()
	require.NotEmpty(t, oldKey)
	require.NotEmpty(t, newKey)
	require.NotEqual(t, oldKey, newKey)
	direct := reloadSecurityServe(newServer, request())
	require.Equal(t, http.StatusForbidden, direct.Code)
	require.Equal(t, "NEW-DENY-POLICY", direct.Body.String())
	retired := reloadSecurityServe(oldServer, request())
	require.Equal(t, http.StatusServiceUnavailable, retired.Code)
	require.Empty(t, retired.Body.String())
}

func TestReloadKeyTracingDefaultOrderFailsClosed(t *testing.T) {
	root, _ := reloadSecurityRoots(t)
	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	t.Setenv("OTEL_PROPAGATORS", "tracecontext,baggage")
	routes := fmt.Sprintf("tracing\nphp /index.php {\nroot %s\n}", root)
	old, oldServer := reloadSecurityLoad(t, routes)
	next, newServer := reloadSecurityLoad(t, routes)
	oldKey, _ := old.modules[0].reloadKey()
	newKey, _ := next.modules[0].reloadKey()
	require.NotEmpty(t, oldKey)
	require.NotEmpty(t, newKey)
	require.NotEqual(t, oldKey, newKey, "unchanged tracing in the default directive order must refuse handoff")
	request := func() *http.Request { return httptest.NewRequest(http.MethodGet, "http://example.com/index.php", nil) }
	direct := reloadSecurityServe(newServer, request())
	require.Equal(t, http.StatusOK, direct.Code)
	require.Equal(t, "old:/index.php:", direct.Body.String())
	retired := reloadSecurityServe(oldServer, request())
	require.Equal(t, http.StatusServiceUnavailable, retired.Code)
	require.Empty(t, retired.Body.String())
}

type reloadValueRequestBodyHandler struct {
	Set string `json:"set,omitempty"`
}

func (reloadValueRequestBodyHandler) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{ID: "http.handlers.request_body"}
}

func (reloadValueRequestBodyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	return next.ServeHTTP(w, r)
}

func TestReloadKeyValueRequestBodyFailsClosed(t *testing.T) {
	key := func(h caddyhttp.MiddlewareHandler) string {
		app := new(FrankenPHPApp)
		module := &FrankenPHPModule{app: app}
		app.modules = []*FrankenPHPModule{module}
		app.httpApp = &caddyhttp.App{Servers: map[string]*caddyhttp.Server{"srv0": {Routes: caddyhttp.RouteList{{Handlers: []caddyhttp.MiddlewareHandler{h, module}}}}}}
		result, _ := module.reloadKey()
		require.NotEmpty(t, result)
		return result
	}
	for _, set := range []string{"", "allowed"} {
		t.Run("set="+set, func(t *testing.T) {
			h := reloadValueRequestBodyHandler{Set: set}
			require.NotEqual(t, key(h), key(h), "value-typed handlers must poison without calling reflect.Value.Elem")
		})
	}
	h := &reloadValueRequestBodyHandler{}
	require.Equal(t, key(h), key(h), "an empty setter on a pointer handler remains equivalent")
}

func reloadSecurityConfig(t *testing.T, routes string, definitions ...string) []byte {
	t.Helper()
	config := `{
    admin off
    persist_config off
    auto_https off
    frankenphp {
        num_threads 2
    }
}
` + strings.Join(definitions, "\n") + `
http://example.com:0 {
    bind 127.0.0.1
` + routes + "\n}"
	adapted, _, err := caddyconfig.GetAdapter("caddyfile").Adapt([]byte(config), nil)
	require.NoError(t, err)
	return adapted
}

func reloadSecurityLoad(t *testing.T, routes string, definitions ...string) (*FrankenPHPApp, *caddyhttp.Server) {
	t.Helper()
	require.NoError(t, caddy.Load(reloadSecurityConfig(t, routes, definitions...), true))
	app := activeApp.Load()
	require.NotNil(t, app)
	for _, server := range app.httpApp.Servers {
		return app, server
	}
	t.Fatal("no HTTP server")
	return nil, nil
}

func reloadSecurityRoots(t *testing.T) (string, string) {
	t.Helper()
	if testing.Short() {
		t.Skip("requires the PHP runtime")
	}
	root := t.TempDir()
	old, next := filepath.Join(root, "old"), filepath.Join(root, "new")
	for directory, generation := range map[string]string{old: "old", next: "new"} {
		require.NoError(t, os.Mkdir(directory, 0700))
		script := fmt.Sprintf(`<?php echo '%s:' . $_SERVER['REQUEST_URI'] . ':' . file_get_contents('php://input');`, generation)
		require.NoError(t, os.WriteFile(filepath.Join(directory, "index.php"), []byte(script), 0600))
	}
	t.Cleanup(func() {
		require.NoError(t, caddy.Stop())
		frankenphp.Shutdown()
		activeApp.Store(nil)
	})
	return old, next
}

func reloadSecurityServe(server *caddyhttp.Server, request *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	server.ServeHTTP(w, request)
	return w
}

func TestReloadHandoffEnforcesAddedBasicAuth(t *testing.T) {
	for _, handler := range []string{"php", "php_server"} {
		t.Run(handler, func(t *testing.T) {
			oldRoot, newRoot := reloadSecurityRoots(t)
			routes := func(root, auth string) string {
				return fmt.Sprintf("route /index.php {\n%s%s {\nroot %s\n}\n}", auth, handler, root)
			}
			old, oldServer := reloadSecurityLoad(t, routes(oldRoot, ""))
			hash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
			require.NoError(t, err)
			auth := fmt.Sprintf("basic_auth {\nalice %s\n}\n", hash)
			next, newServer := reloadSecurityLoad(t, routes(newRoot, auth))
			oldKey, _ := old.modules[0].reloadKey()
			newKey, _ := next.modules[0].reloadKey()
			require.NotEmpty(t, oldKey)
			require.Equal(t, oldKey, newKey, "the equivalent scope must retain its handoff")
			for i, server := range []*caddyhttp.Server{newServer, oldServer} {
				w := reloadSecurityServe(server, httptest.NewRequest(http.MethodGet, "http://example.com/index.php", nil))
				t.Logf("chain %d returned %d, body %q", i, w.Code, w.Body.String())
				expected := http.StatusUnauthorized
				if i == 1 {
					expected = http.StatusServiceUnavailable
				}
				require.Equal(t, expected, w.Code)
				require.NotContains(t, w.Body.String(), "new:")
				request := httptest.NewRequest(http.MethodPost, "http://example.com/index.php?query=kept", strings.NewReader("preserved body"))
				request.SetBasicAuth("alice", "password")
				w = reloadSecurityServe(server, request)
				require.Equal(t, http.StatusOK, w.Code)
				require.Equal(t, "new:/index.php?query=kept:preserved body", w.Body.String())
			}
		})
	}
}

func TestReloadHandoffUsesReplacementRoot(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(fmt.Sprintf("explicit=%t", explicit), func(t *testing.T) {
			oldRoot, newRoot := reloadSecurityRoots(t)
			routes := func(root string) string {
				if explicit {
					return fmt.Sprintf("php_server {\nroot %s\n}", root)
				}
				return fmt.Sprintf("root * %s\nphp_server", root)
			}
			_, oldServer := reloadSecurityLoad(t, routes(oldRoot))
			_, newServer := reloadSecurityLoad(t, routes(newRoot))
			for name, server := range map[string]*caddyhttp.Server{"new": newServer, "retired": oldServer} {
				w := reloadSecurityServe(server, httptest.NewRequest(http.MethodGet, "http://example.com/release?version=2", nil))
				t.Logf("%s chain returned %d, body %q", name, w.Code, w.Body.String())
				require.Equal(t, http.StatusOK, w.Code)
				require.Equal(t, "new:/release?version=2:", w.Body.String())
			}
			require.NoError(t, os.RemoveAll(oldRoot))
			w := reloadSecurityServe(oldServer, httptest.NewRequest(http.MethodGet, "http://example.com/index.php", nil))
			require.Equal(t, http.StatusOK, w.Code)
			require.Equal(t, "new:/index.php:", w.Body.String())
		})
	}
}

func TestReloadHandoffRechecksChangedBasicAuth(t *testing.T) {
	oldRoot, newRoot := reloadSecurityRoots(t)
	routes := func(root, password string) string {
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
		require.NoError(t, err)
		return fmt.Sprintf("route /protected {\nbasic_auth {\nalice %s\n}\nrewrite * /index.php\nphp {\nroot %s\n}\n}", hash, root)
	}
	_, oldServer := reloadSecurityLoad(t, routes(oldRoot, "old-password"))
	_, newServer := reloadSecurityLoad(t, routes(newRoot, "new-password"))
	for i, server := range []*caddyhttp.Server{newServer, oldServer} {
		request := httptest.NewRequest(http.MethodPost, "http://example.com/protected?query=kept", strings.NewReader("preserved body"))
		request.SetBasicAuth("alice", "old-password")
		w := reloadSecurityServe(server, request)
		expected := http.StatusUnauthorized
		if i == 1 {
			expected = http.StatusServiceUnavailable
		}
		require.Equal(t, expected, w.Code, "old credentials must not authorize new PHP")
		require.Empty(t, w.Body.String())
	}
	// The retired chain must first accept the old credentials. Keeping both
	// accounts in the next policy proves the replay retains credentials and body.
	oldHash, err := bcrypt.GenerateFromPassword([]byte("old-password"), bcrypt.MinCost)
	require.NoError(t, err)
	newHash, err := bcrypt.GenerateFromPassword([]byte("new-password"), bcrypt.MinCost)
	require.NoError(t, err)
	_, _ = reloadSecurityLoad(t, fmt.Sprintf("route /protected {\nbasic_auth {\nalice %s\nbob %s\n}\nrewrite * /index.php\nphp {\nroot %s\n}\n}", oldHash, newHash, newRoot))
	request := httptest.NewRequest(http.MethodPost, "http://example.com/protected?query=kept", strings.NewReader("preserved body"))
	request.SetBasicAuth("alice", "old-password")
	w := reloadSecurityServe(oldServer, request)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "new:/protected?query=kept:preserved body", w.Body.String())
}

func TestReloadHandoffRefreshesVarsAndAuthPlaceholders(t *testing.T) {
	oldRoot, newRoot := reloadSecurityRoots(t)
	script := `<?php echo $_SERVER['GENERATION'] . ':' . $_SERVER['USER'] . ':' . $_SERVER['REQUEST_URI'] . ':' . file_get_contents('php://input');`
	for _, root := range []string{oldRoot, newRoot} {
		require.NoError(t, os.WriteFile(filepath.Join(root, "index.php"), []byte(script), 0600))
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	require.NoError(t, err)
	routes := func(root, generation, auth string) string {
		return fmt.Sprintf("route /index.php {\nvars generation %s\n%sphp {\nroot %s\nenv GENERATION {http.vars.generation}\nenv USER {http.auth.user.id}\n}\n}", generation, auth, root)
	}
	_, oldServer := reloadSecurityLoad(t, routes(oldRoot, "old", fmt.Sprintf("basic_auth {\nalice %s\n}\n", hash)))
	initial := httptest.NewRequest(http.MethodGet, "http://example.com/index.php", nil)
	initial.SetBasicAuth("alice", "password")
	require.Equal(t, "old:alice:/index.php:", reloadSecurityServe(oldServer, initial).Body.String())
	_, newServer := reloadSecurityLoad(t, routes(newRoot, "new", ""))
	for _, server := range []*caddyhttp.Server{newServer, oldServer} {
		request := httptest.NewRequest(http.MethodPost, "http://example.com/index.php?query=kept", strings.NewReader("preserved body"))
		request.SetBasicAuth("alice", "password")
		w := reloadSecurityServe(server, request)
		require.Equal(t, http.StatusOK, w.Code)
		// ReplaceKnown preserves unknown placeholders after auth is removed.
		require.Equal(t, "new:{http.auth.user.id}:/index.php?query=kept:preserved body", w.Body.String(), "old variables and auth identity must not reach replacement PHP")
	}
}

func TestReloadHandoffRejectsStaleAuthorizationHeader(t *testing.T) {
	oldRoot, newRoot := reloadSecurityRoots(t)
	_, oldServer := reloadSecurityLoad(t, fmt.Sprintf("route /index.php {\nrequest_header Authorization \"Basic YWxpY2U6cGFzc3dvcmQ=\"\nphp {\nroot %s\n}\n}", oldRoot))
	hash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	require.NoError(t, err)
	_, newServer := reloadSecurityLoad(t, fmt.Sprintf("route /index.php {\nbasic_auth {\nalice %s\n}\nphp {\nroot %s\n}\n}", hash, newRoot))
	request := func() *http.Request { return httptest.NewRequest(http.MethodGet, "http://example.com/index.php", nil) }
	require.Equal(t, http.StatusUnauthorized, reloadSecurityServe(newServer, request()).Code)
	w := reloadSecurityServe(oldServer, request())
	require.Equal(t, http.StatusServiceUnavailable, w.Code, "an old header mutation cannot authorize the new policy")
	require.Empty(t, w.Body.String())
}

func TestReloadHandoffNamedRouteUsesReplacementCaller(t *testing.T) {
	for _, nested := range []bool{false, true} {
		for _, staleHeader := range []bool{false, true} {
			t.Run(fmt.Sprintf("nested=%t/stale_header=%t", nested, staleHeader), func(t *testing.T) {
				oldRoot, newRoot := reloadSecurityRoots(t)
				definition := func(root string) string { return fmt.Sprintf("&(runtime) {\nphp /index.php {\nroot %s\n}\n}", root) }
				prefix := ""
				if staleHeader {
					prefix = "request_header Authorization \"Basic YWxpY2U6cGFzc3dvcmQ=\"\n"
				}
				caller, oldDefinition, newDefinition := "runtime", definition(oldRoot), definition(newRoot)
				if nested {
					caller = "entry"
					// Keep both handlers: the named-route adapter can otherwise
					// collapse a merged route to its first handler.
					oldDefinition += "\n&(entry) {\nroute {\n" + prefix + "invoke runtime\n}\n}"
					newDefinition += "\n&(entry) {\nroute {\ninvoke runtime\n}\n}"
					prefix = ""
				}
				_, oldServer := reloadSecurityLoad(t, "route /index.php {\n"+prefix+"invoke "+caller+"\n}", oldDefinition)
				initial := reloadSecurityServe(oldServer, httptest.NewRequest(http.MethodGet, "http://example.com/index.php", nil))
				require.Equal(t, "old:/index.php:", initial.Body.String(), "the old fixture must reach PHP before testing handoff")
				if !staleHeader {
					// Equivalent named scopes keep their handoff and request body.
					_, _ = reloadSecurityLoad(t, "route /index.php {\ninvoke "+caller+"\n}", newDefinition)
					w := reloadSecurityServe(oldServer, httptest.NewRequest(http.MethodPost, "http://example.com/index.php?query=kept", strings.NewReader("preserved body")))
					require.Equal(t, http.StatusOK, w.Code)
					require.Equal(t, "new:/index.php?query=kept:preserved body", w.Body.String())
				}
				hash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
				require.NoError(t, err)
				_, newServer := reloadSecurityLoad(t, fmt.Sprintf("route /index.php {\nbasic_auth {\nalice %s\n}\ninvoke %s\n}", hash, caller), newDefinition)
				request := func() *http.Request { return httptest.NewRequest(http.MethodGet, "http://example.com/index.php", nil) }
				require.Equal(t, http.StatusUnauthorized, reloadSecurityServe(newServer, request()).Code)
				w := reloadSecurityServe(oldServer, request())
				t.Logf("retired named-route chain returned %d, body %q", w.Code, w.Body.String())
				require.Equal(t, http.StatusServiceUnavailable, w.Code)
				require.Empty(t, w.Body.String())
			})
		}
	}
}

// Caddy starts apps in map order. Ensure PHP started before failing, including
// when this app happens to be visited first, so the rejected config cancels an
// app that hasStarted and has retired the original PHP servers.
type reloadSecurityFailingApp struct{ app *FrankenPHPApp }

func (*reloadSecurityFailingApp) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "reload_security_failure",
		New: func() caddy.Module { return new(reloadSecurityFailingApp) },
	}
}

func (app *reloadSecurityFailingApp) Provision(ctx caddy.Context) error {
	f, err := ctx.App("frankenphp")
	if err == nil {
		app.app = f.(*FrankenPHPApp)
	}
	return err
}

func (app *reloadSecurityFailingApp) Start() error {
	if !app.app.hasStarted.Load() {
		if err := app.app.Start(); err != nil {
			return err
		}
	}
	return errors.New("deliberate startup failure after PHP started")
}

func (*reloadSecurityFailingApp) Stop() error { return nil }

func init() { caddy.RegisterModule(new(reloadSecurityFailingApp)) }

func TestReloadHandoffRejectsCanceledReplacement(t *testing.T) {
	oldRoot, newRoot := reloadSecurityRoots(t)
	routes := func(root string) string { return fmt.Sprintf("php /index.php {\nroot %s\n}", root) }
	old, oldServer := reloadSecurityLoad(t, routes(oldRoot))
	var config map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(reloadSecurityConfig(t, routes(newRoot)), &config))
	var apps map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(config["apps"], &apps))
	apps["reload_security_failure"] = json.RawMessage(`{}`)
	var err error
	config["apps"], err = json.Marshal(apps)
	require.NoError(t, err)
	rejected, err := json.Marshal(config)
	require.NoError(t, err)
	require.ErrorContains(t, caddy.Load(rejected, true), "deliberate startup failure")
	next := old.next.Load()
	require.NotNil(t, next)
	require.True(t, next.hasStarted.Load())
	require.ErrorIs(t, next.ctx.Err(), context.Canceled)
	require.NoError(t, old.ctx.Err(), "Caddy keeps the original config after rollback")
	w := reloadSecurityServe(oldServer, httptest.NewRequest(http.MethodGet, "http://example.com/index.php", nil))
	t.Logf("original chain after rollback returned %d, body %q", w.Code, w.Body.String())
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	require.NotContains(t, w.Body.String(), "new:")
}

// A graceful shutdown can legitimately take longer than the former ten-second
// startup timeout. Both accepted and newly routed requests must keep waiting.
func TestReloadWaitsPastTenSeconds(t *testing.T) {
	if testing.Short() {
		t.Skip("exercises a reload lasting longer than ten seconds")
	}
	root := t.TempDir()
	server, err := frankenphp.NewServer(root)
	require.NoError(t, err)
	old := &FrankenPHPApp{ctx: context.Background(), started: make(chan any)}
	close(old.started)
	old.hasStarted.Store(true)
	next := &FrankenPHPApp{ctx: context.Background(), started: make(chan any)}
	old.next.Store(next)
	oldModule := &FrankenPHPModule{app: old, server: server, resolvedDocumentRoot: root}
	newModule := &FrankenPHPModule{app: next, server: server, resolvedDocumentRoot: root}
	oldRequest := reloadTestRequest("/accepted/request", "")
	oldContext, cancelOld := context.WithCancel(oldRequest.Context())
	newRequest := reloadTestRequest("/new/request", "")
	newContext, cancelNew := context.WithCancel(newRequest.Context())
	oldResult, newResult := make(chan error, 1), make(chan error, 1)
	starting := make(chan struct{}, 2)
	var requests sync.WaitGroup
	var released sync.Once
	release := func() { released.Do(func() { close(next.started) }) }
	t.Cleanup(func() {
		cancelOld()
		cancelNew()
		release()
		requests.Wait()
	})
	requests.Add(2)
	go func() {
		defer requests.Done()
		starting <- struct{}{}
		oldResult <- oldModule.ServeHTTP(httptest.NewRecorder(), oldRequest.WithContext(oldContext), nil)
	}()
	go func() {
		defer requests.Done()
		starting <- struct{}{}
		newResult <- newModule.ServeHTTP(httptest.NewRecorder(), newRequest.WithContext(newContext), nil)
	}()
	<-starting
	<-starting
	timer := time.NewTimer(10*time.Second + 100*time.Millisecond)
	defer timer.Stop()
	select {
	case err := <-oldResult:
		t.Fatalf("accepted request returned before reload completed: %v", err)
	case err := <-newResult:
		t.Fatalf("new request returned before its app completed startup: %v", err)
	case <-timer.C:
	}
	// Client cancellation still releases an accepted request immediately.
	cancelOld()
	select {
	case err := <-oldResult:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("accepted request ignored client cancellation")
	}
	// A failed startup signals completion too: the waiting new request must
	// return an unavailable error instead of waiting indefinitely.
	release()
	select {
	case err := <-newResult:
		var handlerError caddyhttp.HandlerError
		require.ErrorAs(t, err, &handlerError)
		require.Equal(t, http.StatusServiceUnavailable, handlerError.StatusCode)
	case <-time.After(time.Second):
		t.Fatal("new request ignored failed-startup completion")
	}
}

type reloadObservedContext struct {
	context.Context
	waiting chan struct{}
	once    sync.Once
}

func (ctx *reloadObservedContext) Done() <-chan struct{} {
	ctx.once.Do(func() { close(ctx.waiting) })
	return ctx.Context.Done()
}

func TestReloadAbandonedStartupReleasesRequest(t *testing.T) {
	appContext, abandon := context.WithCancel(context.Background())
	app := &FrankenPHPApp{ctx: appContext, started: make(chan any)}
	module := &FrankenPHPModule{app: app}
	requestContext, cancelRequest := context.WithCancel(context.Background())
	observed := &reloadObservedContext{Context: requestContext, waiting: make(chan struct{})}
	request := httptest.NewRequest(http.MethodGet, "http://example.com/", nil).WithContext(observed)
	result := make(chan error, 1)
	var requests sync.WaitGroup
	requests.Add(1)
	t.Cleanup(func() {
		abandon()
		cancelRequest()
		requests.Wait()
	})
	go func() {
		defer requests.Done()
		result <- module.ServeHTTP(httptest.NewRecorder(), request, nil)
	}()
	select {
	case <-observed.waiting:
	case <-time.After(time.Second):
		t.Fatal("request never waited for startup")
	}
	// Caddy can cancel a provisioned app without ever calling Start().
	// Its completion channel remains open, but waiting traffic must return.
	abandon()
	select {
	case err := <-result:
		var handlerError caddyhttp.HandlerError
		require.ErrorAs(t, err, &handlerError)
		require.Equal(t, http.StatusServiceUnavailable, handlerError.StatusCode)
	case <-time.After(time.Second):
		t.Fatal("request ignored abandoned startup")
	}
	select {
	case <-app.started:
		t.Fatal("abandoned app unexpectedly completed startup")
	default:
	}
}

func TestReloadFailedStartupDoesNotDispatch(t *testing.T) {
	app := &FrankenPHPApp{ctx: context.Background(), started: make(chan any)}
	close(app.started)
	module := &FrankenPHPModule{app: app} // failed startup never assigned a server
	var err error
	require.NotPanics(t, func() {
		err = module.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil), nil)
	})
	var handlerError caddyhttp.HandlerError
	require.ErrorAs(t, err, &handlerError)
	require.Equal(t, http.StatusServiceUnavailable, handlerError.StatusCode)
}

func TestReloadAbandonedReplacementReleasesAcceptedRequest(t *testing.T) {
	appContext, abandon := context.WithCancel(context.Background())
	next := &FrankenPHPApp{ctx: appContext, started: make(chan any)}
	old := &FrankenPHPApp{ctx: context.Background(), started: make(chan any)}
	old.hasStarted.Store(true)
	old.next.Store(next)
	server, err := frankenphp.NewServer(t.TempDir())
	require.NoError(t, err)
	module := &FrankenPHPModule{app: old, server: server, resolvedDocumentRoot: "/unused"}
	request := reloadTestRequest("/accepted/request", "")
	requestContext, cancelRequest := context.WithCancel(request.Context())
	observed := &reloadObservedContext{Context: requestContext, waiting: make(chan struct{})}
	result := make(chan error, 1)
	var requests sync.WaitGroup
	requests.Add(1)
	t.Cleanup(func() {
		abandon()
		cancelRequest()
		requests.Wait()
	})
	go func() {
		defer requests.Done()
		result <- module.ServeHTTP(httptest.NewRecorder(), request.WithContext(observed), nil)
	}()
	select {
	case <-observed.waiting:
	case <-time.After(time.Second):
		t.Fatal("accepted request never waited for replacement")
	}
	abandon()
	select {
	case err := <-result:
		var handlerError caddyhttp.HandlerError
		require.ErrorAs(t, err, &handlerError)
		require.Equal(t, http.StatusServiceUnavailable, handlerError.StatusCode)
	case <-time.After(time.Second):
		t.Fatal("accepted request ignored replacement cancellation")
	}
}
