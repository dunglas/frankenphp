<?php

require_once __DIR__.'/_executor.php';

return function () {
    header('Link: </style.css>; rel=preload; as=style');
    header("Request: {$_GET['i']}");
    headers_send(103);

    header_remove('Link');

    echo 'Hello';
};
