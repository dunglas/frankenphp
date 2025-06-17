# Writing PHP Extensions in Go

FrankenPHP is bundled with a tool that allows you **to create a PHP extension** only using Go.
**No need to write C code** or use CGO directly: FrankenPHP also includes a **public types API**
to help you write your extensions in Go without having to worry about
**the type juggling between PHP/C and Go**.

> [!TIP]
> If you want to understand how extensions can be written in Go from scratch, you can read the
> dedicated page of the [FrankenPHP documentation](extensions.md) demonstrating how to write a
> PHP extension in Go without using the generator.

Keep in mind that this tool is **not a full-fledged extension generator**. It is meant to help you write simple
extensions in Go, but it does not provide the most advanced features of PHP extensions. If you need to write a more
**complex and optimized** extension, you may need to write some C code or use CGO directly.

## Creating a Native Function

We will first see how to create a new native function in Go that can be called from PHP.

### Prerequisites

The first thing to do is to [get the PHP sources](https://www.php.net/downloads.php) before going further. Once you have
them, decompress them in the directory of your choice:

```console
tar xf php-*
```

Keep in mind the directory where you decompressed the PHP sources, as you will need it later. You can now create a new
Go module in the directory of your choice:

```console
go mod init github.com/my-account/my-module
```

### Writing the Extension

Everything is now setup to write your native function in Go. Create a new file named `stringext.go`. Our first function
will take a string as an argument, the number of times to repeat it, a boolean to indicate whether to reverse the
string, and return the resulting string. This should look like this:

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
* The function must return an `unsafe.Pointer`. FrankenPHP provides an API to help you with type juggling between C and
  Go.

While the first point speaks for itself, the second may be harder to apprehend. Let's take a deeper dive to type
juggling in the next section.

## Type Juggling

While some variable types have the same memory representation between C/PHP and Go, some types require more logic to be
directly used. This is maybe the hardest part when it comes to writing extensions because it requires understanding
internals of the Zend Engine and how variables are stored internally in PHP. This table summarizes what you need to
know:

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
> For class methods specifically, only primitive types are currently supported. Arrays and objects cannot
> be used as method parameters or return types yet.

If you refer to the code snippet of the previous section, you can see that helpers are used to convert the first
parameter and the return value. The second and third parameter of our `repeat_this()` function don't need to be
converted as memory representation of the underlying types are the same for both C and Go.

## Declaring a Native PHP Class

The generator supports declaring **opaque classes** as Go structs, which can be used to create PHP objects. You can use the
`//export_php:class` directive comment to define a PHP class. For example:

```go
//export_php:class User
type UserStruct struct {
    Name string
    Age  int
}
```

### What are Opaque Classes?

**Opaque classes** are classes where the internal structure (properties) is hidden from PHP code. This means:

* **No direct property access**: You cannot read or write properties directly from PHP (`$user->name` won't work)
* **Method-only interface** - All interactions must go through methods you define
* **Better encapsulation** - Internal data structure is completely controlled by Go code
* **Type safety** - No risk of PHP code corrupting internal state with wrong types
* **Cleaner API** - Forces to design a proper public interface

This approach provides better encapsulation and prevents PHP code from accidentally corrupting the internal state of
your Go objects. All interactions with the object must go through the methods you explicitly define.

### Adding Methods to Classes

Since properties are not directly accessible, you **must define methods** to interact with your opaque classes. Use
the `//export_php:method` directive to define behavior:

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

### Nullable Parameters

The generator supports nullable parameters using the `?` prefix in PHP signatures. When a parameter is nullable, it
becomes a pointer in your Go function, allowing you to check if the value was `null` in PHP:

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
> Currently, class methods have the following limitations.
> **Arrays and objects are not supported** as parameter types or return types. Only primitive types are
> supported: `string`, `int`, `float`, `bool` and `void` (for return type).
> **Nullable parameter types are fully supported** for all primitive types (`?string`, `?int`, `?float`, `?bool`).

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

This design ensures that your Go code has complete control over how the object's state is accessed and
modified, providing better encapsulation and type safety.

## Declaring Constants

The generator supports exporting Go constants to PHP using two directives: `//export_php:const` for global constants
and `//export_php:classconstant` for class constants. This allows you to share configuration values, status codes,
and other constants between Go and PHP code.

### Global Constants

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

### Class Constants

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

The directive supports various value types including strings, integers, booleans, floats, and iota constants. When
using `iota`, the generator automatically assigns sequential values (0, 1, 2, etc.). Global constants become available
in your PHP code as global constants, while class constants are scoped to their respective classes. When using integers,
different possible notation (binary, hex, octal) are supported and dumped as is in the PHP stub file.

You can use constants just like you are used to in the Go code. For example, let's take the `repeat_this()` function
we declared earlier and change the last argument to an integer:

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

## Generating the Extension

This is where the magic happens, and your extension can now be generated. You can run the generator with the following
command:

```console
GEN_STUB_FILE=php-src/build/gen_stub.php frankenphp extension-init my_extension.go 
```

> [!NOTE]
> Don't forget to set the `GEN_STUB_FILE` environment variable to the path of the `gen_stub.php` file in the PHP
> sources. This file is used by the generator to create the PHP function stubs.

If everything went well, a new directory named `build` should have been created. This directory contains the generated
files for your extension, including the `my_extension.go` file with the generated PHP function stubs.

### Integrate the Extension into FrankenPHP

Our extension is now ready to be compiled and integrated into FrankenPHP. To do this, refer to the
FrankenPHP [compilation documentation](compile.md) to learn how to compile FrankenPHP. The only difference is that you
need to add your Go module to the compilation command. Using `xcaddy`, it will look like this:

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
CGO_CFLAGS=$(php-config --includes) \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
xcaddy build \
    --output frankenphp \
    --with github.com/my-account/my-module/build
```

All that's left is to create a PHP file to test the extension. For example, create an `index.php` file with the
following content:

```php
<?php

var_dump(repeat_this("Hello World!", 4, true));
```

You can now run FrankenPHP with this file using `./frankenphp php-server`, and you should see the message
`string(48) "!dlroW olleH!dlroW olleH!dlroW olleH!dlroW olleH"` on your screen.
