package frankenphp_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/http/httptrace"
	"net/textproto"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"
)

type testOptions struct {
	workerScript        string
	nbWorkers           int
	env                 map[string]string
	nbParrallelRequests int
	realServer          bool
	logger              *zap.Logger
	initOpts            []frankenphp.Option
}

func runTest(t *testing.T, test func(func(http.ResponseWriter, *http.Request), *httptest.Server, int), opts *testOptions) {
	if opts == nil {
		opts = &testOptions{}
	}
	if opts.nbParrallelRequests == 0 {
		opts.nbParrallelRequests = 100
	}

	cwd, _ := os.Getwd()
	testDataDir := cwd + "/testdata/"

	if opts.logger == nil {
		opts.logger = zaptest.NewLogger(t)
	}

	initOpts := []frankenphp.Option{frankenphp.WithLogger(opts.logger)}
	if opts.workerScript != "" {
		initOpts = append(initOpts, frankenphp.WithWorkers(testDataDir+opts.workerScript, opts.nbWorkers, opts.env))
	}
	initOpts = append(initOpts, opts.initOpts...)

	err := frankenphp.Init(initOpts...)
	require.Nil(t, err)
	defer frankenphp.Shutdown()

	handler := func(w http.ResponseWriter, r *http.Request) {
		req, err := frankenphp.NewRequestWithContext(r, frankenphp.WithRequestDocumentRoot(testDataDir, false))
		assert.NoError(t, err)

		err = frankenphp.ServeHTTP(w, req)
		assert.NoError(t, err)
	}

	var ts *httptest.Server
	if opts.realServer {
		ts = httptest.NewServer(http.HandlerFunc(handler))
		defer ts.Close()
	}

	var wg sync.WaitGroup
	wg.Add(opts.nbParrallelRequests)
	for i := 0; i < opts.nbParrallelRequests; i++ {
		go func(i int) {
			test(handler, ts, i)
			wg.Done()
		}(i)
	}

	wg.Wait()
}

func TestHelloWorld_module(t *testing.T) { testHelloWorld(t, nil) }
func TestHelloWorld_worker(t *testing.T) {
	testHelloWorld(t, &testOptions{workerScript: "index.php"})
}
func testHelloWorld(t *testing.T, opts *testOptions) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		req := httptest.NewRequest("GET", fmt.Sprintf("http://example.com/index.php?i=%d", i), nil)
		w := httptest.NewRecorder()
		handler(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)
		assert.Equal(t, fmt.Sprintf("I am by birth a Genevese (%d)", i), string(body))
	}, opts)
}

func TestFinishRequest_module(t *testing.T) { testFinishRequest(t, nil) }
func TestFinishRequest_worker(t *testing.T) {
	testFinishRequest(t, &testOptions{workerScript: "finish-request.php"})
}
func testFinishRequest(t *testing.T, opts *testOptions) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		req := httptest.NewRequest("GET", fmt.Sprintf("http://example.com/finish-request.php?i=%d", i), nil)
		w := httptest.NewRecorder()
		handler(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)
		assert.Equal(t, fmt.Sprintf("This is output %d\n", i), string(body))
	}, opts)
}

