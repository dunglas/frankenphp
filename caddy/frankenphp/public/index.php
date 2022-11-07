<?php

ini_set('output_buffering', '0');

header('Content-Type: text/plain');

echo "immediate\n";
flush();
echo "later?\n";
sleep(2);
flush();
sleep(3);
echo "more later\n";
die();
