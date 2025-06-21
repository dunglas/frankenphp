# Writing PHP Extensions in Go

With FrankenPHP, you can **write PHP extensions in Go**, which allows you to create **high-performance native functions** that can be called directly from PHP. Your applications can leverage any existing or new Go library, as well as the infamous concurrency model of **goroutines right from your PHP code**.

Writing PHP extensions is typically done in C, but it's also possible to write them in other languages with a bit of extra work. PHP extensions allow you to leverage the power of low-level languages to extend PHP's functionalities, for example, by adding native functions or optimizing specific operations.

Thanks to Caddy modules, you can write PHP extensions in Go and integrate them very quickly into FrankenPHP.

> [!TIP]
> FrankenPHP is bundled with a subcommand to generate a PHP extension from a Go file, without
> the need to write any C code. This is a great way to get started with writing PHP extensions in Go.
> To know more about it, refer to the dedicated page on [the FrankenPHP Extension Generator](extensions-generator.md).

## Basic Function

We'll see how to write a simple PHP extension in Go that defines a new native function. This function will be called from PHP and will trigger a goroutine that logs a message in Caddy's logs. This function doesn't take any parameters and returns nothing.

### Create a New Module and Get PHP Sources

The first step to writing a PHP extension in Go is to create a new Go module. You can use the following command for this:

```console
go mod init github.com/my-account/my-module
```