func TestServerVariable_module(t *testing.T) {
	testServerVariable(t, nil)
}
func TestServerVariable_worker(t *testing.T) {
	testServerVariable(t, &testOptions{workerScript: "server-variable.php"})
}
func testServerVariable(t *testing.T, opts *testOptions) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		req := httptest.NewRequest("POST", fmt.Sprintf("http://example.com/server-variable.php/baz/bat?foo=a&bar=b&i=%d#hash", i), strings.NewReader("foo"))
		req.SetBasicAuth("kevin", "password")
		req.Header.Add("Content-Type", "text/plain")
		w := httptest.NewRecorder()
		handler(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)

		strBody := string(body)

		assert.Contains(t, strBody, "[REMOTE_HOST]")
		assert.Contains(t, strBody, "[REMOTE_USER] => kevin")
		assert.Contains(t, strBody, "[PHP_AUTH_USER] => kevin")
		assert.Contains(t, strBody, "[PHP_AUTH_PW] => password")
		assert.Contains(t, strBody, "[HTTP_AUTHORIZATION] => Basic a2V2aW46cGFzc3dvcmQ=")
		assert.Contains(t, strBody, "[DOCUMENT_ROOT]")
		assert.Contains(t, strBody, "[PHP_SELF] => /server-variable.php/baz/bat")
		assert.Contains(t, strBody, "[CONTENT_TYPE] => text/plain")
		assert.Contains(t, strBody, fmt.Sprintf("[QUERY_STRING] => foo=a&bar=b&i=%d#hash", i))
		assert.Contains(t, strBody, fmt.Sprintf("[REQUEST_URI] => /server-variable.php/baz/bat?foo=a&bar=b&i=%d#hash", i))
		assert.Contains(t, strBody, "[CONTENT_LENGTH]")
		assert.Contains(t, strBody, "[REMOTE_ADDR]")
		assert.Contains(t, strBody, "[REMOTE_PORT]")
		assert.Contains(t, strBody, "[REQUEST_SCHEME] => http")
		assert.Contains(t, strBody, "[DOCUMENT_URI]")
		assert.Contains(t, strBody, "[AUTH_TYPE]")
		assert.Contains(t, strBody, "[REMOTE_IDENT]")
		assert.Contains(t, strBody, "[REQUEST_METHOD] => POST")
		assert.Contains(t, strBody, "[SERVER_NAME] => example.com")
		assert.Contains(t, strBody, "[SERVER_PROTOCOL] => HTTP/1.1")
		assert.Contains(t, strBody, "[SCRIPT_FILENAME]")
		assert.Contains(t, strBody, "[SERVER_SOFTWARE] => FrankenPHP")
		assert.Contains(t, strBody, "[REQUEST_TIME_FLOAT]")
		assert.Contains(t, strBody, "[REQUEST_TIME]")
		assert.Contains(t, strBody, "[SERVER_PORT] => 80")
	}, opts)
}

func TestPathInfo_module(t *testing.T) { testPathInfo(t, nil) }
func TestPathInfo_worker(t *testing.T) {
	testPathInfo(t, &testOptions{workerScript: "server-variable.php"})
}
func testPathInfo(t *testing.T, opts *testOptions) {
	runTest(t, func(_ func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		handler := func(w http.ResponseWriter, r *http.Request) {
			cwd, _ := os.Getwd()
			testDataDir := cwd + "/testdata/"

			requestURI := r.URL.RequestURI()
			r.URL.Path = "/server-variable.php/pathinfo"

			rewriteRequest, err := frankenphp.NewRequestWithContext(r,
				frankenphp.WithRequestDocumentRoot(testDataDir, false),
				frankenphp.WithRequestEnv(map[string]string{"REQUEST_URI": requestURI}),
			)
			assert.NoError(t, err)

			err = frankenphp.ServeHTTP(w, rewriteRequest)
			assert.NoError(t, err)
		}

		req := httptest.NewRequest("GET", fmt.Sprintf("http://example.com/pathinfo/%d", i), nil)
		w := httptest.NewRecorder()
		handler(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)

		strBody := string(body)

		assert.Contains(t, strBody, "[PATH_INFO] => /pathinfo")
		assert.Contains(t, strBody, fmt.Sprintf("[REQUEST_URI] => /pathinfo/%d", i))
		assert.Contains(t, strBody, "[PATH_TRANSLATED] =>")
		assert.Contains(t, strBody, "[SCRIPT_NAME] => /server-variable.php")

	}, opts)
}

func TestHeaders_module(t *testing.T) { testHeaders(t, nil) }
func TestHeaders_worker(t *testing.T) { testHeaders(t, &testOptions{workerScript: "headers.php"}) }
func testHeaders(t *testing.T, opts *testOptions) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		req := httptest.NewRequest("GET", fmt.Sprintf("http://example.com/headers.php?i=%d", i), nil)
		w := httptest.NewRecorder()
		handler(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)

		assert.Equal(t, "Hello", string(body))
		assert.Equal(t, 201, resp.StatusCode)
		assert.Equal(t, "bar", resp.Header.Get("Foo"))
		assert.Equal(t, "bar2", resp.Header.Get("Foo2"))
		assert.Empty(t, resp.Header.Get("Invalid"))
		assert.Equal(t, fmt.Sprintf("%d", i), resp.Header.Get("I"))
	}, opts)
}

