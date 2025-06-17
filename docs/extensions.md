# Writing PHP Extensions in Go

With FrankenPHP, you can **write PHP extensions in Go**, which allows you to create **high-performance native functions** that can be called directly from PHP. Your applications can leverage any existing or new Go library, as well as the infamous concurrency model of **goroutines right from your PHP code**.

Writing PHP extensions is typically done in C, but it's also possible to write them in other languages with a bit of extra work. PHP extensions allow you to leverage the power of low-level languages to extend PHP's functionalities, for example, by adding native functions or optimizing specific operations.

Thanks to Caddy modules, you can write PHP extensions in Go and integrate them very quickly into FrankenPHP.

## Two Approaches

FrankenPHP provides two ways to create PHP extensions in Go:

1. **Manual Implementation** - Full control over the extension structure
2. **Using the Extension Generator** - A simplified approach that generates all necessary boilerplate

We'll start with the manual approach to understand how extensions work under the hood, then show how the generator simplifies the process.

## Manual Implementation

If you want to understand how extensions work or need full control over your extension, you can write them manually. This approach gives you complete control but requires more boilerplate code.

### Basic Function

We'll see how to write a simple PHP extension in Go that defines a new native function. This function will be called from PHP and will trigger a goroutine that logs a message in Caddy's logs. This function doesn't take any parameters and returns nothing.

#### Create a New Module and Get PHP Sources

The first step to writing a PHP extension in Go is to create a new Go module. You can use the following command for this:

```console
go mod init github.com/my-account/my-module
```

