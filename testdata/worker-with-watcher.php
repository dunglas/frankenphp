<?php

$textFile = file_get_contents(__DIR__ . '/files/test.txt');

$printFiles = function () use ($textFile) {
    echo $textFile;
};

while (frankenphp_handle_request($printFiles)) {

}
