# Known Issues

## Fibers

Calling PHP functions and language constructs that themselves call [cgo](https://go.dev/blog/cgo) in [Fibers](https://www.php.net/manual/en/language.fibers.php) is known to cause crashes.

This issue [is being worked on by the Go project](https://github.com/golang/go/issues/62130).


In the meantime, one solution is not to use constructs (like `echo`) and functions (like `header()`) that delegate to Go from inside Fibers.

This code will likely crash because it uses `echo` in the Fiber:

```php
$fiber = new Fiber(function() {
    echo 'In the Fiber'.PHP_EOL;
    echo 'Still inside'.PHP_EOL;
});
$fiber->start();
```

Instead, return the value from the Fiber and use it outside:

```php
$fiber = new Fiber(function() {
    Fiber::suspend('In the Fiber'.PHP_EOL));
    Fiber::suspend('Still inside'.PHP_EOL));
});
echo $fiber->start();
echo $fiber->resume();
$fiber->resume();
```

## Unsupported PHP Extensions

The following extensions are known not to be compatible with FrankenPHP:

| Name                                                        | Reason          | Alternatives                                                                                                         |
| ----------------------------------------------------------- | --------------- | -------------------------------------------------------------------------------------------------------------------- |
| [imap](https://www.php.net/manual/en/imap.installation.php) | Not thread-safe | [javanile/php-imap2](https://github.com/javanile/php-imap2), [webklex/php-imap](https://github.com/Webklex/php-imap) |

## get_browser

The [get_browser](https://www.php.net/manual/en/function.get-browser.php) function seems to have a bad performance after a while. A workaround is to cache (e.g. with APCU) the results per User Agent, as they are static anyway.
