// Package caddy provides a PHP module for the Caddy web server.
// FrankenPHP embeds the PHP interpreter directly in Caddy, giving it the ability to run your PHP scripts directly.
// No PHP FPM required!
package caddy

import (
	"net/http"
	"strconv"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/dunglas/frankenphp"
	"go.uber.org/zap"
)

func init() {
	caddy.RegisterModule(&FrankenPHPApp{})
	caddy.RegisterModule(&FrankenPHPModule{})
	httpcaddyfile.RegisterGlobalOption("frankenphp", parseGlobalOption)
	httpcaddyfile.RegisterHandlerDirective("php", parseCaddyfile)
	httpcaddyfile.RegisterDirective("frankenphp", parseFrankenPHP)
}

type mainPHPinterpreterKeyType int

var mainPHPInterpreterKey mainPHPinterpreterKeyType

var phpInterpreter = caddy.NewUsagePool()

type phpInterpreterDestructor struct{}

func (phpInterpreterDestructor) Destruct() error {
	frankenphp.Shutdown()

	return nil
}

type workerConfig struct {
	FileName string `json:"file_name,omitempty"`
	Num      int    `json:"num,omitempty"`
}

type FrankenPHPApp struct {
	NumThreads int            `json:"num_threads,omitempty"`
	Workers    []workerConfig `json:"workers,omitempty"`
}

// CaddyModule returns the Caddy module information.
func (a *FrankenPHPApp) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "frankenphp",
		New: func() caddy.Module { return a },
	}
}

func (f *FrankenPHPApp) Start() error {
	repl := caddy.NewReplacer()
	logger := caddy.Log()

	opts := []frankenphp.Option{frankenphp.WithNumThreads(f.NumThreads), frankenphp.WithLogger(logger)}
	for _, w := range f.Workers {
		opts = append(opts, frankenphp.WithWorkers(repl.ReplaceKnown(w.FileName, ""), w.Num))
	}

	_, loaded, err := phpInterpreter.LoadOrNew(mainPHPInterpreterKey, func() (caddy.Destructor, error) {
		if err := frankenphp.Init(opts...); err != nil {
			return nil, err
		}

		return phpInterpreterDestructor{}, nil
	})
	if err != nil {
		return err
	}

	if loaded {
		frankenphp.Shutdown()
		if err := frankenphp.Init(opts...); err != nil {
			return err
		}
	}

	logger.Info("FrankenPHP started 🐘")

	return nil
}

func (*FrankenPHPApp) Stop() error {
	caddy.Log().Info("FrankenPHP stopped 🐘")

	return nil
}

// UnmarshalCaddyfile implements caddyfile.Unmarshaler.
func (f *FrankenPHPApp) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		for d.NextBlock(0) {
			switch d.Val() {
			case "num_threads":
				if !d.NextArg() {
					return d.ArgErr()
				}

				v, err := strconv.Atoi(d.Val())
				if err != nil {
					return err
				}

				f.NumThreads = v

			case "worker":
				if !d.NextArg() {
					return d.ArgErr()
				}

				wc := workerConfig{FileName: d.Val()}
				if d.NextArg() {
					v, err := strconv.Atoi(d.Val())
					if err != nil {
						return err
					}

					wc.Num = v
				}

				f.Workers = append(f.Workers, wc)
			}
		}
	}

	return nil
}

func parseGlobalOption(d *caddyfile.Dispenser, _ interface{}) (interface{}, error) {
	app := &FrankenPHPApp{}
	if err := app.UnmarshalCaddyfile(d); err != nil {
		return nil, err
	}

	// tell Caddyfile adapter that this is the JSON for an app
	return httpcaddyfile.App{
		Name:  "frankenphp",
		Value: caddyconfig.JSON(app, nil),
	}, nil
}

type FrankenPHPModule struct {
	Root               string            `json:"root,omitempty"`
	SplitPath          []string          `json:"split_path,omitempty"`
	ResolveRootSymlink bool              `json:"resolve_root_symlink,omitempty"`
	Env                map[string]string `json:"env,omitempty"`
	Index              string            `json:"index,omitempty"`
	logger             *zap.Logger
	upstreams    []caddyhttp.Upstream
	WorkerConfig 	   workerConfig
}

