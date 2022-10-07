<?php

require_once __DIR__.'/_executor.php';
require_once __DIR__.'/persistent-object-require.php';

$foo = new MyObject('obj1');

return function () use ($foo) {
    echo 'request: ' . $_GET['i'] . "\n";
    echo 'class exists: ' . class_exists(MyObject::class) . "\n";
    echo 'id: ' . $foo->id . "\n";
    echo 'object id: '. spl_object_id($foo);
};
