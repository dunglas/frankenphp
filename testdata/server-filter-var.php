<?php
/**
 * This file tests the filter_input(INPUT_SERVER, $name) feature
 * Specifically when it's enabled via fringe_mode
 * If it's not enabled, just echo out the variables directly from $_SERVER
 */

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
         ] as $name) {

    if ($_GET['withFilterVar'] === 'true') {
        echo "$name:" . filter_input(INPUT_SERVER, $name) . "\n";
    } else {
        echo "$name:" . $_SERVER[$name] . "\n";
    }

}
echo "</pre>";