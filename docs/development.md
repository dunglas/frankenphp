# FrankenPHP Development Environment Setup

---

## For Windows: WSL2 Setup

1. Install WSL2:

   ```powershell
   wsl --install
   ```

2. Install a Linux distribution (example: AlmaLinux 10) via e.g. `wsl --install AlmaLinux-10`.

---

## Initial setup

Follow the guide on [compiling from sources](compile.md).
We will assume you installed these things into the following paths:

- go: `/usr/local/go`
- cloned `~/php-src`
- php: `/usr/local/bin/php`
- cloned `~/frankenphp`

## CLion Setup for CGO glue/PHP Source Development

### 1. Install CLion (on your host OS)

- Download from: [https://www.jetbrains.com/clion/download/](https://www.jetbrains.com/clion/download/)

- Launch (if on Windows, in WSL):

  ```bash
  clion &>/dev/null
  ```

### 2. Open Project in CLion

- Open CLion → Open → Select the `~/frankenphp` directory
- Add a build chain: Settings → Build, Execution, Deployment → Custom Build Targets
- Select any Build Target, under `Build` set up an External Tool (call it e.g. go build)
- Set up a wrapper script that builds frankenphp for you, called `go_compile_frankenphp.sh`

```bash
export CGO_CFLAGS="-O0 -g $(php-config --includes)"
export CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)"
go build -tags=nobadger,nomysql,nopgx
```

- Under Program, select `go_compile_frankenphp.sh`
- Leave Arguments blank
- Working Directory: `~/frankenphp/caddy/frankenphp`

### 3. Configure Run Targets

- Go to **Run → Edit Configurations**
- Create:

  - **frankenphp**:

    - Type: Native Application
    - Target: select the `go build` target you created
    - Executable: `~/frankenphp/caddy/frankenphp/frankenphp`
    - Arguments: the arguments you want to start frankenphp with, e.g. `php-cli test.php`

### 4. Debug Go files from CLion

- Right click on a *.go file in the Project view on the left
- Override file type
- C/C++

Now you can place breakpoints in C, C++ and Go files.
To get syntax highlighting for imports from php-src, you may need to tell CLion about the include paths. Create a
`compile_flags.txt` file in `~/frankenphp` with the following contents:

```gcc
-I/usr/local/include/php
-I/usr/local/include/php/Zend
-I/usr/local/include/php/main
-I/usr/local/include/php/TSRM
```

---

## GoLand Setup for FrankenPHP Development

Use GoLand for primary Go development, but the debugger cannot debug C code.

### 1. Install GoLand (on your host OS)

- Download from: [https://www.jetbrains.com/go/download/](https://www.jetbrains.com/go/download/)

- Launch (if on Windows, in WSL):

  ```bash
  goland &>/dev/null
  ```

### 2. Open in GoLand

- Launch GoLand → Open → Select the `~/frankenphp` directory

---

### Go Configuration

- Select Go Build
  - Name `frankenphp`
  - Run kind: Directory

- Directory: `~/frankenphp/caddy/frankenphp`
- Output directory: `~/frankenphp/caddy/frankenphp`
- Working directory: `~/frankenphp/caddy/frankenphp`
- Environment (adjust for your $(php-config ...) output):
  `CGO_CFLAGS=-O0 -g -I/usr/local/include/php -I/usr/local/include/php/main -I/usr/local/include/php/TSRM -I/usr/local/include/php/Zend -I/usr/local/include/php/ext -I/usr/local/include/php/ext/date/lib;CGO_LDFLAGS=-lm -lpthread -lsqlite3 -lxml2 -lbrotlienc -lbrotlidec -lbrotlicommon -lwatcher`
- Go tool arguments: `-tags=nobadger,nomysql,nopgx`
- Program arguments: e.g. `php-cli -i`

You can now place breakpoints and debug through Go code when you debug the `frankenphp` configuration, but breakpoints
in C code will not work.

---

## Debugging and Integration Notes

- Use **CLion** for debugging PHP internals and `cgo` glue code
- Use **GoLand** for primary Go development and debugging
- FrankenPHP can be added as a run configuration in CLion for unified C/Go debugging if needed, but syntax highlighting
  won't work in Go files

---