Also, you need to [get the PHP sources](https://www.php.net/downloads.php) for the next steps. Once you have them, decompress them into the directory of your choice, not inside your Go module:

```console
tar xf php-*
```

### Define the Go Function

In your module, you need to define a new native function that will be called from PHP. To do this, create a file with the name you want, for example, `extension.go`, and add the following code:

```go
package ext_go

//#include "extension.h"
import "C"
import "github.com/caddyserver/caddy/v2"

func init() {
	C.register_extension()
}

//export go_print_something
func go_print_something() {
    go func() {
        caddy.Log().Info("Hello from a goroutine!")
    }()
}
```

We will detail what `frankenphp.RegisterExtension()` does later. For now, it's important to note that the `go_print_something` function uses the `//export` directive to indicate that it will be accessible in the C code we will write, thanks to CGO.

In this example, our new function will trigger a goroutine that logs a message in Caddy's logs.

### Define the PHP Function

To allow PHP to call our function, we need to define a corresponding PHP function. For this, we will create a stub file, for example, `extension.stub.php`, which will contain the following code:

```php
<?php

/** @generate-class-entries */

function go_print(): void {}
```

This file defines the signature of the `go_print()` function, which will be called from PHP. The `@generate-class-entries` directive allows PHP to automatically generate function entries for our extension.

This is not done manually but using a script provided in the PHP sources (make sure to adjust the path to the `gen_stub.php` script based on where your PHP sources are located):

```bash
php ../php-src/build/gen_stub.php extension.stub.php
```

This script will generate a file named `extension_arginfo.h` that contains the necessary information for PHP to know how to define and call our function.

### Write the Bridge Between Go and C

Now, we need to write the bridge between Go and C. Create a file named `extension.h` in your module directory with the following content:

```c
#ifndef _EXTENSION_H
#define _EXTENSION_H

void register_extension();

#endif
```

Next, create a file named `extension.c` that will perform the following steps:

* Include PHP headers;
* Declare our new native PHP function `go_print()`;
* Declare the extension metadata;
* Define the `register_extension()` function that will register our extension with PHP.

Let's start by including the required headers:

```c
#include <php.h>
#include "extension.h"
#include "extension_arginfo.h"

// Contains symbols exported by Go
#include "_cgo_export.h"

static int (*original_php_register_internal_extensions_func)(void) = NULL;
```

For PHP to recognize our extension, we need to use [one of the function pointers](https://github.com/php/php-src/blob/d585a5609d9d9ad9390eeb8c3109b64ff02bd632/main/main.c#L90) used internally by PHP. To avoid losing the original pointer, we store it in an `original_php_register_internal_extensions_func` variable to call it when registering our extension.

We then define our PHP function as a native language function:

```c
PHP_FUNCTION(go_print)
{
    if (zend_parse_parameters_none() == FAILURE) {
        RETURN_THROWS();
    }

    go_print_something();
}

zend_module_entry ext_module_entry = {
    STANDARD_MODULE_HEADER,
    "ext_go",
    ext_functions, /* Functions */
    NULL,          /* MINIT */
    NULL,          /* MSHUTDOWN */
    NULL,          /* RINIT */
    NULL,          /* RSHUTDOWN */
    NULL,          /* MINFO */
    "0.1.1",
    STANDARD_MODULE_PROPERTIES
};
```

In this case, our function takes no parameters and returns nothing. It simply calls the Go function we defined earlier, exported using the `//export` directive.

Secondly, we define the extension's metadata in a `zend_module_entry` structure, such as its name, version, and properties. This information is necessary for PHP to recognize and load our extension. Note that `ext_functions` is an array of pointers to the PHP functions we defined, and it was automatically generated by the `gen_stub.php` script in the `extension_arginfo.h` file.

Finally, we need to register our extension with PHP using the `register_extension()` functions (which we call in our Go code), but also taking care to call the original function pointer so as not to overwrite other extensions:

```c
PHPAPI int register_internal_extensions(void)
{
    if (original_php_register_internal_extensions_func != NULL && original_php_register_internal_extensions_func() != SUCCESS) {
        return FAILURE;
    }

    zend_module_entry *module = &ext_module_entry;
    if (zend_register_internal_module(module) == NULL) {
        return FAILURE;
    }

    return SUCCESS;
}

void register_extension() {
    original_php_register_internal_extensions_func = php_register_internal_extensions_func;
    php_register_internal_extensions_func = register_internal_extensions;
}
```

### Integrate the Extension into FrankenPHP

Our extension is now ready to be compiled and integrated into FrankenPHP. To do this, refer to the FrankenPHP [compilation documentation](compile.md) to learn how to compile FrankenPHP. The only difference is that you need to add your Go module to the compilation command. Using `xcaddy`, it will look like this:

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
CGO_CFLAGS=$(php-config --includes) \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
xcaddy build \
    --output frankenphp \
    --with github.com/my-account/my-module
```

All that's left is to create a PHP file to test our extension. For example, create an `index.php` file with the following content:

```php
<?php

go_print();
```

You can now run FrankenPHP with this file using `./frankenphp php-server`, and you should see the message "Hello from a goroutine!" in the Caddy logs when accessing `localhost`.

## Advanced Usage

Now that we know how to create a basic PHP extension in Go, let's complexify our example. We will now create a PHP function that takes a string as a parameter and returns its uppercase version.

### Define the PHP Function Stub

To define the new PHP function, we will modify our `extension.stub.php` file to include the new function signature:

```php
<?php

/** @generate-class-entries */

/**
 * Converts a string to uppercase.
 *
 * @param string $string The string to convert.
 * @return string The uppercase version of the string.
 */
function go_upper(string $string): string {}
```

> [!TIP]
> Don't neglect the documentation of your functions! You are likely to share your
> extension stubs with other developers to document how to use your extension and
> which features are available.

By regenerating the stub file with the `gen_stub.php` script, the `extension_arginfo.h` file should look like this:

```c
ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_go_upper, 0, 1, IS_STRING, 0)
    ZEND_ARG_TYPE_INFO(0, string, IS_STRING, 0)
ZEND_END_ARG_INFO()

ZEND_FUNCTION(go_upper);

static const zend_function_entry ext_functions[] = {
    ZEND_FE(go_upper, arginfo_go_upper)
    ZEND_FE_END
};
```

We can see that the `go_upper` function is defined with a parameter of type `string` and a return type of `string`.

### Type Juggling Between Go and PHP/C

Your Go function cannot directly accept a PHP string as a parameter. You need to convert it to a Go string. A PHP string consists of a pointer to the string data and its length. This is why we first need to define a structure in our `extension.h` file to hold this information:

```c
#ifndef _EXTENSION_H
#define _EXTENSION_H

typedef struct go_string {
    size_t len;
    char *data;
} go_string;

void register_extension(); // Register the extension with PHP, this part doesn't change

#endif
```

We can now write the bridge between Go and C in our `extension.c` file. We will define a new function that converts a PHP string to a Go string, and then we will implement the `go_upper` function:

```c
PHP_FUNCTION(go_upper)
{
    char *str;
    size_t string_len;

    ZEND_PARSE_PARAMETERS_START(1, 1)
        Z_PARAM_STRING(str, string_len)
    ZEND_PARSE_PARAMETERS_END();

    go_string param = {string_len, str};
    go_string *result = go_upper(&param);

    RETVAL_STRINGL(result->data, result->len);
}
```

You can learn more about the `ZEND_PARSE_PARAMETERS_START` and parameters parsing in the dedicated page of [the PHP Internals Book](https://www.phpinternalsbook.com/php7/extensions_design/php_functions.html#parsing-parameters-zend-parse-parameters). Here, we tell PHP that our function takes one mandatory parameter of type `string`. We then convert this string to a `go_string` structure that we can pass to our Go function. Finally, we call the Go function `go_upper` and return the result as a PHP string using `RETVAL_STRINGL`.

Because we'll create a new string to store the result of the Go function, we need to free it after returning it to PHP. We do this by freeing the memory after setting the return value. Let's define a C internal function that frees the memory allocated by a `go_string`:

```c
// Don't forget to add this function prototype in extension.h
void cleanup_go_string(go_string *s) {
    if (s && s->data) {
        free(s->data);
        free(s);
    }
}
```

A call to `cleanup_go_string(result);` should be added after `RETVAL_STRINGL(result->data, result->len);` in the `go_upper` function to ensure that the memory is properly freed after returning the value to PHP:

```c
PHP_FUNCTION(go_upper)
{
    // ...

    RETVAL_STRINGL(result->data, result->len);
    
    cleanup_go_string(result); // <-- Free the memory allocated by the Go function
}
```

There's only one thing left to do: implement the `go_upper` function in Go.

### Implement the Go Function

Our Go function will take a `go_string` as a parameter, convert it to a Go string, and return the uppercase version of the string as a new `go_string`. The conversion logic looks pretty big compared to the actual logic of the function, but keep in mind our function is not very complex. You can add any amount of logic in your Go functions, including calling other Go functions, using packages, triggering goroutines, and so on before performing the conversion back to what you need.

```go
//export go_upper
func go_upper(s *C.go_string) *C.go_string {
    data := C.GoStringN(s.data, C.int(s.len))
    upper := strings.ToUpper(data)

    result := (*C.go_string)(C.malloc(C.sizeof_go_string))
    result.data = C.CString(upper)
    result.len = C.size_t(len(upper))

    return result
}
```

Note the call to the `malloc()`, which implies that we need to free the memory we allocated. After allocating the memory, we convert our string to the format PHP expects, which is a `C.go_string` structure containing a pointer to the string data and its length. We use `C.CString()` to convert the Go string to a C string, and we set the length accordingly.

> [!TIP]
> In this example, we don't perform any error handling, but you should always check that pointers
> are not `nil` and that the data is valid before using it in your Go functions.

You can now compile your extension and edit the `index.php` file accordingly:

```php
<?php

print_r("Uppercase from Go: " . go_upper("Hello"));
```

If everything runs smoothly, you should see the output:

```text
Uppercase from Go: HELLO
```
