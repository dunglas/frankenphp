package caddy

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/caddyauth"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/fileserver"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/headers"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/intercept"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/rewrite"
	"github.com/dunglas/frankenphp"
	"github.com/dunglas/frankenphp/internal/fastabs"
)

// FrankenPHPModule represents the "php_server" and "php" directives in the Caddyfile
// they are responsible for forwarding requests to FrankenPHP via "ServeHTTP"
// keep in mind that the module reference gets lost between Caddy parsing and Caddy provisioning
//
//	example.com {
//		php_server {
//			root /var/www/html
//		}
//	}
type FrankenPHPModule struct {
	mercureContext
	hotReloadContext
	GeneratedRewrite bool `json:"generated_rewrite,omitempty"` // Marks the php_server try_files pipeline.

	// Root sets the root folder to the site. Default: `root` directive, or the path of the public directory of the embed app it exists.
	Root string `json:"root,omitempty"`
	// SplitPath sets the substrings for splitting the URI into two parts. The first matching substring will be used to split the "path info" from the path. The first piece is suffixed with the matching substring and will be assumed as the actual resource (CGI script) name. The second piece will be set to PATH_INFO for the CGI script to use. Default: `.php`.
	SplitPath []string `json:"split_path,omitempty"`
	// ResolveRootSymlink enables resolving the `root` directory to its actual value by evaluating a symbolic link, if one exists.
	ResolveRootSymlink *bool `json:"resolve_root_symlink,omitempty"`
	// Env sets an extra environment variable to the given value. Can be specified more than once for multiple environment variables.
	Env map[string]string `json:"env,omitempty"`
	// Workers configures the worker scripts to start.
	Workers []workerConfig `json:"workers,omitempty"`
	// ServerIndex is set automatically to pair the route embeds of one php_server directive. Do not set it manually: modules sharing an index share one server, defined by the first of them.
	ServerIndex int `json:"server_index,omitempty"`
	// RequestBodyTimeout is an idle timeout on request body reads: a stalled (slow POST) client is cut off while a steady upload of any size succeeds. Defaults to 60s when omitted; set to 0 to disable.
	RequestBodyTimeout *caddy.Duration `json:"request_body_timeout,omitempty"`
	// Name is the name of the php_server this module belongs to for logging purposes
	Name string `json:"name,omitempty"`

	resolvedDocumentRoot string
	resolvedEnv          map[string]string
	requestEnv           frankenphp.PreparedEnv
	requestOptions       []frankenphp.RequestOption
	server               *frankenphp.Server
	logger               *slog.Logger
	app                  *FrankenPHPApp
}

// CaddyModule returns the Caddy module information.
func (*FrankenPHPModule) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.php",
		New: func() caddy.Module { return &FrankenPHPModule{} },
	}
}

