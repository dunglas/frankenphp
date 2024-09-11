<?php
require_once __DIR__.'/_executor.php';

return function()
{
    $uploaded = ($_FILES['file']['tmp_name'] ?? null) ? file_get_contents($_FILES['file']['tmp_name']) : null;
    if ($uploaded) {
        echo 'Upload OK'; 
        return;
    }

    echo <<<'HTML'
    <form method="POST" enctype="multipart/form-data">
        <input type="file" name="file">
        <input type="submit">
    </form>
    HTML;
};
