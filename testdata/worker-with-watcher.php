<?php

$jsonFile = file_get_contents(__DIR__ . '/files/test.json');
$textFile = file_get_contents(__DIR__ . '/files/test.txt');

$printFiles = function () use ($jsonFile, $textFile) {
    echo $jsonFile;
    echo $textFile;
};

while (frankenphp_handle_request($printFiles)) {

}