// Provision sets up the module.
func (f *FrankenPHPModule) Provision(ctx caddy.Context) error {
	f.logger = ctx.Slogger()

	app, err := ctx.App("frankenphp")
	if err != nil {
		return err
	}
	fapp, ok := app.(*FrankenPHPApp)
	if !ok {
		return fmt.Errorf(`expected ctx.App("frankenphp") to return *FrankenPHPApp, got %T`, app)
	}
	if fapp == nil {
		return fmt.Errorf(`expected ctx.App("frankenphp") to return *FrankenPHPApp, got nil`)
	}

	f.app = fapp
	f.assignMercureHub(ctx)

	for i, wc := range f.Workers {
		// make the file path absolute from the public directory
		// this can only be done if the root is defined inside php_server
		if !filepath.IsAbs(wc.FileName) && f.Root != "" {
			wc.FileName = filepath.Join(f.Root, wc.FileName)
		}

		if frankenphp.EmbeddedAppPath != "" && filepath.IsLocal(wc.FileName) {
			wc.FileName = filepath.Join(frankenphp.EmbeddedAppPath, wc.FileName)
		}

		f.Workers[i] = wc
	}

	if f.Root == "" {
		if frankenphp.EmbeddedAppPath == "" {
			f.Root = "{http.vars.root}"
		} else {
			f.Root = filepath.Join(frankenphp.EmbeddedAppPath, defaultDocumentRoot)

			f.ResolveRootSymlink = new(false)
		}
	} else if frankenphp.EmbeddedAppPath != "" && filepath.IsLocal(f.Root) {
		f.Root = filepath.Join(frankenphp.EmbeddedAppPath, f.Root)
	}

	opt, err := frankenphp.WithRequestSplitPath(f.SplitPath)
	if err != nil {
		return fmt.Errorf("invalid split_path: %w", err)
	}
	f.requestOptions = append(f.requestOptions, opt)

	if f.RequestBodyTimeout == nil {
		f.RequestBodyTimeout = new(defaultRequestBodyTimeout)
	}

	if *f.RequestBodyTimeout > 0 {
		f.requestOptions = append(f.requestOptions, frankenphp.WithRequestBodyTimeout(time.Duration(*f.RequestBodyTimeout)))
	}

	if f.ResolveRootSymlink == nil {
		f.ResolveRootSymlink = new(true)
	}

	if !needReplacement(f.Root) {
		root, err := fastabs.FastAbs(f.Root)
		if err != nil {
			return fmt.Errorf("unable to make the root path absolute: %w", err)
		}
		f.resolvedDocumentRoot = root

		if *f.ResolveRootSymlink {
			root, err := filepath.EvalSymlinks(root)
			if err != nil {
				return fmt.Errorf("unable to resolve root symlink: %w", err)
			}

			f.resolvedDocumentRoot = root

			// Resolve symlinks in worker file paths
			for i, wc := range f.Workers {
				if filepath.IsAbs(wc.FileName) {
					resolvedPath, _ := filepath.EvalSymlinks(wc.FileName)
					f.Workers[i].FileName = resolvedPath
				}
			}
		}
	}

	if err := f.configureHotReload(fapp); err != nil {
		return err
	}

	f.resolvedEnv = make(map[string]string, len(f.Env)) // env variables that do not need replacement
	f.requestEnv = make(map[string]string, len(f.Env))  // env variables that need replacement, e.g. {http.vars.root}

	for k, e := range f.Env {
		if needReplacement(e) {
			f.requestEnv[k+"\x00"] = e // prepare the env with a null byte
		} else {
			f.resolvedEnv[k] = e
		}
	}

	fapp.modules = append(fapp.modules, f)

	return nil
}

// needReplacement checks if a string contains placeholders.
func needReplacement(s string) bool {
	return strings.ContainsAny(s, "{}")
}

// ServeHTTP implements caddyhttp.MiddlewareHandler.
func (f *FrankenPHPModule) ServeHTTP(w http.ResponseWriter, r *http.Request, _ caddyhttp.Handler) error {
	if !f.app.hasStarted.Load() {
		select {
		case <-f.app.started:
		case <-f.app.ctx.Done():
		case <-r.Context().Done():
			return r.Context().Err()
		}
		if !f.app.hasStarted.Load() || f.app.ctx.Err() != nil {
			return caddyhttp.Error(http.StatusServiceUnavailable, frankenphp.ErrNotRunning)
		}
	}

	ctx := r.Context()
	repl := ctx.Value(caddy.ReplacerCtxKey).(*caddy.Replacer)

	opts := make([]frankenphp.RequestOption, 0, len(f.requestOptions)+4)
	opts = append(opts, f.requestOptions...)
	opts = append(opts, frankenphp.WithOriginalRequest(new(ctx.Value(caddyhttp.OriginalRequestCtxKey).(http.Request))))

	// if the root contains a caddy placeholder, it needs to be resolved here in the hot path
	if f.resolvedDocumentRoot == "" {
		documentRoot := repl.ReplaceKnown(f.Root, "")
		if documentRoot == "" && frankenphp.EmbeddedAppPath != "" {
			documentRoot = frankenphp.EmbeddedAppPath
		}

		// If we do not have a resolved document root, then we cannot resolve the symlink of our cwd because it may
		// resolve to a different directory than the one we are currently in.
		// This is especially important if there are workers running.
		opts = append(opts, frankenphp.WithRequestDocumentRoot(documentRoot, false))
	}

	// all env variables that contain caddy placeholders need to be resolved here in the hot path
	if len(f.requestEnv) > 0 {
		env := make(frankenphp.PreparedEnv, len(f.requestEnv))
		for k, v := range f.requestEnv {
			env[k] = repl.ReplaceKnown(v, "")
		}

		opts = append(opts, frankenphp.WithRequestPreparedEnv(env))
	}

	err := f.server.ServeHTTP(w, r, opts...)

	if err != nil && errors.Is(err, frankenphp.ErrNotRunning) {
		return f.requestReload(w, r, f.app.next.Load())
	}
	if _, rejected := errors.AsType[frankenphp.ErrRejected](err); err != nil && !rejected {
		return caddyhttp.Error(http.StatusInternalServerError, err)
	}

	return nil
}

