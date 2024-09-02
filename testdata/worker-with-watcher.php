<?php

$numberOfRequests = 0;
$printFiles = function () use (&$numberOfRequests) {
    $numberOfRequests++;
    echo "requests:$numberOfRequests";
};

while (frankenphp_handle_request($printFiles)) {

}
