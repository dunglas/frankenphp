<?php

header('Foo: bar');
header('Foo2: bar2');
http_response_code(201);

echo 'Hello';