func (f *FrankenPHPModule) requestReload(w http.ResponseWriter, r *http.Request, app *FrankenPHPApp) error {
	unavailable := caddyhttp.Error(http.StatusServiceUnavailable, frankenphp.ErrNotRunning)
	for ; app != nil && app != f.app; app = app.next.Load() {
		select {
		case <-app.started:
		case <-app.ctx.Done():
		case <-r.Context().Done():
			return r.Context().Err()
		}
		if app.next.Load() == nil {
			key, _ := f.reloadKey()
			for _, target := range app.modules {
				if other, srv := target.reloadKey(); key != "" && other == key {
					for wrapper, ok := w.(interface {
						Unwrap() http.ResponseWriter
					}); ok; wrapper, ok = w.(interface {
						Unwrap() http.ResponseWriter
					}) {
						w = wrapper.Unwrap()
					}
					r, original, rec :=
						r.Clone(r.Context()),
						r.Context().Value(caddyhttp.OriginalRequestCtxKey).(http.Request),
						caddyhttp.NewResponseRecorder(w, nil, nil)
					r.Method, r.RequestURI, r.RemoteAddr, r.URL =
						original.Method, original.RequestURI,
						original.RemoteAddr, new(*original.URL)
					replay, r :=
						srv.Routes.Compile(caddyhttp.HandlerFunc(
							func(http.ResponseWriter, *http.Request) error {
								return unavailable
							},
						)),
						caddyhttp.PrepareRequest(r, caddy.NewReplacer(), rec, srv)
					if (!app.hasStarted.Load() ||
						app.ctx.Err() != nil ||
						replay.ServeHTTP(rec, r) != nil) && rec.Status() < 200 {
						w.WriteHeader(http.StatusServiceUnavailable)
					}
					return nil
				}
			}
		}
	}
	return unavailable
}

func (f *FrankenPHPModule) reloadKey() (string, *caddyhttp.Server) {
	if httpApp := f.app.httpApp; httpApp != nil {
		for _, srv := range httpApp.Servers {
			lists, callers, v :=
				map[string]caddyhttp.RouteList{"routes": srv.Routes},
				[]string{},
				reflect.ValueOf(srv).Elem().FieldByName("trustedProxies")
			if srv.Errors != nil {
				lists["errors"+reloadRouteKeys(
					srv.Routes, "", "", make(map[*FrankenPHPModule][2]string), false, f.app,
				)] = srv.Errors.Routes
			}
			for name, route := range srv.NamedRoutes {
				lists["named:"+name+reloadRouteKeys(
					srv.Routes, "", "", make(map[*FrankenPHPModule][2]string), false, f.app,
				)], callers = caddyhttp.RouteList{*route}, append(callers, reloadRouteKeys(
					caddyhttp.RouteList{*route}, "", "",
					make(map[*FrankenPHPModule][2]string), false, f.app,
				))
			}
			for name, routes := range lists {
				keys := make(map[*FrankenPHPModule][2]string)
				reloadRouteKeys(routes, "", "", keys, true, f.app)
				if pair, found := keys[f]; found {
					if !v.IsValid() || slices.ContainsFunc(
						f.app.modules,
						func(module *FrankenPHPModule) bool {
							return module != f && keys[module] == pair
						},
					) {
						return "", nil // Empty keys also fail closed on Caddy field renames.
					}
					return string(caddyconfig.JSON([]any{
						slices.Sorted(slices.Values(srv.Listen)),
						name,
						pair,
						slices.Sorted(slices.Values(callers)),
						srv.ExpectedUnderscoreHeaders,
						srv.ExpectedDotHeaders,
						srv.StrictSNIHost,
						srv.TrustedProxiesRaw,
						fmt.Sprintf("%#v", v.Elem()),
						srv.ClientIPHeaders,
						srv.TrustedProxiesStrict,
						srv.TrustedProxiesUnix,
					}, nil)), srv
				}
			}
		}
	}
	return "", nil
}

