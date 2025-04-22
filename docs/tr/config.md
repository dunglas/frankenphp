# Konfigürasyon

FrankenPHP, Caddy'nin yanı sıra Mercure ve Vulcain modülleri [Caddy tarafından desteklenen formatlar](https://caddyserver.com/docs/getting-started#your-first-config) kullanılarak yapılandırılabilir.

Docker imajlarında] (docker.md), `Caddyfile` `/etc/caddy/Caddyfile` adresinde bulunur.
Statik ikili, başlatıldığı dizinde `Caddyfile` dosyasını arayacaktır.

PHP'nin kendisi [bir `php.ini` dosyası kullanılarak yapılandırılabilir](https://www.php.net/manual/tr/configuration.file.php).

Varsayılan olarak, Docker imajlarıyla birlikte verilen PHP ve statik ikili dosyada bulunan PHP, FrankenPHP'nin başlatıldığı dizinde ve `/usr/local/etc/php/` içinde bir `php.ini` dosyası arayacaktır. Ayrıca `.ini` ile biten tüm dosyaları `/usr/local/etc/php/conf.d/` dizininden yükleyecektir.

Öntanımlı olarak `php.ini` dosyası yoktur, PHP projesi tarafından sağlanan resmi bir şablonu kopyalamanız gerekir.
Docker'da şablonlar imajlar içinde sağlanır:

```dockerfile
FROM dunglas/frankenphp

# Developement:
RUN cp $PHP_INI_DIR/php.ini-development $PHP_INI_DIR/php.ini

# Veya production:
RUN cp $PHP_INI_DIR/php.ini-production $PHP_INI_DIR/php.ini
```

