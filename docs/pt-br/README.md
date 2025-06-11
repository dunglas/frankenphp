# FrankenPHP: Servidor de aplicações moderno para PHP

<h1 align="center"><a href="https://frankenphp.dev"><img src="frankenphp.png" alt="FrankenPHP" width="600"></a></h1>

O FrankenPHP é um servidor de aplicações moderno para PHP, construído sobre o
servidor web [Caddy](https://caddyserver.com/).

O FrankenPHP oferece superpoderes às suas aplicações PHP graças aos seus
recursos impressionantes:
[_Early Hints_](https://frankenphp.dev/docs/early-hints/),
[modo worker](https://frankenphp.dev/docs/worker/),
[recursos em tempo real](https://frankenphp.dev/docs/mercure/), suporte
automático a HTTPS, HTTP/2 e HTTP/3...

O FrankenPHP funciona com qualquer aplicação PHP e torna seus projetos Laravel e
Symfony mais rápidos do que nunca, graças às suas integrações oficiais com o
modo worker.

O FrankenPHP também pode ser usado como uma biblioteca Go independente para
incorporar PHP em qualquer aplicação usando `net/http`.

[**Saiba mais** em _frankenphp.dev_](https://frankenphp.dev) e neste conjunto de
slides:

<a href="https://dunglas.dev/2022/10/frankenphp-the-modern-php-app-server-written-in-go/"><img src="https://dunglas.dev/wp-content/uploads/2022/10/frankenphp.png" alt="Slides" width="600"></a>

## Começando

### Binário independente

Fornecemos binários estáticos do FrankenPHP para Linux e macOS contendo o
[PHP 8.4](https://www.php.net/releases/8.4/pt_BR.php) e as extensões PHP mais
populares.

No Windows, use [WSL](https://learn.microsoft.com/windows/wsl/) para executar o
FrankenPHP.

[Baixe o FrankenPHP](https://github.com/php/frankenphp/releases) ou copie esta
linha no seu terminal para instalar automaticamente a versão apropriada para sua
plataforma:

```console
curl https://frankenphp.dev/install.sh | sh
mv frankenphp /usr/local/bin/
```

Para servir o conteúdo do diretório atual, execute:

```console
frankenphp php-server
```

Você também pode executar scripts de linha de comando com:

```console
frankenphp php-cli /caminho/para/seu/script.php
```

### Docker

Alternativamente, [imagens do Docker](https://frankenphp.dev/docs/docker/) estão
disponíveis:

```console
docker run -v .:/app/public \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

Acesse `https://localhost` e divirta-se!

> [!TIP]
>
> Não tente usar `https://127.0.0.1`.
> Use `https://localhost` e aceite o certificado autoassinado.
> Use a
> [variável de ambiente `SERVER_NAME`](docs/config.md#environment-variables)
> para alterar o domínio a ser usado.

### Homebrew

O FrankenPHP também está disponível como um pacote [Homebrew](https://brew.sh)
para macOS e Linux.

Para instalá-lo:

```console
brew install dunglas/frankenphp/frankenphp
```

Para servir o conteúdo do diretório atual, execute:

```console
frankenphp php-server
```

## Documentação

- [Modo clássico](https://frankenphp.dev/docs/classic/)
- [Modo Worker](https://frankenphp.dev/docs/worker/)
- [Suporte a Early Hints (código de status HTTP 103)](https://frankenphp.dev/docs/early-hints/)
- [Tempo real](https://frankenphp.dev/docs/mercure/)
- [Servindo grandes arquivos estáticos com eficiência](https://frankenphp.dev/docs/x-sendfile/)
- [Configuração](https://frankenphp.dev/docs/config/)
- [Imagens Docker](https://frankenphp.dev/docs/docker/)
- [Implantação em produção](https://frankenphp.dev/docs/production/)
- [Otimização de desempenho](https://frankenphp.dev/docs/performance/)
- [Crie aplicações PHP **independentes** e autoexecutáveis](https://frankenphp.dev/docs/embed/)
- [Crie binários estáticos](https://frankenphp.dev/docs/static/)
- [Compile a partir do código-fonte](https://frankenphp.dev/docs/compile/)
- [Monitorando o FrankenPHP](https://frankenphp.dev/docs/metrics/)
- [Integração com Laravel](https://frankenphp.dev/docs/laravel/)
- [Problemas conhecidos](https://frankenphp.dev/docs/known-issues/)
- [Aplicação de demonstração (Symfony) e benchmarks](https://github.com/dunglas/frankenphp-demo)
- [Documentação da biblioteca Go](https://pkg.go.dev/github.com/dunglas/frankenphp)
- [Contribuindo e depurando](https://frankenphp.dev/docs/contributing/)

## Exemplos e esqueletos

- [Symfony](https://github.com/dunglas/symfony-docker)
- [API Platform](https://api-platform.com/docs/symfony)
- [Laravel](https://frankenphp.dev/docs/laravel/)
- [Sulu](https://sulu.io/blog/running-sulu-with-frankenphp)
- [WordPress](https://github.com/StephenMiracle/frankenwp)
- [Drupal](https://github.com/dunglas/frankenphp-drupal)
- [Joomla](https://github.com/alexandreelise/frankenphp-joomla)
- [TYPO3](https://github.com/ochorocho/franken-typo3)
- [Magento2](https://github.com/ekino/frankenphp-magento2)
