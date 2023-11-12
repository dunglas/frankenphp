package frankenphp

import (
	"path/filepath"

	"go.uber.org/zap"
)

// RequestOption instances allow to configure a FrankenPHP Request.
type RequestOption func(h *FrankenPHPContext) error

// WithRequestDocumentRoot sets the root directory of the PHP application.
// if resolveSymlink is true, oath declared as root directory will be resolved
// to its absolute value after the evaluation of any symbolic links.
// Due to the nature of PHP opcache, root directory path is cached: when
// using a symlinked directory as root this could generate errors when
// symlink is changed without PHP being restarted; enabling this
// directive will set $_SERVER['DOCUMENT_ROOT'] to the real directory path.
func WithRequestDocumentRoot(documentRoot string, resolveSymlink bool) RequestOption {
	return func(o *FrankenPHPContext) error {
		// make sure file root is absolute
		root, err := filepath.Abs(documentRoot)
		if err != nil {
			return err
		}

		if resolveSymlink {
			if root, err = filepath.EvalSymlinks(root); err != nil {
				return err
			}
		}

		o.documentRoot = root

		return nil
	}
}

// The path in the URL will be split into two, with the first piece ending
// with the value of SplitPath. The first piece will be assumed as the
// actual resource (CGI script) name, and the second piece will be set to
// PATH_INFO for the CGI script to use.
//
// Future enhancements should be careful to avoid CVE-2019-11043,
// which can be mitigated with use of a try_files-like behavior
// that 404s if the fastcgi path info is not found.
func WithRequestSplitPath(splitPath []string) RequestOption {
	return func(o *FrankenPHPContext) error {
		o.splitPath = splitPath

		return nil
	}
}

// WithEnv set CGI-like environment variables that will be available in $_SERVER.
// Values set with WithEnv always have priority over automatically populated values.
func WithRequestEnv(env map[string]string) RequestOption {
	return func(o *FrankenPHPContext) error {
		o.env = env

		return nil
	}
}

// WithLogger sets the logger associated with the current request
func WithRequestLogger(logger *zap.Logger) RequestOption {
	return func(o *FrankenPHPContext) error {
		o.logger = logger

		return nil
	}
}