func TestInput_module(t *testing.T) { testInput(t, nil) }
func TestInput_worker(t *testing.T) { testInput(t, &testOptions{workerScript: "input.php"}) }
func testInput(t *testing.T, opts *testOptions) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		req := httptest.NewRequest("POST", "http://example.com/input.php", strings.NewReader(fmt.Sprintf("post data %d", i)))
		w := httptest.NewRecorder()
		handler(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)

		assert.Equal(t, fmt.Sprintf("post data %d", i), string(body))
		assert.Equal(t, "bar", resp.Header.Get("Foo"))
	}, opts)
}

func TestPostSuperGlobals_module(t *testing.T) { testPostSuperGlobals(t, nil) }
func TestPostSuperGlobals_worker(t *testing.T) {
	testPostSuperGlobals(t, &testOptions{workerScript: "super-globals.php"})
}
func testPostSuperGlobals(t *testing.T, opts *testOptions) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		formData := url.Values{"baz": {"bat"}, "i": {fmt.Sprintf("%d", i)}}
		req := httptest.NewRequest("POST", fmt.Sprintf("http://example.com/super-globals.php?foo=bar&iG=%d", i), strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		handler(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)

		assert.Contains(t, string(body), "'foo' => 'bar'")
		assert.Contains(t, string(body), fmt.Sprintf("'i' => '%d'", i))
		assert.Contains(t, string(body), "'baz' => 'bat'")
		assert.Contains(t, string(body), fmt.Sprintf("'iG' => '%d'", i))
	}, opts)
}

func TestCookies_module(t *testing.T) { testCookies(t, nil) }
func TestCookies_worker(t *testing.T) { testCookies(t, &testOptions{workerScript: "cookies.php"}) }
func testCookies(t *testing.T, opts *testOptions) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		req := httptest.NewRequest("GET", "http://example.com/cookies.php", nil)
		req.AddCookie(&http.Cookie{Name: "foo", Value: "bar"})
		req.AddCookie(&http.Cookie{Name: "i", Value: fmt.Sprintf("%d", i)})
		w := httptest.NewRecorder()
		handler(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)

		assert.Contains(t, string(body), "'foo' => 'bar'")
		assert.Contains(t, string(body), fmt.Sprintf("'i' => '%d'", i))
	}, opts)
}

func TestSession_module(t *testing.T) { testSession(t, nil) }
func TestSession_worker(t *testing.T) {
	testSession(t, &testOptions{workerScript: "session.php"})
}
func testSession(t *testing.T, opts *testOptions) {
	if opts == nil {
		opts = &testOptions{}
	}
	opts.realServer = true

	runTest(t, func(_ func(http.ResponseWriter, *http.Request), ts *httptest.Server, i int) {
		jar, err := cookiejar.New(&cookiejar.Options{})
		assert.NoError(t, err)

		client := &http.Client{Jar: jar}

		resp1, err := client.Get(ts.URL + "/session.php")
		assert.NoError(t, err)

		body1, _ := io.ReadAll(resp1.Body)
		assert.Equal(t, "Count: 0\n", string(body1))

		resp2, err := client.Get(ts.URL + "/session.php")
		assert.NoError(t, err)

		body2, _ := io.ReadAll(resp2.Body)
		assert.Equal(t, "Count: 1\n", string(body2))
	}, opts)
}

