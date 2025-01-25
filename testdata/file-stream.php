<?php

$fileStream = fopen(__DIR__ . '/file-stream.txt', 'r');
$input = fopen('php://input', 'r');

while (frankenphp_handle_request(function () use ($fileStream, $input) {
    echo fread($fileStream, 5);

    // this line will lead to a zend_mm_heap corrupted error if the input stream was destroyed
    stream_is_local($input);
})) ;

fclose($fileStream);
