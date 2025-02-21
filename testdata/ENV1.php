<?php

namespace MyApp;

$ENV1 = new class
{

    public function bootstrap(): void
    {

        $_ENV['my_var'] = 'value is defined!';

    }

};