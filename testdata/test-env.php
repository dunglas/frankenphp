<?php
// Setting an environment variable
$result = putenv('MY_VAR=HelloWorld');
if ($result) {
    echo "Set MY_VAR successfully.\n";
    echo "MY_VAR = " . getenv('MY_VAR') . "\n";
} else {
    echo "Failed to set MY_VAR.\n";
}

// Unsetting the environment variable
$result = putenv('MY_VAR');
if ($result) {
    echo "Unset MY_VAR successfully.\n";
    $value = getenv('MY_VAR');
    if ($value === false) {
        echo "MY_VAR is unset.\n";
    } else {
        echo "MY_VAR = " . $value . "\n";
    }
} else {
    echo "Failed to unset MY_VAR.\n";
}

// Attempt to unset a non-existing variable
$result = putenv('NON_EXISTING_VAR');
if ($result) {
    echo "Unset NON_EXISTING_VAR successfully.\n";
} else {
    echo "Failed to unset NON_EXISTING_VAR.\n";
}
