<?php

echo "<pre>\n";
foreach ([
             'CONTENT_LENGTH',
             'HTTP_CONTENT_LENGTH',
             'HTTP_SPECIAL_CHARS',
             'DOCUMENT_ROOT',
             'DOCUMENT_URI',
             'GATEWAY_INTERFACE',
             'HTTP_HOST',
             'HTTPS',
             'PATH_INFO',
             'CONTENT_TYPE',
             'DOCUMENT_ROOT',
             'REMOTE_ADDR',
             'CONTENT_LENGTH',
             'PHP_SELF',
             'REMOTE_HOST',
             'REQUEST_SCHEME',
             'SCRIPT_FILENAME',
             'SCRIPT_NAME',
             'SERVER_NAME',
             'SERVER_PORT',
             'SERVER_PROTOCOL',
             'SERVER_SOFTWARE',
             'SSL_PROTOCOL',
             'AUTH_TYPE',
             'REMOTE_IDENT',
             'CONTENT_TYPE',
             'PATH_TRANSLATED',
             'QUERY_STRING',
             'REMOTE_USER',
             'REQUEST_METHOD',
             'REQUEST_URI',
             'HTTP_X_EMPTY_HEADER',
         ] as $name) {
    echo "$name:" . $_SERVER[$name] . "\n";
}
echo "</pre>";