func TestPhpInfo_module(t *testing.T) { testPhpInfo(t, nil) }
func TestPhpInfo_worker(t *testing.T) { testPhpInfo(t, &testOptions{workerScript: "phpinfo.php"}) }
func testPhpInfo(t *testing.T, opts *testOptions) {
	var logOnce sync.Once
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		req := httptest.NewRequest("GET", fmt.Sprintf("http://example.com/phpinfo.php?i=%d", i), nil)
		w := httptest.NewRecorder()
		handler(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)

		logOnce.Do(func() {
			t.Log(string(body))
		})

		assert.Contains(t, string(body), "frankenphp")
		assert.Contains(t, string(body), fmt.Sprintf("i=%d", i))
	}, opts)
}

func TestPersistentObject_module(t *testing.T) { testPersistentObject(t, nil) }
func TestPersistentObject_worker(t *testing.T) {
	testPersistentObject(t, &testOptions{workerScript: "persistent-object.php"})
}
func testPersistentObject(t *testing.T, opts *testOptions) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		req := httptest.NewRequest("GET", fmt.Sprintf("http://example.com/persistent-object.php?i=%d", i), nil)
		w := httptest.NewRecorder()
		handler(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)

		assert.Equal(t, fmt.Sprintf(`request: %d
class exists: 1
id: obj1
object id: 1`, i), string(body))
	}, opts)
}

func TestAutoloader_module(t *testing.T) { testAutoloader(t, nil) }
func TestAutoloader_worker(t *testing.T) {
	testAutoloader(t, &testOptions{workerScript: "autoloader.php"})
}
func testAutoloader(t *testing.T, opts *testOptions) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		req := httptest.NewRequest("GET", fmt.Sprintf("http://example.com/autoloader.php?i=%d", i), nil)
		w := httptest.NewRecorder()
		handler(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)

		assert.Equal(t, fmt.Sprintf(`request %d
my_autoloader`, i), string(body))
	}, opts)
}

func TestLog_module(t *testing.T) { testLog(t, &testOptions{}) }
func TestLog_worker(t *testing.T) {
	testLog(t, &testOptions{workerScript: "log.php"})
}
func testLog(t *testing.T, opts *testOptions) {
	logger, logs := observer.New(zap.InfoLevel)
	opts.logger = zap.New(logger)

	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		req := httptest.NewRequest("GET", fmt.Sprintf("http://example.com/log.php?i=%d", i), nil)
		w := httptest.NewRecorder()
		handler(w, req)

		for logs.FilterMessage(fmt.Sprintf("request %d", i)).Len() <= 0 {
		}
	}, opts)
}

func TestConnectionAbort_module(t *testing.T) { testConnectionAbort(t, &testOptions{}) }
func TestConnectionAbort_worker(t *testing.T) {
	testConnectionAbort(t, &testOptions{workerScript: "connectionStatusLog.php"})
}
func testConnectionAbort(t *testing.T, opts *testOptions) {
	testFinish := func(finish string) {
		t.Run(fmt.Sprintf("finish=%s", finish), func(t *testing.T) {
			logger, logs := observer.New(zap.InfoLevel)
			opts.logger = zap.New(logger)

			runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
				req := httptest.NewRequest("GET", fmt.Sprintf("http://example.com/connectionStatusLog.php?i=%d&finish=%s", i, finish), nil)
				w := httptest.NewRecorder()

				ctx, cancel := context.WithCancel(req.Context())
				req = req.WithContext(ctx)
				cancel()
				handler(w, req)

				for logs.FilterMessage(fmt.Sprintf("request %d: 1", i)).Len() <= 0 {
				}
			}, opts)
		})
	}

	testFinish("0")
	testFinish("1")
}

func TestException_module(t *testing.T) { testException(t, &testOptions{}) }
func TestException_worker(t *testing.T) {
	testException(t, &testOptions{workerScript: "exception.php"})
}
func testException(t *testing.T, opts *testOptions) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		req := httptest.NewRequest("GET", fmt.Sprintf("http://example.com/exception.php?i=%d", i), nil)
		w := httptest.NewRecorder()
		handler(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)

		assert.Contains(t, string(body), "hello")
		assert.Contains(t, string(body), fmt.Sprintf(`Uncaught Exception: request %d`, i))
	}, opts)
}

