<?php

$file_path = 'Zend/zend_portability.h';
$content = file_get_contents($file_path);
$content = str_replace('__assume(c)', 'assert(c)', $content);
$content = str_replace('__vectorcall', '__fastcall', $content);
$content = str_replace('__forceinline', 'inline', $content);
file_put_contents($file_path, $content);
echo "process Zend/zend_portability.h done;\n";

$directories = ['ext/hash/murmur', 'ext/hash/xxhash', 'ext/opcache/jit/vtune', 'win32'];
foreach ($directories as $dir) {
    $files = scandir($dir);
    foreach ($files as $file) {
        if (is_file($dir . '/' . $file)) {
            $file_path = $dir . '/' . $file;
            
            $content = file_get_contents($file_path);
            $new_content = str_replace('__forceinline', 'inline', $content);
            
            if ($content !== $new_content) {
                file_put_contents($file_path, $new_content);
                echo "$file_path 中的 __forceinline 已替换为 inline\n";
            }
        }
    }
}
echo "Replace __forceinline in assigned directory done;\n";

$file_path = 'sapi/embed/php_embed.h';
$content = file_get_contents($file_path);
$old_code = '#ifndef PHP_WIN32
    #define EMBED_SAPI_API SAPI_API
#else
    #define EMBED_SAPI_API
#endif';
$new_code = '#ifndef __MINGW64__
    #define EMBED_SAPI_API __declspec(dllexport)
#else
    #define EMBED_SAPI_API __declspec(dllimport)
#endif';
$content = str_replace($old_code, $new_code, $content);
file_put_contents($file_path, $content);
echo "Process sapi/embed/php_embed.h done;\n";

$file_path = 'Makefile';
$content = file_get_contents($file_path);
$content = str_replace('php.exe php8embed.lib', 'php.exe php8embed.dll', $content);
$content = str_replace('php8embed.lib: $(BUILD_DIR)\php8embed.lib', 'php8embed.dll: $(BUILD_DIR)\php8embed.dll', $content);
$content = str_replace('$(BUILD_DIR)\php8embed.lib:', '$(BUILD_DIR)\php8embed.dll:', $content);
$content = str_replace('$(MAKE_LIB)', '@"$(LINK)"', $content);
$content = str_replace('/nologo /out:$(BUILD_DIR)\php8embed.lib', '/nologo /DLL /out:$(BUILD_DIR)\php8embed.dll', $content);
file_put_contents($file_path, $content);
echo "Process Makefile done;\n";