func reloadRouteKeys(
	routes caddyhttp.RouteList,
	scope, before string,
	keys map[*FrankenPHPModule][2]string,
	allowConsume bool,
	app *FrankenPHPApp,
) (prefix string) {
	cut := ^uint(0) // No pending consumable rewrite.
	for i, route := range routes {
		b, _ := json.Marshal(reloadMatchers(route.MatcherSets))
		var owner *FrankenPHPModule
		if i+1 < len(routes) && len(routes[i+1].Handlers) == 1 {
			owner, _ = routes[i+1].Handlers[0].(*FrankenPHPModule)
		}
		for _, h := range route.Handlers {
			config, err := json.Marshal(h)
			signature := string(b) + string(config)
			rw, isRewrite := h.(*rewrite.Rewrite)
			if owner != nil && (isRewrite && owner.GeneratedRewrite &&
				rw.URI == "{http.matchers.file.relative}{http.matchers.file.remainder}" ||
				owner.ServerIndex != 0 && slices.ContainsFunc(owner.Workers, func(w workerConfig) bool {
					return len(w.MatchPath) != 0
				}) && caddy.GetModuleName(h) == "file_server") {
				signature = strings.ReplaceAll(
					signature,
					`"root":`+string(caddyconfig.JSON(owner.Root, nil)),
					`"root":"{php.root}"`,
				)
			}
			if isRewrite {
				if allowConsume && len(route.Handlers) == 1 && owner != nil &&
					routes[i+1].Group == "" && strings.HasPrefix(rw.URI, "/") &&
					!strings.ContainsAny(rw.URI, "{}?#%\\*[]\x00\r\n\t") &&
					caddyhttp.CleanPath(rw.URI, !strings.Contains(rw.URI, "//")) == rw.URI &&
					string(config) == `{"uri":`+string(caddyconfig.JSON(rw.URI, nil))+`}` &&
					(len(routes[i+1].MatcherSets) == 0 ||
						string(caddyconfig.JSON(
							reloadMatchers(routes[i+1].MatcherSets), nil,
						)) == `[{"path":[`+string(caddyconfig.JSON(
							strings.ToLower(rw.URI), nil,
						))+`]}]`) {
					cut = uint(len(prefix))
				}
				prefix += signature
			} else if module, ok := h.(*FrankenPHPModule); ok {
				keys[module] = [2]string{scope + prefix + string(b), before + prefix + string(b)}
				prefix, cut = prefix[:min(cut, uint(len(prefix)))], ^uint(0)
			} else if fs, ok := h.(*fileserver.FileServer); ok && allowConsume &&
				len(route.MatcherSets) == 0 && route.Group == "" && !fs.PassThru {
				prefix = ""
			} else if sub, ok := h.(*caddyhttp.Subroute); ok {
				if key := reloadRouteKeys(
					sub.Routes,
					scope+string(b),
					before+prefix+string(b),
					keys,
					allowConsume && sub.Errors == nil,
					app,
				); key != "" {
					prefix += string(b) + key
				}
				if sub.Errors != nil {
					reloadRouteKeys(
						sub.Errors.Routes,
						scope+string(b)+"errors",
						before+prefix+string(b)+"errors",
						keys,
						false,
						app,
					)
				}
			} else if auth, _ := h.(*caddyauth.Authentication); !slices.Contains(
				[]string{"vars", "static_response", "invoke"}, caddy.GetModuleName(h),
			) && (auth == nil || slices.ContainsFunc(
				slices.Collect(maps.Keys(auth.Providers)),
				func(name string) bool {
					return name != "http_basic"
				},
			)) {
				hdr, _ := h.(*headers.Handler)
				inter, _ := h.(*intercept.Intercept)
				if proxy, ok := h.(*reverseproxy.Handler); err != nil ||
					ok && len(proxy.HandleResponse) > 0 ||
					auth != nil ||
					inter != nil && slices.ContainsFunc(
						inter.HandleResponse,
						func(rh caddyhttp.ResponseHandler) bool {
							return rh.Routes != nil
						},
					) ||
					hdr != nil && hdr.Request != nil ||
					caddy.GetModuleName(h) == "tracing" ||
					reflect.ValueOf(h).Kind() != reflect.Pointer ||
					caddy.GetModuleName(h) == "request_body" &&
						reflect.ValueOf(h).Elem().FieldByName("Set").String() != "" {
					signature = fmt.Sprintf("%p:%T", app, h)
				}
				prefix += caddy.GetModuleName(h) + signature
			}
		}
	}
	return
}

