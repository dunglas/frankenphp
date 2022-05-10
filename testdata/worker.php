<?php

do {
    $ok = frankenphp_handle_request(function (RequestWriter $rw, Request $r): void {

    });
    echo 'Hey';
} while ($ok);