func TestEarlyHints_module(t *testing.T) { testEarlyHints(t, &testOptions{}) }
func TestEarlyHints_worker(t *testing.T) {
	testEarlyHints(t, &testOptions{workerScript: "early-hints.php"})
}
func testEarlyHints(t *testing.T, opts *testOptions) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		var earlyHintReceived bool
		trace := &httptrace.ClientTrace{
			Got1xxResponse: func(code int, header textproto.MIMEHeader) error {
				switch code {
				case http.StatusEarlyHints:
					assert.Equal(t, "</style.css>; rel=preload; as=style", header.Get("Link"))
					assert.Equal(t, strconv.Itoa(i), header.Get("Request"))

					earlyHintReceived = true
				}

				return nil
			},
		}
		req := httptest.NewRequest("GET", fmt.Sprintf("http://example.com/early-hints.php?i=%d", i), nil)
		w := NewRecorder()
		w.ClientTrace = trace
		handler(w, req)

		assert.Equal(t, strconv.Itoa(i), w.Header().Get("Request"))
		assert.Equal(t, "", w.Header().Get("Link"))

		assert.True(t, earlyHintReceived)
	}, opts)
}

type streamResponseRecorder struct {
	*httptest.ResponseRecorder
	writeCallback func(buf []byte)
}

func (srr *streamResponseRecorder) Write(buf []byte) (int, error) {
	srr.writeCallback(buf)

	return srr.ResponseRecorder.Write(buf)
}

func TestFlush_module(t *testing.T) { testFlush(t, &testOptions{}) }
func TestFlush_worker(t *testing.T) {
	testFlush(t, &testOptions{workerScript: "flush.php"})
}
func testFlush(t *testing.T, opts *testOptions) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		var j int

		req := httptest.NewRequest("GET", fmt.Sprintf("http://example.com/flush.php?i=%d", i), nil)
		w := &streamResponseRecorder{httptest.NewRecorder(), func(buf []byte) {
			if j == 0 {
				assert.Equal(t, []byte("He"), buf)
			} else {
				assert.Equal(t, []byte(fmt.Sprintf("llo %d", i)), buf)
			}

			j++
		}}
		handler(w, req)

		assert.Equal(t, 2, j)
	}, opts)
}

func TestLargeRequest_module(t *testing.T) {
	testLargeRequest(t, &testOptions{})
}
func TestLargeRequest_worker(t *testing.T) {
	testLargeRequest(t, &testOptions{workerScript: "large-request.php"})
}
func testLargeRequest(t *testing.T, opts *testOptions) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		req := httptest.NewRequest(
			"POST",
			fmt.Sprintf("http://example.com/large-request.php?i=%d", i),
			strings.NewReader(strings.Repeat("f", 1_048_576)),
		)
		w := httptest.NewRecorder()
		handler(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)

		assert.Contains(t, string(body), fmt.Sprintf("Request body size: 1048576 (%d)", i))
	}, opts)
}

func TestVersion(t *testing.T) {
	v := frankenphp.Version()

	assert.GreaterOrEqual(t, v.MajorVersion, 8)
	assert.GreaterOrEqual(t, v.MinorVersion, 0)
	assert.GreaterOrEqual(t, v.ReleaseVersion, 0)
	assert.GreaterOrEqual(t, v.VersionID, 0)
	assert.NotEmpty(t, v.Version, 0)
}

func TestFiberNoCgo_module(t *testing.T) { testFiberNoCgo(t, &testOptions{}) }
func TestFiberNonCgo_worker(t *testing.T) {
	testFiberNoCgo(t, &testOptions{workerScript: "fiber-no-cgo.php"})
}
func testFiberNoCgo(t *testing.T, opts *testOptions) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		req := httptest.NewRequest("GET", fmt.Sprintf("http://example.com/fiber-no-cgo.php?i=%d", i), nil)
		w := httptest.NewRecorder()
		handler(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)

		assert.Equal(t, string(body), fmt.Sprintf("Fiber %d", i))
	}, opts)
}

