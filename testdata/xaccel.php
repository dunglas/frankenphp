<?php

header('Content-Type: text/plain');
header("X-Accel-Redirect: " . ($_GET['redir'] ?? '/hello.txt'));

echo "hello from php";