// CaddyModule returns the Caddy module information.
func (FrankenPHPModule) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.php",
		New: func() caddy.Module { return new(FrankenPHPModule) },
	}
}

// Provision sets up the module.
func (f *FrankenPHPModule) Provision(ctx caddy.Context) error {
	f.logger = ctx.Logger(f)

	if f.Root == "" {
		f.Root = "{http.vars.root}"
	}
	if len(f.SplitPath) == 0 {
		f.SplitPath = []string{".php"}
	}

	return nil
}

// ServeHTTP implements caddyhttp.MiddlewareHandler.
// TODO: Expose TLS versions as env vars, as Apache's mod_ssl: https://github.com/caddyserver/caddy/blob/master/modules/caddyhttp/reverseproxy/fastcgi/fastcgi.go#L298
func (f FrankenPHPModule) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	origReq := r.Context().Value(caddyhttp.OriginalRequestCtxKey).(http.Request)
	repl := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)

	documentRoot := repl.ReplaceKnown(f.Root, "")
	fr := frankenphp.NewRequestWithContext(r, documentRoot, f.logger)
	fc, _ := frankenphp.FromContext(fr.Context())
	fc.ResolveRootSymlink = f.ResolveRootSymlink
	fc.SplitPath = f.SplitPath

	fc.Env["REQUEST_URI"] = origReq.URL.RequestURI()
	for k, v := range f.Env {
		fc.Env[k] = repl.ReplaceKnown(v, "")
	}

	return frankenphp.ServeHTTP(w, fr)
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
				if d.NextArg() {
					return d.ArgErr()
				}
				f.ResolveRootSymlink = true
			}
		}
	}

	return nil
}

// parseCaddyfile unmarshals tokens from h into a new Middleware.
func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	var m FrankenPHPModule
	err := m.UnmarshalCaddyfile(h.Dispenser)

	return m, err
}

func parseFrankenPHP(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	var err error
	var index, root string
	var resolveRootSymlink bool
	splitPath :=[]string{".php"}
	var env map[string]string
	frankenphp := FrankenPHPModule{}
	dispenser := h.NewFromNextSegment()

	for dispenser.Next() {
		for dispenser.NextBlock(0) {
			if dispenser.Nesting() != 1 {
				continue
			}

			switch dispenser.Val() {
			case "root":
				if !dispenser.NextArg() {
					return nil, dispenser.ArgErr()
				}
				frankenphp.Root = dispenser.Val()
				dispenser.Delete()
				dispenser.Delete()

			case "env":
				args := dispenser.RemainingArgs()
				dispenser.Delete()
				for range args {
					dispenser.Delete()
				}
				if len(args) != 2 {
					return nil, dispenser.ArgErr()
				}
				if frankenphp.Env == nil {
					frankenphp.Env = make(map[string]string)
				}
				frankenphp.Env[args[0]] = args[1]
			case "index":
				args := dispenser.RemainingArgs()
				dispenser.Delete()
				for range args {
					dispenser.Delete()
				}
				if len(args) != 1 {
					return nil, dispenser.ArgErr()
				}
				indexFile = args[0]
			case "resolve_root_symlink":
				args := dispenser.RemainingArgs()
				dispenser.Delete()
				for range args {
					dispenser.Delete()
				}
				frankenphp.ResolveRootSymlink = true
			}
		}
		frankenphp.SplitPath = splitPath
		dispenser.Reset()
		err = frankenphp.UnmarshalCaddyfile(dispenser) 
		if err != nil {
			return nil, err
		}
	return func(next caddyhttp.Handler) caddyhttp.Handler {
		return frankenphp
	}, err
}

// Interface guards
var (
	_ caddy.App                   = (*FrankenPHPApp)(nil)
	_ caddy.Provisioner           = (*FrankenPHPModule)(nil)
	_ caddyhttp.MiddlewareHandler = (*FrankenPHPModule)(nil)
	_ caddyfile.Unmarshaler       = (*FrankenPHPModule)(nil)
)
