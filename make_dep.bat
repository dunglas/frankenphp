@echo off

REM process php.h
set "php_h_path=.\x64\Release_TS\php-8.3.0-devel-vs16-x64\include\main\php.h"
if exist "%php_h_path%" (
    echo Updating pid_t definition in %php_h_path%...
    powershell -Command "(Get-Content '%php_h_path%' | ForEach-Object { $_ -replace '^typedef int pid_t;', 'typedef long long pid_t;' }) | Set-Content '%php_h_path%'"
) else (
    echo File %php_h_path% does not exist.
)

REM copy include
set "src_include_dir=.\x64\Release_TS\php-8.3.0-devel-vs16-x64\include"
set "dest_include_dir=C:\msys64\usr\local\include\php"

if not exist "%dest_include_dir%" (
    echo Creating directory %dest_include_dir%...
    mkdir "%dest_include_dir%"
)

echo Copying folders from %src_include_dir% to %dest_include_dir%...
xcopy "%src_include_dir%\*" "%dest_include_dir%\" /E /I /Y

REM copy php8ts.dll php8embed.dll
set "src_php_dll_dir=.\x64\Release_TS"
set "dest_lib_dir=C:\msys64\usr\local\lib"

if not exist "%dest_lib_dir%" (
    echo Creating directory %dest_lib_dir%...
    mkdir "%dest_lib_dir%"
)

echo Copying php8ts.dll and php8embed.dll to %dest_lib_dir%...
copy "%src_php_dll_dir%\php8ts.dll" "%dest_lib_dir%\" /Y
copy "%src_php_dll_dir%\php8embed.dll" "%dest_lib_dir%\" /Y

echo Done!