func reloadMatchers(sets caddyhttp.MatcherSets) []map[string]any {
	config := make([]map[string]any, len(sets))
	for i, set := range sets {
		config[i] = make(map[string]any, len(set))
		for _, m := range set {
			switch m := m.(type) {
			case *caddyhttp.MatchNot:
				config[i]["not"] = reloadMatchers(m.MatcherSets)
			case *caddyhttp.MatchHost:
				config[i]["host"] = slices.Sorted(slices.Values(*m))
			case caddyhttp.MatchHost:
				config[i]["host"] = slices.Sorted(slices.Values(m))
			default:
				config[i][caddy.GetModuleName(m)] = m
			}
		}
	}
	return config
}

// UnmarshalCaddyfile implements caddyfile.Unmarshaler.
func (f *FrankenPHPModule) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		for d.NextBlock(0) {
			switch d.Val() {
			case "root":
				if !d.NextArg() {
					return d.ArgErr()
				}
				f.Root = d.Val()

			case "split":
				f.SplitPath = d.RemainingArgs()
				if len(f.SplitPath) == 0 {
					return d.ArgErr()
				}

			case "env":
				args := d.RemainingArgs()
				if len(args) != 2 {
					return d.ArgErr()
				}
				if f.Env == nil {
					f.Env = make(map[string]string)
				}
				f.Env[args[0]] = args[1]

			case "resolve_root_symlink":
				if !d.NextArg() {
					continue
				}
				v, err := strconv.ParseBool(d.Val())
				if err != nil {
					return err
				}
				if d.NextArg() {
					return d.ArgErr()
				}
				f.ResolveRootSymlink = &v

			case "name":
				if !d.NextArg() {
					return d.ArgErr()
				}
				f.Name = d.Val()

			case "worker":
				wc, err := unmarshalWorker(d)
				if err != nil {
					return err
				}

				f.Workers = append(f.Workers, wc)

			case "hot_reload":
				if err := f.unmarshalHotReload(d); err != nil {
					return err
				}

			case "request_body_timeout":
				if !d.NextArg() {
					return d.ArgErr()
				}
				v, err := caddy.ParseDuration(d.Val())
				if err != nil {
					return err
				}
				if d.NextArg() {
					return d.ArgErr()
				}
				f.RequestBodyTimeout = new(caddy.Duration(v))

			default:
				return wrongSubDirectiveError("php or php_server", "hot_reload, name, root, split, env, resolve_root_symlink, request_body_timeout, worker", d.Val())
			}
		}
	}

	// Check if a worker with this filename already exists in this module
	fileNames := make(map[string]struct{}, len(f.Workers))
	for _, w := range f.Workers {
		if _, ok := fileNames[w.FileName]; ok {
			return fmt.Errorf(`workers in a single "php" or "php_server" block must not have duplicate filenames: %q`, w.FileName)
		}

		if len(w.MatchPath) == 0 {
			fileNames[w.FileName] = struct{}{}
		}
	}

	return nil
}

