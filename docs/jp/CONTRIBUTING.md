# 貢献方法

## PHPのコンパイル

### Dockerを使う (Linux)

開発Dockerイメージをビルド:

```console
docker build -t frankenphp-dev -f dev.Dockerfile .
docker run --cap-add=SYS_PTRACE --security-opt seccomp=unconfined -p 8080:8080 -p 443:443 -p 443:443/udp -v $PWD:/go/src/app -it frankenphp-dev
```

このイメージは一般的な開発ツール (Go, GDB, Valgrind, Neovim...) を含んでいる。そして次のphp設定箇所を利用する。

- php.ini: `/etc/frankenphp/php.ini` 開発プリセットのphp.iniファイルがデフォルトで提供されている。
- 追加の設定ファイル: `/etc/frankenphp/php.d/*.ini`
- php 拡張: `/usr/lib/frankenphp/modules/`

もしあなたのdockerバージョンが23.0未満ならdockerignoreの影響でビルドが十敗します [pattern issue](https://github.com/moby/moby/pull/42676)。 ディレクトリに `.dockerignore`を追加しましょう。

```patch
 !testdata/*.php
 !testdata/*.txt
+!caddy
+!internal
```

### Dockerを使わない (Linux と macOS)

[こちらに従ってソースからコンパイルします](https://frankenphp.dev/docs/compile/)。そして`--debug`フラグを渡す。

## テストスイートを実行

```console
go test -tags watcher -race -v ./...
```

## Caddy module

FrankenPHP Caddy moduleでCaddyをビルド:

```console
cd caddy/frankenphp/
go build -tags watcher,brotli,nobadger,nomysql,nopgx
cd ../../
```

FrankenPHP Caddy moduleでCaddyを実行:

```console
cd testdata/
../caddy/frankenphp/frankenphp run
```

サーバーは `127.0.0.1:80` でリッスン:

> [注意！]
> もしDockerを使っているのならば80番ポートをバインドするかコンテナ内部で実行する必要がある。

```console
curl -vk http://127.0.0.1/phpinfo.php
```

## 最小のテストサーバー

最小のテストサーバーをビルド

```console
cd internal/testserver/
go build
cd ../../
```

テストサーバー実行:

```console
cd testdata/
../internal/testserver/testserver
```

サーバーは `127.0.0.1:80` でリッスン:

```console
curl -v http://127.0.0.1:8080/phpinfo.php
```

## Docker Imagesをローカルでビルド

Print bake plan:

```console
docker buildx bake -f docker-bake.hcl --print
```

amd64用のFrankenPHPイメージをローカルでビルド:

```console
docker buildx bake -f docker-bake.hcl --pull --load --set "*.platform=linux/amd64"
```

arm64用のFrankenPHPイメージをローカルでビルド:

```console
docker buildx bake -f docker-bake.hcl --pull --load --set "*.platform=linux/arm64"
```

arm64とamd64用のFrankenPHPイメージをゼロから構築し、Docker Hubにプッシュ:

```console
docker buildx bake -f docker-bake.hcl --pull --no-cache --push
```

## 静的ビルドによるセグメンテーション違反のデバッグ

1. GitHubからFrankenPHPバイナリのデバック版をダウンロードするかデバックシンボルを含むカスタム静的ビルドを作成:

   ```console
   docker buildx bake \
       --load \
       --set static-builder.args.DEBUG_SYMBOLS=1 \
       --set "static-builder.platform=linux/amd64" \
       static-builder
   docker cp $(docker create --name static-builder-musl dunglas/frankenphp:static-builder-musl):/go/src/app/dist/frankenphp-linux-$(uname -m) frankenphp
   ```

2. 現在のバージョンの`frankenphp`をデバッグ用FrankenPHP実行ファイルに置き換えます 
3. FrankenPHPを開始 (もしくは、GDBを使ってFrankenPHPを直接起動することもできます: `gdb --args frankenphp run`)
4. GDBでプロセスにアタッチ:

   ```console
   gdb -p `pidof frankenphp`
   ```

5. 必要なら、GDBシェルで`continue`と入力
6. FrankenPHPをクラッシュさせる
7. GDBシェルで `bt` と入力
8. 出力をコピー

## GitHub Actionsで静的ビルドによるセグメンテーション違反のデバッグ

1. `.github/workflows/tests.yml`を開く
2. PHPデバックシンボルを有効にする

   ```patch
       - uses: shivammathur/setup-php@v2
         # ...
         env:
           phpts: ts
   +       debug: true
   ```

3. コンテナへ接続するために `tmate` を有効にする

   ```patch
       - name: Set CGO flags
         run: echo "CGO_CFLAGS=$(php-config --includes)" >> "$GITHUB_ENV"
   +   - run: |
   +       sudo apt install gdb
   +       mkdir -p /home/runner/.config/gdb/
   +       printf "set auto-load safe-path /\nhandle SIG34 nostop noprint pass" > /home/runner/.config/gdb/gdbinit
   +   - uses: mxschmitt/action-tmate@v3
   ```

4. コンテナへ接続
5. `frankenphp.go` を開く
6. `cgosymbolizer` を有効にする

   ```patch
   -	//_ "github.com/ianlancetaylor/cgosymbolizer"
   +	_ "github.com/ianlancetaylor/cgosymbolizer"
   ```

7. モジュールをダウンロード: `go get`
8. コンテナ内でGDBが使える:

   ```console
   go test -tags watcher -c -ldflags=-w
   gdb --args frankenphp.test -test.run ^MyTest$
   ```

9. バグ修正後、これら全ての変更をリバートする

## その他開発情報

- [PHP embedding in uWSGI](https://github.com/unbit/uwsgi/blob/master/plugins/php/php_plugin.c)
- [PHP embedding in NGINX Unit](https://github.com/nginx/unit/blob/master/src/nxt_php_sapi.c)
- [PHP embedding in Go (go-php)](https://github.com/deuill/go-php)
- [PHP embedding in Go (GoEmPHP)](https://github.com/mikespook/goemphp)
- [PHP embedding in C++](https://gist.github.com/paresy/3cbd4c6a469511ac7479aa0e7c42fea7)
- [Extending and Embedding PHP by Sara Golemon](https://books.google.fr/books?id=zMbGvK17_tYC&pg=PA254&lpg=PA254#v=onepage&q&f=false)
- [What the heck is TSRMLS_CC, anyway?](http://blog.golemon.com/2006/06/what-heck-is-tsrmlscc-anyway.html)
- [SDL bindings](https://pkg.go.dev/github.com/veandco/go-sdl2@v0.4.21/sdl#Main)

## Docker関連の情報

- [Bakeファイル定義](https://docs.docker.com/build/customize/bake/file-definition/)
- [docker buildx build](https://docs.docker.com/engine/reference/commandline/buildx_build/)

## 役に立つコマンド

```console
apk add strace util-linux gdb
strace -e 'trace=!futex,epoll_ctl,epoll_pwait,tgkill,rt_sigreturn' -p 1
```

## ドキュメントの翻訳

ドキュメントとサイトを新しい言語に翻訳する場合、次の手順に従ってください:

1. このリポジトリの `docs/` ディレクトリに、言語の2文字のISOコードで名付けた新しいディレクトリを作成する
2. `docs/` ディレクトリのルートにあるすべての `.md` ファイルを新しいディレクトリにコピーする (常に最新状態の英語版を翻訳のソースとして使用してください)
3. ルートディレクトリから新しいディレクトリに`README.md`ファイルと`CONTRIBUTING.md`ファイルをコピーする
4. ファイルの内容を翻訳しますが、ファイル名は変更しないでください。また、`> [!` で始まる文字列も翻訳しないでください (GitHubの特別なマークアップです)
5. 翻訳のプルリクエストを作成
6. [サイトリポジトリ](https://github.com/dunglas/frankenphp-website/tree/main)で`content/`, `data/`, `i18n/`ディレクトリの中にあるファイルをコピーして翻訳する
7. 作成されたYAMLファイルの値を翻訳する
8. サイトリポジトリでプルリクエストを公開する