Docker kullanmıyorsanız, [PHP kaynak kodu](https://github.com/php/php-src/) ile birlikte verilen `php.ini-production` veya `php.ini-development` dosyalarından birini kopyalayın.

## Caddyfile Konfigürasyonu

FrankenPHP yürütücüsünü kaydetmek için `frankenphp` [global seçenek](https://caddyserver.com/docs/caddyfile/concepts#global-options) ayarlanmalıdır, ardından PHP uygulamanızı sunmak için site blokları içinde `php_server` veya `php` [HTTP yönergeleri](https://caddyserver.com/docs/caddyfile/concepts#directives) kullanılabilir.

Minimal örnek:

```caddyfile
{
	# FrankenPHP'yi aktif et
	frankenphp
}

localhost {
	# Sıkıştırmayı etkinleştir (isteğe bağlı)
	encode zstd br gzip
	# Geçerli dizindeki PHP dosyalarını çalıştırın ve varlıkları sunun
	php_server
}
```

İsteğe bağlı olarak, oluşturulacak iş parçacığı sayısı ve sunucuyla birlikte başlatılacak [işçi betikleri] (worker.md) global seçenek altında belirtilebilir.

```caddyfile
{
	frankenphp {
		num_threads <num_threads> # Başlatılacak PHP iş parçacığı sayısını ayarlar. Varsayılan: Mevcut CPU çekirdek sayısının 2 katı.
		worker {
			file <path> # Çalışan komut dosyasının yolunu ayarlar.
			num <num> # Başlatılacak PHP iş parçacığı sayısını ayarlar, varsayılan değer mevcut CPU çekirdek sayısının 2 katıdır.
			env <key> <value> # Ek bir ortam değişkenini verilen değere ayarlar. Birden fazla ortam değişkeni için birden fazla kez belirtilebilir.
		}
	}
}

# ...
```

Alternatif olarak, `worker` seçeneğinin tek satırlık kısa formunu kullanabilirsiniz:

```caddyfile
{
	frankenphp {
		worker <file> <num>
	}
}

# ...
```

Aynı sunucuda birden fazla uygulamaya hizmet veriyorsanız birden fazla işçi de tanımlayabilirsiniz:

```caddyfile
{
	frankenphp {
		worker /path/to/app/public/index.php <num>
		worker /path/to/other/public/index.php <num>
	}
}

app.example.com {
	root /path/to/app/public
	php_server
}

other.example.com {
	root /path/to/other/public
	php_server
}

# ...
```

Genellikle ihtiyacınız olan şey `php_server` yönergesini kullanmaktır,
ancak tam kontrole ihtiyacınız varsa, daha düşük seviyeli `php` yönergesini kullanabilirsiniz:

php_server` yönergesini kullanmak bu yapılandırmay ile aynıdır:

```caddyfile
route {
	# Dizin istekleri için sondaki eğik çizgiyi, diğer adıyla taksim işaretini ekleyin
	@canonicalPath {
		file {path}/index.php
		not path */
	}
	redir @canonicalPath {path}/ 308
	# İstenen dosya mevcut değilse, dizin dosyalarını deneyin
	@indexFiles file {
		try_files {path} {path}/index.php index.php
		split_path .php
	}
	rewrite @indexFiles {http.matchers.file.relative}
	# FrankenPHP!
	@phpFiles path *.php
	php @phpFiles
	file_server
}
```

php_server`ve`php` yönergeleri aşağıdaki seçeneklere sahiptir:

```caddyfile
php_server [<matcher>] {
	root <directory> # Sitenin kök klasörünü ayarlar. Öntanımlı: `root` yönergesi.
	split_path <delim...> # URI'yi iki parçaya bölmek için alt dizgeleri ayarlar. İlk eşleşen alt dizge "yol bilgisini" yoldan ayırmak için kullanılır. İlk parça eşleşen alt dizeyle sonlandırılır ve gerçek kaynak (CGI betiği) adı olarak kabul edilir. İkinci parça betiğin kullanması için PATH_INFO olarak ayarlanacaktır. Varsayılan: `.php`
	resolve_root_symlink false # Varsa, sembolik bir bağlantıyı değerlendirerek `root` dizininin gerçek değerine çözümlenmesini devre dışı bırakır (varsayılan olarak etkindir).
	env <key> <value> # Ek bir ortam değişkenini verilen değere ayarlar. Birden fazla ortam değişkeni için birden fazla kez belirtilebilir.
}
```

## Ortam Değişkenleri

Aşağıdaki ortam değişkenleri `Caddyfile` içinde değişiklik yapmadan Caddy yönergelerini entegre etmek için kullanılabilir:

- `SERVER_NAME`: değiştirin [dinlenecek adresleri](https://caddyserver.com/docs/caddyfile/concepts#addresses), sağlanan ana bilgisayar adları oluşturulan TLS sertifikası için de kullanılacaktır
- `CADDY_GLOBAL_OPTIONS`: entegre edin [global seçenekler](https://caddyserver.com/docs/caddyfile/options)
- `FRANKENPHP_CONFIG`: `frankenphp` yönergesi altına yapılandırma entegre edin

FPM ve CLI SAPI'lerinde olduğu gibi, ortam değişkenleri varsayılan olarak `$_SERVER` süper globalinde gösterilir.

[`variables_order`'a ait PHP yönergesinin](https://www.php.net/manual/en/ini.core.php#ini.variables-order) `S` değeri bu yönergede `E`'nin başka bir yere yerleştirilmesinden bağımsız olarak her zaman `ES` ile eş değerdir.

## PHP konfigürasyonu

Ek olarak [PHP yapılandırma dosyalarını](https://www.php.net/manual/en/configuration.file.php#configuration.file.scan) yüklemek için
`PHP_INI_SCAN_DIR` ortam değişkeni kullanılabilir.
Ayarlandığında, PHP verilen dizinlerde bulunan `.ini` uzantılı tüm dosyaları yükleyecektir.

## Hata Ayıklama Modunu Etkinleştirin

Docker imajını kullanırken, hata ayıklama modunu etkinleştirmek için `CADDY_GLOBAL_OPTIONS` ortam değişkenini `debug` olarak ayarlayın:

```console
docker run -v $PWD:/app/public \
    -e CADDY_GLOBAL_OPTIONS=debug \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```