func TestExecuteScriptCLI(t *testing.T) {
	if _, err := os.Stat("internal/testcli/testcli"); err != nil {
		t.Skip("internal/testcli/testcli has not been compiled, run `cd internal/testcli/ && go build`")
	}

	cmd := exec.Command("internal/testcli/testcli", "testdata/command.php", "foo", "bar")
	stdoutStderr, err := cmd.CombinedOutput()
	assert.Error(t, err)

	if exitError, ok := err.(*exec.ExitError); ok {
		assert.Equal(t, 3, exitError.ExitCode())
	}

	stdoutStderrStr := string(stdoutStderr)

	assert.Contains(t, stdoutStderrStr, `"foo"`)
	assert.Contains(t, stdoutStderrStr, `"bar"`)
	assert.Contains(t, stdoutStderrStr, "From the CLI")
}

func ExampleServeHTTP() {
	if err := frankenphp.Init(); err != nil {
		panic(err)
	}
	defer frankenphp.Shutdown()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		req, err := frankenphp.NewRequestWithContext(r, frankenphp.WithRequestDocumentRoot("/path/to/document/root", false))
		if err != nil {
			panic(err)
		}

		if err := frankenphp.ServeHTTP(w, req); err != nil {
			panic(err)
		}
	})
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func ExampleExecuteScriptCLI() {
	if len(os.Args) <= 1 {
		log.Println("Usage: my-program script.php")
		os.Exit(1)
	}

	os.Exit(frankenphp.ExecuteScriptCLI(os.Args[1], os.Args))
}

func BenchmarkHelloWorld(b *testing.B) {
	if err := frankenphp.Init(frankenphp.WithLogger(zap.NewNop())); err != nil {
		panic(err)
	}
	defer frankenphp.Shutdown()
	cwd, _ := os.Getwd()
	testDataDir := cwd + "/testdata/"

	handler := func(w http.ResponseWriter, r *http.Request) {
		req, err := frankenphp.NewRequestWithContext(r, frankenphp.WithRequestDocumentRoot(testDataDir, false))
		if err != nil {
			panic(err)
		}

		if err := frankenphp.ServeHTTP(w, req); err != nil {
			panic(err)
		}
	}

	req := httptest.NewRequest("GET", "http://example.com/index.php", nil)
	w := httptest.NewRecorder()

	for i := 0; i < b.N; i++ {
		handler(w, req)
	}
}

func BenchmarkEcho(b *testing.B) {
	if err := frankenphp.Init(frankenphp.WithLogger(zap.NewNop())); err != nil {
		panic(err)
	}
	defer frankenphp.Shutdown()
	cwd, _ := os.Getwd()
	testDataDir := cwd + "/testdata/"

	handler := func(w http.ResponseWriter, r *http.Request) {
		req, err := frankenphp.NewRequestWithContext(r, frankenphp.WithRequestDocumentRoot(testDataDir, false))
		if err != nil {
			panic(err)
		}
		if err := frankenphp.ServeHTTP(w, req); err != nil {
			panic(err)
		}
	}

	const body = `{
		"squadName": "Super hero squad",
		"homeTown": "Metro City",
		"formed": 2016,
		"secretBase": "Super tower",
		"active": true,
		"members": [
		  {
			"name": "Molecule Man",
			"age": 29,
			"secretIdentity": "Dan Jukes",
			"powers": ["Radiation resistance", "Turning tiny", "Radiation blast"]
		  },
		  {
			"name": "Madame Uppercut",
			"age": 39,
			"secretIdentity": "Jane Wilson",
			"powers": [
			  "Million tonne punch",
			  "Damage resistance",
			  "Superhuman reflexes"
			]
		  },
		  {
			"name": "Eternal Flame",
			"age": 1000000,
			"secretIdentity": "Unknown",
			"powers": [
			  "Immortality",
			  "Heat Immunity",
			  "Inferno",
			  "Teleportation",
			  "Interdimensional travel"
			]
		  }
		]
	  }`

	r := strings.NewReader(body)
	req := httptest.NewRequest("POST", "http://example.com/echo.php", r)
	w := httptest.NewRecorder()

	for i := 0; i < b.N; i++ {
		r.Reset(body)
		handler(w, req)
	}
}
