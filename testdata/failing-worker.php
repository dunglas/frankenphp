<?php

$fail = random_int(1, 100) > 50;
$wait = random_int(1, 5);

sleep($wait);
if($fail) {
    exit(1);
}

while(frankenphp_handle_request(function() {
    echo "ok";
})) {
    $fail = random_int(1, 100) > 50;
    if ($fail) {
        exit(1);
    }
}
