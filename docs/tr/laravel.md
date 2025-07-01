# Laravel

## Docker

Bir [Laravel](https://laravel.com) web uygulamasını FrankenPHP ile çalıştırmak, projeyi resmi Docker imajının `/app` dizinine monte etmek kadar kolaydır.

Bu komutu Laravel uygulamanızın ana dizininden çalıştırın:

```console
docker run -p 80:80 -p 443:443 -p 443:443/udp -v $PWD:/app dunglas/frankenphp
```

And tadını çıkarın!

## Yerel Kurulum

Alternatif olarak, Laravel projelerinizi FrankenPHP ile yerel makinenizden çalıştırabilirsiniz:

1. [Sisteminize karşılık gelen binary dosyayı indirin](https://github.com/php/frankenphp/releases)
2. Aşağıdaki yapılandırmayı Laravel projenizin kök dizinindeki `Caddyfile` adlı bir dosyaya ekleyin:

   ```caddyfile
   {
   	frankenphp
   }

   # Sunucunuzun alan adı
   localhost {
   	# Webroot'u public/ dizinine ayarlayın
   	root public/
   	# Sıkıştırmayı etkinleştir (isteğe bağlı)
   	encode zstd br gzip
   	# PHP dosyalarını public/ dizininden çalıştırın ve varlıkları sunun
   	php_server
   }
   ```

3. FrankenPHP'yi Laravel projenizin kök dizininden başlatın: `frankenphp run`

## Laravel Octane

Octane, Composer paket yöneticisi aracılığıyla kurulabilir:

```console
composer require laravel/octane
```

Octane'ı kurduktan sonra, Octane'ın yapılandırma dosyasını uygulamanıza yükleyecek olan `octane:install` Artisan komutunu çalıştırabilirsiniz:

```console
php artisan octane:install --server=frankenphp
```

Octane sunucusu `octane:frankenphp` Artisan komutu aracılığıyla başlatılabilir.

```console
php artisan octane:frankenphp
```

`octane:frankenphp` komutu aşağıdaki seçenekleri alabilir:

- `--host`: Sunucunun bağlanması gereken IP adresi (varsayılan: `127.0.0.1`)
- `--port`: Sunucunun erişilebilir olması gereken port (varsayılan: `8000`)
- `--admin-port`: Yönetici sunucusunun erişilebilir olması gereken port (varsayılan: `2019`)
- `--workers`: İstekleri işlemek için hazır olması gereken worker sayısı (varsayılan: `auto`)
- `--max-requests`: Sunucu yeniden yüklenmeden önce işlenecek istek sayısı (varsayılan: `500`)
- `--caddyfile`: FrankenPHP `Caddyfile` dosyasının yolu
- `--https`: HTTPS, HTTP/2 ve HTTP/3'ü etkinleştirin ve sertifikaları otomatik olarak oluşturup yenileyin
- `--http-redirect`: HTTP'den HTTPS'ye yeniden yönlendirmeyi etkinleştir (yalnızca --https geçilirse etkinleştirilir)
- `--watch`: Uygulamada kod değişikliği olduğunda sunucuyu otomatik olarak yeniden yükle
- `--poll`: Dosyaları bir ağ üzerinden izlemek için izleme sırasında dosya sistemi yoklamasını kullanın
- `--log-level`: Belirtilen günlük seviyesinde veya üzerinde günlük mesajları

Laravel Octane hakkında daha fazla bilgi edinmek için [Laravel Octane resmi belgelerine](https://laravel.com/docs/octane) göz atın.
