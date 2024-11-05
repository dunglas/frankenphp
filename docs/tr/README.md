# FrankenPHP: PHP için Modern Uygulama Sunucusu

<h1 align="center"><a href="https://frankenphp.dev"><img src="frankenphp.png" alt="FrankenPHP" width="600"></a></h1>

FrankenPHP, [Caddy](https://caddyserver.com/) web sunucusunun üzerine inşa edilmiş PHP için modern bir uygulama sunucusudur.

FrankenPHP, çarpıcı özellikleri sayesinde PHP uygulamalarınıza süper güçler kazandırır: [Early Hints*](https://frankenphp.dev/docs/early-hints/), [worker modu](https://frankenphp.dev/docs/worker/), [real-time yetenekleri](https://frankenphp.dev/docs/mercure/), otomatik HTTPS, HTTP/2 ve HTTP/3 desteği...

FrankenPHP herhangi bir PHP uygulaması ile çalışır ve worker modu ile resmi entegrasyonları sayesinde Laravel ve Symfony projelerinizi her zamankinden daha performanslı hale getirir.

FrankenPHP, PHP'yi `net/http` kullanarak herhangi bir uygulamaya yerleştirmek için bağımsız bir Go kütüphanesi olarak da kullanılabilir.

[*Frankenphp.dev*](https://frankenphp.dev) adresinden ve bu slayt üzerinden daha fazlasını öğrenin:

<a href="https://dunglas.dev/2022/10/frankenphp-the-modern-php-app-server-written-in-go/"><img src="https://dunglas.dev/wp-content/uploads/2022/10/frankenphp.png" alt="Slides" width="600"></a>

## Başlarken

### Docker

```console
docker run -v $PWD:/app/public \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

`https://localhost` adresine gidin ve keyfini çıkarın!

> [!TIP]
>
> `https://127.0.0.1` kullanmaya çalışmayın. `https://localhost` kullanın ve kendinden imzalı sertifikayı kabul edin.
> Kullanılacak alan adını değiştirmek için [`SERVER_NAME` ortam değişkenini](docs/config.md#environment-variables) kullanın.

### Binary Çıktısı

Docker kullanmayı tercih etmiyorsanız, Linux ve macOS için bağımsız FrankenPHP binary dosyası sağlıyoruz
[PHP 8.3](https://www.php.net/releases/8.3/en.php) ve en popüler PHP eklentilerini de içermekte: [FrankenPHP](https://github.com/dunglas/frankenphp/releases) indirin

Geçerli dizinin içeriğini başlatmak için çalıştırın:

```console
./frankenphp php-server
```

Ayrıca aşağıdaki tek komut satırı ile de çalıştırabilirsiniz:

```console
./frankenphp php-cli /path/to/your/script.php
```

## Docs

* [Worker modu](https://frankenphp.dev/docs/worker/)
* [Early Hints desteği (103 HTTP durum kodu)](https://frankenphp.dev/docs/early-hints/)
* [Real-time](https://frankenphp.dev/docs/mercure/)
* [Konfigürasyon](https://frankenphp.dev/docs/config/)
* [Docker imajları](https://frankenphp.dev/docs/docker/)
* [Production'a dağıtım](docs/production.md)
* [**Bağımsız** kendiliğinden çalıştırılabilir PHP uygulamaları oluşturma](https://frankenphp.dev/docs/embed/)
* [Statik binary'leri oluşturma](https://frankenphp.dev/docs/static/)
* [Kaynak dosyalarından derleme](https://frankenphp.dev/docs/compile/)
* [Laravel entegrasyonu](https://frankenphp.dev/docs/laravel/)
* [Bilinen sorunlar](https://frankenphp.dev/docs/known-issues/)
* [Demo uygulama (Symfony) ve kıyaslamalar](https://github.com/dunglas/frankenphp-demo)
* [Go kütüphane dokümantasonu](https://pkg.go.dev/github.com/dunglas/frankenphp)
* [Katkıda bulunma ve hata ayıklama](https://frankenphp.dev/docs/contributing/)

## Örnekler ve İskeletler

* [Symfony](https://github.com/dunglas/symfony-docker)
* [API Platform](https://api-platform.com/docs/distribution/)
* [Laravel](https://frankenphp.dev/docs/laravel/)
* [Sulu](https://sulu.io/blog/running-sulu-with-frankenphp)
* [WordPress](https://github.com/StephenMiracle/frankenwp)
* [Drupal](https://github.com/dunglas/frankenphp-drupal)
* [Joomla](https://github.com/alexandreelise/frankenphp-joomla)
* [TYPO3](https://github.com/ochorocho/franken-typo3)
