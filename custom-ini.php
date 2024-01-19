<?php
// patch it before `./buildconf` executed
if (patch_point() === 'before-php-buildconf') {
    \SPC\store\FileSystem::replaceFileStr(
        SOURCE_PATH . '/php-src/sapi/cli/php_cli.c',
        'sapi_module->php_ini_ignore_cwd = 1;',
        'sapi_module->php_ini_ignore_cwd = 0;'
    );
}
