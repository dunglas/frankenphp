<?php

require_once __DIR__.'/_executor.php';

return function () {
    header('Foo: bar');

    echo file_get_contents('php://input');
};