func (f *FrankenPHPModule) assignServerIndex(h httpcaddyfile.Helper) {
	counter, _ := h.State["php_server_count"].(int)
	f.ServerIndex = counter + 1
	h.State["php_server_count"] = counter + 1
}

// parseCaddyfile unmarshals tokens from h into a new Middleware.
func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	m := &FrankenPHPModule{}
	err := m.UnmarshalCaddyfile(h.Dispenser)

	m.assignServerIndex(h)

	return m, err
}

// parsePhpServer parses the php_server directive, which has a similar syntax
// to the php_fastcgi directive. A line such as this:
//
//	php_server
//
// is equivalent to a route consisting of:
//
//		# Add trailing slash for directory requests
//		@canonicalPath {
//		    file {path}/index.php
//		    not path */
//		}
//		redir @canonicalPath {path}/ 308
//
//		# If the requested file does not exist, try index files
//		@indexFiles file {
//		    try_files {path} {path}/index.php index.php
//		    split_path .php
//		}
//		rewrite @indexFiles {http.matchers.file.relative}
//
//		# FrankenPHP!
//		@phpFiles path *.php
//	 	php @phpFiles
//		file_server
//
// parsePhpServer is freely inspired from the php_fastgci directive of the Caddy server (Apache License 2.0, Matthew Holt and The Caddy Authors)
func parsePhpServer(h httpcaddyfile.Helper) ([]httpcaddyfile.ConfigValue, error) {
	if !h.Next() {
		return nil, h.ArgErr()
	}

	// set up FrankenPHP
	phpsrv := FrankenPHPModule{}

	// set up file server
	fsrv := fileserver.FileServer{}
	disableFsrv := false

	// set up the set of file extensions allowed to execute PHP code
	extensions := []string{".php"}

	// set the default index file for the try_files rewrites
	indexFile := "index.php"

	// set up for explicitly overriding try_files
	var tryFiles []string

	// if the user specified a matcher token, use that
	// matcher in a route that wraps both of our routes;
	// either way, strip the matcher token and pass
	// the remaining tokens to the unmarshaler so that
	// we can gain the rest of the directive syntax
	userMatcherSet, err := h.ExtractMatcherSet()
	if err != nil {
		return nil, err
	}

	// make a new dispenser from the remaining tokens so that we
	// can reset the dispenser back to this point for the
	// php unmarshaler to read from it as well
	dispenser := h.NewFromNextSegment()

	// read the subdirectives that we allow as overrides to
	// the php_server shortcut
	// NOTE: we delete the tokens as we go so that the php
	// unmarshal doesn't see these subdirectives which it cannot handle
	for dispenser.Next() {
		for dispenser.NextBlock(0) {
			// ignore any sub-subdirectives that might
			// have the same name somewhere within
			// the php passthrough tokens
			if dispenser.Nesting() != 1 {
				continue
			}

			// parse the php_server subdirectives
			switch dispenser.Val() {
			case "root":
				if !dispenser.NextArg() {
					return nil, dispenser.ArgErr()
				}
				phpsrv.Root = dispenser.Val()
				fsrv.Root = phpsrv.Root
				dispenser.DeleteN(2)

			case "split":
				extensions = dispenser.RemainingArgs()
				dispenser.DeleteN(len(extensions) + 1)
				if len(extensions) == 0 {
					return nil, dispenser.ArgErr()
				}

			case "index":
				args := dispenser.RemainingArgs()
				dispenser.DeleteN(len(args) + 1)
				if len(args) != 1 {
					return nil, dispenser.ArgErr()
				}
				indexFile = args[0]

			case "try_files":
				args := dispenser.RemainingArgs()
				dispenser.DeleteN(len(args) + 1)
				if len(args) < 1 {
					return nil, dispenser.ArgErr()
				}
				tryFiles = args

			case "file_server":
				args := dispenser.RemainingArgs()
				dispenser.DeleteN(len(args) + 1)
				if len(args) < 1 || args[0] != "off" {
					return nil, dispenser.ArgErr()
				}
				disableFsrv = true
			}
		}
	}

	// reset the dispenser after we're done so that the frankenphp
	// unmarshaler can read it from the start
	dispenser.Reset()

	// the rest of the config is specified by the user
	// using the php directive syntax
	dispenser.Next() // consume the directive name
	if err := phpsrv.UnmarshalCaddyfile(dispenser); err != nil {
		return nil, err
	}

	// assign a unique index to the php server
	phpsrv.assignServerIndex(h)

	if frankenphp.EmbeddedAppPath != "" {
		if phpsrv.Root == "" {
			phpsrv.Root = filepath.Join(frankenphp.EmbeddedAppPath, defaultDocumentRoot)
			fsrv.Root = phpsrv.Root
			phpsrv.ResolveRootSymlink = new(false)
		} else if filepath.IsLocal(fsrv.Root) {
			phpsrv.Root = filepath.Join(frankenphp.EmbeddedAppPath, phpsrv.Root)
			fsrv.Root = phpsrv.Root
		}
	}

	// set up a route list that we'll append to
	routes := caddyhttp.RouteList{}

	// set the list of allowed path segments on which to split
	phpsrv.SplitPath = extensions

	// prepend routes from the 'worker match *' directives
	routes = prependWorkerRoutes(routes, h, phpsrv, fsrv, disableFsrv)

	// if the index is turned off, we skip the redirect and try_files
	if indexFile != "off" {
		dirRedir := false
		dirIndex := "{http.request.uri.path}/" + indexFile
		// On Windows, first_exist_fallback doesn't work correctly because
		// glob is skipped and patterns are returned as-is without checking existence.
		// Use first_exist instead to ensure all files are checked.
		tryPolicy := "first_exist_fallback"
		if runtime.GOOS == "windows" {
			tryPolicy = "first_exist"
		}

		// if tryFiles wasn't overridden, use a reasonable default
		if len(tryFiles) == 0 {
			if disableFsrv {
				tryFiles = []string{dirIndex, indexFile}
			} else {
				tryFiles = []string{"{http.request.uri.path}", dirIndex, indexFile}
			}

			dirRedir = true
		} else {
			if !strings.HasSuffix(tryFiles[len(tryFiles)-1], ".php") {
				// use first_exist strategy if the last file is not a PHP file
				tryPolicy = ""
			}

			if slices.Contains(tryFiles, dirIndex) {
				dirRedir = true
			}
		}

		// route to redirect to canonical path if index PHP file
		if dirRedir {
			redirMatcherSet := caddy.ModuleMap{
				"file": h.JSON(fileserver.MatchFile{
					TryFiles: []string{dirIndex},
					Root:     phpsrv.Root,
				}),
				"not": h.JSON(caddyhttp.MatchNot{
					MatcherSetsRaw: []caddy.ModuleMap{
						{
							"path": h.JSON(caddyhttp.MatchPath{"*/"}),
						},
					},
				}),
			}
			redirHandler := caddyhttp.StaticResponse{
				StatusCode: caddyhttp.WeakString(strconv.Itoa(http.StatusPermanentRedirect)),
				Headers:    http.Header{"Location": []string{"{http.request.orig_uri.path}/"}},
			}
			redirRoute := caddyhttp.Route{
				MatcherSetsRaw: []caddy.ModuleMap{redirMatcherSet},
				HandlersRaw:    []json.RawMessage{caddyconfig.JSONModuleObject(redirHandler, "handler", "static_response", nil)},
			}

			routes = append(routes, redirRoute)
		}

		// route to rewrite to PHP index file
		rewriteMatcherSet := caddy.ModuleMap{
			"file": h.JSON(fileserver.MatchFile{
				TryFiles:  tryFiles,
				TryPolicy: tryPolicy,
				SplitPath: extensions,
				Root:      phpsrv.Root,
			}),
		}
		rewriteHandler := rewrite.Rewrite{
			URI: "{http.matchers.file.relative}{http.matchers.file.remainder}",
		}
		rewriteRoute := caddyhttp.Route{
			MatcherSetsRaw: []caddy.ModuleMap{rewriteMatcherSet},
			HandlersRaw:    []json.RawMessage{caddyconfig.JSONModuleObject(rewriteHandler, "handler", "rewrite", nil)},
		}

		phpsrv.GeneratedRewrite = true
		routes = append(routes, rewriteRoute)
	}

	// route to actually pass requests to PHP files;
	// match only requests that are for PHP files
	var pathList []string
	for _, ext := range extensions {
		pathList = append(pathList, "*"+ext, "*"+ext+"/*")
	}
	phpMatcherSet := caddy.ModuleMap{
		"path": h.JSON(pathList),
	}

	// create the PHP route which is
	// conditional on matching PHP files
	phpRoute := caddyhttp.Route{
		MatcherSetsRaw: []caddy.ModuleMap{phpMatcherSet},
		HandlersRaw:    []json.RawMessage{caddyconfig.JSONModuleObject(phpsrv, "handler", "php", nil)},
	}
	routes = append(routes, phpRoute)

	// create the file server route
	if !disableFsrv {
		fileRoute := caddyhttp.Route{
			MatcherSetsRaw: []caddy.ModuleMap{},
			HandlersRaw:    []json.RawMessage{caddyconfig.JSONModuleObject(fsrv, "handler", "file_server", nil)},
		}
		routes = append(routes, fileRoute)
	}

	subroute := caddyhttp.Subroute{
		Routes: routes,
	}

	// the user's matcher is a prerequisite for ours, so
	// wrap ours in a subroute and return that
	if userMatcherSet != nil {
		return []httpcaddyfile.ConfigValue{
			{
				Class: "route",
				Value: caddyhttp.Route{
					MatcherSetsRaw: []caddy.ModuleMap{userMatcherSet},
					HandlersRaw:    []json.RawMessage{caddyconfig.JSONModuleObject(subroute, "handler", "subroute", nil)},
				},
			},
		}, nil
	}

	// otherwise, return the literal subroute instead of
	// individual routes, to ensure they stay together and
	// are treated as a single unit, without necessarily
	// creating an actual subroute in the output
	return []httpcaddyfile.ConfigValue{
		{
			Class: "route",
			Value: subroute,
		},
	}, nil
}