Also, you need to [get the PHP sources](https://www.php.net/downloads.php) for the next steps. Once you have them, decompress them into the directory of your choice, not inside your Go module:

```console
tar xf php-*
```

#### Define the Go Function

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

We will detail what `C.register_extension()` does later. For now, it's important to note that the `go_print_something` function uses the `//export` directive to indicate that it will be accessible in the C code we will write, thanks to CGO.

In this example, our new function will trigger a goroutine that logs a message in Caddy's logs.

#### Define the PHP Function

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

#### Write the Bridge Between Go and C

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

### Advanced Usage

Now that we know how to create a basic PHP extension in Go, let's complexify our example. We will now create a PHP function that takes a string as a parameter and returns its uppercase version.

#### Define the PHP Function Stub

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
> Don't neglect the documentation of your functions! You are likely to share your extension stubs with other developers to document how to use your extension and which features are available.

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

#### Type Juggling Between Go and PHP/C

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

#### Implement the Go Function

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
> In this example, we don't perform any error handling, but you should always check that pointers are not `nil` and that the data is valid before using it in your Go functions.

## Using the Extension Generator

FrankenPHP is bundled with a tool that allows you **to create a PHP extension** only using Go. **No need to write C code** or use CGO directly: FrankenPHP also includes a **public types API** to help you write your extensions in Go without having to worry about **the type juggling between PHP/C and Go**.

> [!TIP]
> If you want to understand how extensions can be written in Go from scratch, you can read the manual implementation section above demonstrating how to write a PHP extension in Go without using the generator.

Keep in mind that this tool is **not a full-fledged extension generator**. It is meant to help you write simple extensions in Go, but it does not provide the most advanced features of PHP extensions. If you need to write a more **complex and optimized** extension, you may need to write some C code or use CGO directly.

### Creating a Native Function

We will first see how to create a new native function in Go that can be called from PHP.

#### Prerequisites

As covered in the manual implementation section above, you need to [get the PHP sources](https://www.php.net/downloads.php) and create a new Go module. If you haven't done so already, follow the steps in [Create a New Module and Get PHP Sources](#create-a-new-module-and-get-php-sources).

#### Writing the Extension

Everything is now setup to write your native function in Go. Create a new file named `stringext.go`. Our first function will take a string as an argument, the number of times to repeat it, a boolean to indicate whether to reverse the string, and return the resulting string. This should look like this:

```go
import (
    "C"
    "github.com/dunglas/frankenphp"
    "strings"
)

//export_php:function repeat_this(string $str, int $count, bool $reverse): string
func repeat_this(s *C.zend_string, count int64, reverse bool) unsafe.Pointer {
    str := frankenphp.GoString(unsafe.Pointer(s))

    result := strings.Repeat(str, int(count))
    if reverse {
        runes := []rune(result)
        for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
            runes[i], runes[j] = runes[j], runes[i]
        }
        result = string(runes)
    }

    return frankenphp.PHPString(result, false)
}
```

There are two important things to note here:

* A directive comment `//export_php:function` defines the function signature in PHP. This is how the generator knows how to generate the PHP function with the right parameters and return type;
* The function must return an `unsafe.Pointer`. FrankenPHP provides an API to help you with type juggling between C and Go.

While the first point speaks for itself, the second may be harder to apprehend. Let's take a deeper dive to type juggling in the next section.

### Type Juggling

While some variable types have the same memory representation between C/PHP and Go, some types require more logic to be directly used. This is maybe the hardest part when it comes to writing extensions because it requires understanding internals of the Zend Engine and how variables are stored internally in PHP. This table summarizes what you need to know:

| PHP type           | Go type          | Direct conversion | C to Go helper        | Go to C helper         | Class Methods Support |
|--------------------|------------------|-------------------|-----------------------|------------------------|-----------------------|
| `int`              | `int64`          | ✅                 | -                     | -                      | ✅                     |
| `?int`             | `*int64`         | ✅                 | -                     | -                      | ✅                     |
| `float`            | `float64`        | ✅                 | -                     | -                      | ✅                     |
| `?float`           | `*float64`       | ✅                 | -                     | -                      | ✅                     |
| `bool`             | `bool`           | ✅                 | -                     | -                      | ✅                     |
| `?bool`            | `*bool`          | ✅                 | -                     | -                      | ✅                     |
| `string`/`?string` | `*C.zend_string` | ❌                 | frankenphp.GoString() | frankenphp.PHPString() | ✅                     |
| `array`            | `slice`/`map`    | ❌                 | _Not yet implemented_ | _Not yet implemented_  | ❌                     |
| `object`           | `struct`         | ❌                 | _Not yet implemented_ | _Not yet implemented_  | ❌                     |

> [!NOTE]
> This table is not exhaustive yet and will be completed as the FrankenPHP types API gets more complete.
>
> For class methods specifically, only primitive types are currently supported. Arrays and objects cannot be used as method parameters or return types yet.

If you refer to the code snippet of the previous section, you can see that helpers are used to convert the first parameter and the return value. The second and third parameter of our `repeat_this()` function don't need to be converted as memory representation of the underlying types are the same for both C and Go.

### Declaring a Native PHP Class

The generator supports declaring **opaque classes** as Go structs, which can be used to create PHP objects. You can use the `//export_php:class` directive comment to define a PHP class. For example:

```go
//export_php:class User
type UserStruct struct {
    Name string
    Age  int
}
```

#### What are Opaque Classes?

**Opaque classes** are classes where the internal structure (properties) is hidden from PHP code. This means:

* **No direct property access**: You cannot read or write properties directly from PHP (`$user->name` won't work)
* **Method-only interface** - All interactions must go through methods you define
* **Better encapsulation** - Internal data structure is completely controlled by Go code
* **Type safety** - No risk of PHP code corrupting internal state with wrong types
* **Cleaner API** - Forces to design a proper public interface

This approach provides better encapsulation and prevents PHP code from accidentally corrupting the internal state of your Go objects. All interactions with the object must go through the methods you explicitly define.

#### Adding Methods to Classes

Since properties are not directly accessible, you **must define methods** to interact with your opaque classes. Use the `//export_php:method` directive to define behavior:

```go
//export_php:class User
type UserStruct struct {
    Name string
    Age  int
}

//export_php:method User::getName(): string
func (us *UserStruct) GetUserName() unsafe.Pointer {
    return frankenphp.PHPString(us.Name, false)
}

//export_php:method User::setAge(int $age): void
func (us *UserStruct) SetUserAge(age int64) {
    us.Age = int(age)
}

//export_php:method User::getAge(): int
func (us *UserStruct) GetUserAge() int64 {
    return int64(us.Age)
}

//export_php:method User::setNamePrefix(string $prefix = "User"): void
func (us *UserStruct) SetNamePrefix(prefix *C.zend_string) {
    us.Name = frankenphp.GoString(unsafe.Pointer(prefix)) + ": " + us.Name
}
```

#### Nullable Parameters

The generator supports nullable parameters using the `?` prefix in PHP signatures. When a parameter is nullable, it becomes a pointer in your Go function, allowing you to check if the value was `null` in PHP:

```go
//export_php:method User::updateInfo(?string $name, ?int $age, ?bool $active): void
func (us *UserStruct) UpdateInfo(name *C.zend_string, age *int64, active *bool) {
    // Check if name was provided (not null)
    if name != nil {
        us.Name = frankenphp.GoString(unsafe.Pointer(name))
    }
    
    // Check if age was provided (not null)
    if age != nil {
        us.Age = int(*age)
    }
    
    // Check if active was provided (not null)
    if active != nil {
        us.Active = *active
    }
}
```

**Key points about nullable parameters:**

* **Nullable primitive types** (`?int`, `?float`, `?bool`) become pointers (`*int64`, `*float64`, `*bool`) in Go
* **Nullable strings** (`?string`) remain as `*C.zend_string` but can be `nil`
* **Check for `nil`** before dereferencing pointer values
* **PHP `null` becomes Go `nil`** - when PHP passes `null`, your Go function receives a `nil` pointer

> [!WARNING]
> Currently, class methods have the following limitations. **Arrays and objects are not supported** as parameter types or return types. Only primitive types are supported: `string`, `int`, `float`, `bool` and `void` (for return type). **Nullable parameter types are fully supported** for all primitive types (`?string`, `?int`, `?float`, `?bool`).

After generating the extension, you can use the class and its methods in PHP. Note that you **cannot access properties directly**:

```php
<?php

$user = new User();

// ✅ This works - using methods
$user->setAge(25);
echo $user->getName();           // Output: (empty, default value)
echo $user->getAge();            // Output: 25
$user->setNamePrefix("Employee");

// ✅ This also works - nullable parameters
$user->updateInfo("John", 30, true);        // All parameters provided
$user->updateInfo("Jane", null, false);     // Age is null
$user->updateInfo(null, 25, null);          // Name and active are null

// ❌ This will NOT work - direct property access
// echo $user->name;             // Error: Cannot access private property
// $user->age = 30;              // Error: Cannot access private property
```

This design ensures that your Go code has complete control over how the object's state is accessed and modified, providing better encapsulation and type safety.

### Declaring Constants

The generator supports exporting Go constants to PHP using two directives: `//export_php:const` for global constants and `//export_php:classconstant` for class constants. This allows you to share configuration values, status codes, and other constants between Go and PHP code.

#### Global Constants

Use the `//export_php:const` directive to create global PHP constants:

```go
//export_php:const
const MAX_CONNECTIONS = 100

//export_php:const
const API_VERSION = "1.2.3"

//export_php:const
const STATUS_OK = iota

//export_php:const
const STATUS_ERROR = iota
```

#### Class Constants

Use the `//export_php:classconstant ClassName` directive to create constants that belong to a specific PHP class:

```go
//export_php:classconstant User
const STATUS_ACTIVE = 1

//export_php:classconstant User
const STATUS_INACTIVE = 0

//export_php:classconstant User
const ROLE_ADMIN = "admin"

//export_php:classconstant Order
const STATE_PENDING = iota

//export_php:classconstant Order
const STATE_PROCESSING = iota

//export_php:classconstant Order
const STATE_COMPLETED = iota
```

Class constants are accessible using the class name scope in PHP:

```php
<?php

// Global constants
echo MAX_CONNECTIONS;    // 100
echo API_VERSION;        // "1.2.3"

// Class constants
echo User::STATUS_ACTIVE;    // 1
echo User::ROLE_ADMIN;       // "admin"
echo Order::STATE_PENDING;   // 0
```

The directive supports various value types including strings, integers, booleans, floats, and iota constants. When using `iota`, the generator automatically assigns sequential values (0, 1, 2, etc.). Global constants become available in your PHP code as global constants, while class constants are scoped to their respective classes. When using integers, different possible notation (binary, hex, octal) are supported and dumped as is in the PHP stub file.

You can use constants just like you are used to in the Go code. For example, let's take the `repeat_this()` function we declared earlier and change the last argument to an integer:

```go
import (
    "C"
    "github.com/dunglas/frankenphp"
    "strings"
)

//export_php:const
const STR_REVERSE = iota

//export_php:const
const STR_NORMAL = iota

//export_php:classconstant StringProcessor
const MODE_LOWERCASE = 1

//export_php:classconstant StringProcessor
const MODE_UPPERCASE = 2

//export_php:function repeat_this(string $str, int $count, int $mode): string
func repeat_this(s *C.zend_string, count int64, mode int) unsafe.Pointer {
    str := frankenphp.GoString(unsafe.Pointer(s))

    result := strings.Repeat(str, int(count))
    if mode == STR_REVERSE { 
        // reverse the string
    }

    if mode == STR_NORMAL {
        // no-op, just to showcase the constant
    }

    return frankenphp.PHPString(result, false)
}

//export_php:class StringProcessor
type StringProcessorStruct struct {
    // internal fields
}

//export_php:method StringProcessor::process(string $input, int $mode): string
func (sp *StringProcessorStruct) Process(input *C.zend_string, mode int64) unsafe.Pointer {
    str := frankenphp.GoString(unsafe.Pointer(input))
    
    switch mode {
    case MODE_LOWERCASE:
        str = strings.ToLower(str)
    case MODE_UPPERCASE:
        str = strings.ToUpper(str)
    }
    
    return frankenphp.PHPString(str, false)
}
```

After generating the extension, both global and class constants are available to use:

```php
<?php

// Using global constants
var_dump(repeat_this('Hello World', 5, STR_REVERSE));

// Using class constants
$processor = new StringProcessor();
echo $processor->process('Hello World', StringProcessor::MODE_LOWERCASE);  // "hello world"
echo $processor->process('Hello World', StringProcessor::MODE_UPPERCASE);  // "HELLO WORLD"
```

### Generating the Extension

This is where the magic happens, and your extension can now be generated. You can run the generator with the following command:

```console
GEN_STUB_FILE=php-src/build/gen_stub.php frankenphp extension-init my_extension.go 
```

> [!NOTE]
> Don't forget to set the `GEN_STUB_FILE` environment variable to the path of the `gen_stub.php` file in the PHP sources you downloaded earlier. This is the same `gen_stub.php` script mentioned in the manual implementation section.

If everything went well, a new directory named `build` should have been created. This directory contains the generated files for your extension, including the `my_extension.go` file with the generated PHP function stubs.

## Integrating the Extension into FrankenPHP

Our extension is now ready to be compiled and integrated into FrankenPHP. To do this, refer to the FrankenPHP [compilation documentation](compile.md) to learn how to compile FrankenPHP. The compilation process is the same for both approaches, with one key difference in the module path:

**For manual implementation:**

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
CGO_CFLAGS=$(php-config --includes) \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
xcaddy build \
    --output frankenphp \
    --with github.com/my-account/my-module
```

**For generator-based extensions:**

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
CGO_CFLAGS=$(php-config --includes) \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
xcaddy build \
    --output frankenphp \
    --with github.com/my-account/my-module/build
```

Note that for generator-based extensions, you point to the `/build` subdirectory that was created during the generation step. However, this is not mandatory: you can also copy the generated files to your module directory and point to it directly.

## Testing Your Extension

All that's left is to create a PHP file to test our extension. For example, create an `index.php` file with the following content:

```php
<?php

// For the basic function (manual implementation)
go_print();

// For the advanced function (manual implementation)
echo go_upper("hello world");

// For the generator function
var_dump(repeat_this("Hello World!", 4, true));

// For classes (generator approach)
$user = new User();
$user->setAge(25);
echo $user->getAge();
```

You can now run FrankenPHP with this file using `./frankenphp php-server`, and you should see your extension working.
