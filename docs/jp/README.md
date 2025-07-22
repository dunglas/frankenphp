# FrankenPHP: PHPのモダンAppサーバー

<h1 align="center"><a href="https://frankenphp.dev"><img src="frankenphp.png" alt="FrankenPHP" width="600"></a></h1>

FrankenPHPは [Caddy](https://caddyserver.com/) Webサーバー上に構築されたPHPのモダンアプリケーションサーバーです。

FrankenPHPは[_Early Hints_](https://frankenphp.dev/docs/early-hints/), [worker モード](https://frankenphp.dev/docs/worker/), [real-time 機能](https://frankenphp.dev/docs/mercure/), 自動の HTTPS, HTTP/2, HTTP/3 サポートなど素晴らしい機能であなたのPHPアプリに強力な力を与えます。

FrankenPHP はあらゆるPHPアプリで動作し、ワーカーモードとの統合により、LaravelおよびSymfonyプロジェクトをこれまで以上に高速化します。

FrankenPHPは、`net/http`を使用して任意のアプリにPHPを埋め込むためのスタンドアロンのGoライブラリとしても使用できます。

[**詳細については** _frankenphp.dev_](https://frankenphp.dev) やこのスライドから:

<a href="https://dunglas.dev/2022/10/frankenphp-the-modern-php-app-server-written-in-go/"><img src="https://dunglas.dev/wp-content/uploads/2022/10/frankenphp.png" alt="Slides" width="600"></a>

## はじめる

### スタンドアロンバイナリ

LinuxおよびmacOS向けに、[PHP 8.4](https://www.php.net/releases/8.4/en.php) と人気のあるPHP拡張機能を含む静的FrankenPHPバイナリを提供しています。

WindowsではFrankenPHPを実行するために [WSL](https://learn.microsoft.com/windows/wsl/) を利用する。

[FrankenPHPのダウンロード](https://github.com/php/frankenphp/releases)、もしくは次の行をターミナルにコピーすると自動で適切なバージョンがインストールします:

```console
curl https://frankenphp.dev/install.sh | sh
mv frankenphp /usr/local/bin/
```

カレントディレクトリのコンテンツを配信するには以下を実行:

```console
frankenphp php-server
```

コマンドラインスクリプトは以下で実行できます:

```console
frankenphp php-cli /path/to/your/script.php
```

### Docker

また、[Docker images](https://frankenphp.dev/docs/docker/)も利用可能です:

```console
docker run -v .:/app/public \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

`https://localhost`にアクセスしお楽しみください！

> [!TIP]
>
> `https://127.0.0.1`で使うことを試さないでください。 `https://localhost`を使用し、自己証明書を承諾します。
> ドメインを変更したいときは[`SERVER_NAME` 環境変数](docs/config.md#environment-variables)を使用してください。

### Homebrew

FrankenPHPはmacOSやLinuxの[Homebrew](https://brew.sh)パッケージとしても利用可能です。

インストール:

```console
brew install dunglas/frankenphp/frankenphp
```

カレントディレクトリのコンテンツを配信するために以下を実行:

```console
frankenphp php-server
```

## ドキュメント

- [Classicモード](https://frankenphp.dev/docs/classic/)
- [Workerモード](https://frankenphp.dev/docs/worker/)
- [Early Hints サポート (103 HTTP ステータスコード)](https://frankenphp.dev/docs/early-hints/)
- [Real-time](https://frankenphp.dev/docs/mercure/)
- [大きな静的ファイルを効率的に提供する](https://frankenphp.dev/docs/x-sendfile/)
- [Configuration](https://frankenphp.dev/docs/config/)
- [GoでPHP拡張を書く](https://frankenphp.dev/docs/extensions/)
- [Dockerイメージ](https://frankenphp.dev/docs/docker/)
- [本番デプロイ](https://frankenphp.dev/docs/production/)
- [パフォーマンス最適化](https://frankenphp.dev/docs/performance/)
- [**スタンドアロン** で実行可能なPHPアプリの作成](https://frankenphp.dev/docs/embed/)
- [静的バイナリの作成](https://frankenphp.dev/docs/static/)
- [ソースからコンパイル](https://frankenphp.dev/docs/compile/)
- [FrankenPHPのモニタリング](https://frankenphp.dev/docs/metrics/)
- [Laravelとの統合](https://frankenphp.dev/docs/laravel/)
- [既知の問題](https://frankenphp.dev/docs/known-issues/)
- [デモアプリ(Symfony)とベンチマーク](https://github.com/dunglas/frankenphp-demo)
- [Go ライブラリドキュメント](https://pkg.go.dev/github.com/dunglas/frankenphp)
- [貢献とデバック](https://frankenphp.dev/docs/contributing/)

## 例とスケルトン

- [Symfony](https://github.com/dunglas/symfony-docker)
- [API Platform](https://api-platform.com/docs/symfony)
- [Laravel](https://frankenphp.dev/docs/laravel/)
- [Sulu](https://sulu.io/blog/running-sulu-with-frankenphp)
- [WordPress](https://github.com/StephenMiracle/frankenwp)
- [Drupal](https://github.com/dunglas/frankenphp-drupal)
- [Joomla](https://github.com/alexandreelise/frankenphp-joomla)
- [TYPO3](https://github.com/ochorocho/franken-typo3)
- [Magento2](https://github.com/ekino/frankenphp-magento2)