// workers can also match a path without being in the public directory
// in this case we need to prepend the worker routes to the existing routes
func prependWorkerRoutes(routes caddyhttp.RouteList, h httpcaddyfile.Helper, f FrankenPHPModule, fsrv caddy.Module, disableFsrv bool) caddyhttp.RouteList {
	var allWorkerMatches caddyhttp.MatchPath
	for _, w := range f.Workers {
		for _, path := range w.MatchPath {
			allWorkerMatches = append(allWorkerMatches, path)
		}
	}

	if len(allWorkerMatches) == 0 {
		return routes
	}

	// if there are match patterns, we need to check for files beforehand
	if !disableFsrv {
		routes = append(routes, caddyhttp.Route{
			MatcherSetsRaw: []caddy.ModuleMap{
				{
					"file": h.JSON(fileserver.MatchFile{
						TryFiles: []string{"{http.request.uri.path}"},
						Root:     f.Root,
					}),
					"not": h.JSON(caddyhttp.MatchNot{
						MatcherSetsRaw: []caddy.ModuleMap{
							{"path": h.JSON(caddyhttp.MatchPath{"*.php"})},
						},
					}),
				},
			},
			HandlersRaw: []json.RawMessage{
				caddyconfig.JSONModuleObject(fsrv, "handler", "file_server", nil),
			},
		})
	}

	// forward matching routes to the PHP handler
	routes = append(routes, caddyhttp.Route{
		MatcherSetsRaw: []caddy.ModuleMap{
			{"path": h.JSON(allWorkerMatches)},
		},
		HandlersRaw: []json.RawMessage{
			caddyconfig.JSONModuleObject(f, "handler", "php", nil),
		},
	})

	return routes
}

// Interface guards
var (
	_ caddy.Provisioner           = (*FrankenPHPModule)(nil)
	_ caddyhttp.MiddlewareHandler = (*FrankenPHPModule)(nil)
	_ caddyfile.Unmarshaler       = (*FrankenPHPModule)(nil)
)
