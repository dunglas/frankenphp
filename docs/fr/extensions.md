# Écrire des Extensions PHP en Go

Avec FrankenPHP, vous pouvez **écrire des extensions PHP en Go**, ce qui vous permet de créer des **fonctions natives haute performance** qui peuvent être appelées directement depuis PHP. Vos applications peuvent tirer parti de toute bibliothèque Go existante ou nouvelle, ainsi que du célèbre modèle de concurrence des **goroutines directement depuis votre code PHP**.

L'écriture d'extensions PHP se fait généralement en C, mais il est également possible de les écrire dans d'autres langages avec un peu de travail supplémentaire. Les extensions PHP permettent de tirer parti de la puissance des langages de bas niveau pour étendre les fonctionnalités de PHP, par exemple, en ajoutant des fonctions natives ou en optimisant des opérations spécifiques.

Grâce aux modules Caddy, vous pouvez écrire des extensions PHP en Go et les intégrer très rapidement dans FrankenPHP.

## Deux Approches

FrankenPHP offre deux façons de créer des extensions PHP en Go :

1. **Utilisation du Générateur d'Extensions** - L'approche recommandée qui génère tout le code standard nécessaire pour la plupart des cas d'usage, vous permettant de vous concentrer sur l'écriture de votre code Go
2. **Implémentation Manuelle** - Contrôle total sur la structure de l'extension pour les cas d'usage avancés

Nous commencerons par l'approche du générateur, car c'est le moyen le plus facile de commencer, puis nous montrerons l'implémentation manuelle pour ceux qui ont besoin d'un contrôle complet.

## Utilisation du Générateur d'Extensions

FrankenPHP est livré avec un outil qui vous permet de **créer une extension PHP** en utilisant uniquement Go. **Pas besoin d'écrire du code C** ou d'utiliser CGO directement : FrankenPHP inclut également une **API de types publique** pour vous aider à écrire vos extensions en Go sans avoir à vous soucier du **jonglage de types entre PHP/C et Go**.

> [!TIP]
> Si vous voulez comprendre comment les extensions peuvent être écrites en Go à partir de zéro, vous pouvez lire la section d'implémentation manuelle ci-dessous démontrant comment écrire une extension PHP en Go sans utiliser le générateur.

Gardez à l'esprit que cet outil n'est **pas un générateur d'extensions complet**. Il est destiné à vous aider à écrire des extensions simples en Go, mais il ne fournit pas les fonctionnalités les plus avancées des extensions PHP. Si vous devez écrire une extension plus **complexe et optimisée**, vous devrez peut-être écrire du code C ou utiliser CGO directement.

### Prérequis

Comme aussi couvert dans la section d'implémentation manuelle ci-dessous, vous devez [obtenir les sources PHP](https://www.php.net/downloads.php) et créer un nouveau module Go.

#### Créer un Nouveau Module et Obtenir les Sources PHP

La première étape pour écrire une extension PHP en Go est de créer un nouveau module Go. Vous pouvez utiliser la commande suivante pour cela :

```console
go mod init github.com/my-account/my-module
```

La seconde étape est [l'obtention des sources PHP](https://www.php.net/downloads.php) pour les étapes suivantes. Une fois que vous les avez, décompressez-les dans le répertoire de votre choix, mais pas à l'intérieur de votre module Go :

```console
tar xf php-*
```

### Écrire l'Extension

Tout est maintenant configuré pour écrire votre fonction native en Go. Créez un nouveau fichier nommé `stringext.go`. Notre première fonction prendra une chaîne comme argument, le nombre de fois à la répéter, un booléen pour indiquer s'il faut inverser la chaîne, et retournera la chaîne résultante. Cela devrait ressembler à ceci :

```go
import (
    "C"
    "github.com/dunglas/frankenphp"
    "strings"
)

//export_php:function repeat_this(string $str, int $count, bool $reverse): string
func repeat_this(s *C.zend_string, count int64, reverse bool) unsafe.Pointer {
    str := frankenphp.GoString(unsafe.Pointer(s))

    result := strings.Repeat(str, int(count))
    if reverse {
        runes := []rune(result)
        for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
            runes[i], runes[j] = runes[j], runes[i]
        }
        result = string(runes)
    }

    return frankenphp.PHPString(result, false)
}
```

Il y a deux choses importantes à noter ici :

* Une directive `//export_php:function` définit la signature de la fonction en PHP. C'est ainsi que le générateur sait comment générer la fonction PHP avec les bons paramètres et le bon type de retour ;
* La fonction doit retourner un `unsafe.Pointer`. FrankenPHP fournit une API pour vous aider avec le jonglage de types entre C et Go.

Alors que le premier point parle de lui-même, le second peut être plus difficile à appréhender. Plongeons plus profondément dans la jonglage de types dans la section suivante.

### Jonglage de Types

Bien que certains types de variables aient la même représentation mémoire entre C/PHP et Go, certains types nécessitent plus de logique pour être directement utilisés. C'est peut-être la partie la plus difficile quand il s'agit d'écrire des extensions car cela nécessite de comprendre les fonctionnements internes du moteur Zend et comment les variables sont stockées dans le moteur de PHP. Ce tableau résume ce que vous devez savoir :

| Type PHP           | Type Go          | Conversion directe | Assistant C vers Go     | Assistant Go vers C     | Support des Méthodes de Classe |
|--------------------|------------------|--------------------|-------------------------|-------------------------|--------------------------------|
| `int`              | `int64`          | ✅                  | -                       | -                       | ✅                              |
| `?int`             | `*int64`         | ✅                  | -                       | -                       | ✅                              |
| `float`            | `float64`        | ✅                  | -                       | -                       | ✅                              |
| `?float`           | `*float64`       | ✅                  | -                       | -                       | ✅                              |
| `bool`             | `bool`           | ✅                  | -                       | -                       | ✅                              |
| `?bool`            | `*bool`          | ✅                  | -                       | -                       | ✅                              |
| `string`/`?string` | `*C.zend_string` | ❌                  | frankenphp.GoString()   | frankenphp.PHPString()  | ✅                              |
| `array`            | `slice`/`map`    | ❌                  | _Pas encore implémenté_ | _Pas encore implémenté_ | ❌                              |
| `object`           | `struct`         | ❌                  | _Pas encore implémenté_ | _Pas encore implémenté_ | ❌                              |

> [!NOTE]
> Ce tableau n'est pas encore exhaustif et sera complété au fur et à mesure que l'API de types FrankenPHP deviendra plus complète.
>
> Pour les méthodes de classe spécifiquement, seuls les types primitifs sont actuellement supportés. Les tableaux et objets ne peuvent pas encore être utilisés comme paramètres de méthode ou types de retour.

Si vous vous référez à l'extrait de code de la section précédente, vous pouvez voir que des assistants sont utilisés pour convertir le premier paramètre et la valeur de retour. Les deuxième et troisième paramètres de notre fonction `repeat_this()` n'ont pas besoin d'être convertis car la représentation mémoire des types sous-jacents est la même pour C et Go.

### Déclarer une Classe PHP Native

Le générateur prend en charge la déclaration de **classes opaques** comme structures Go, qui peuvent être utilisées pour créer des objets PHP. Vous pouvez utiliser la directive `//export_php:class` pour définir une classe PHP. Par exemple :

```go
//export_php:class User
type UserStruct struct {
    Name string
    Age  int
}
```

#### Que sont les Classes Opaques ?

Les **classes opaques** sont des classes avec lesquelles la structure interne (comprendre : les propriétés) est cachée du code PHP. Cela signifie :

* **Pas d'accès direct aux propriétés** : Vous ne pouvez pas lire ou écrire des propriétés directement depuis PHP (`$user->name` ne fonctionnera pas)
* **Interface uniquement par méthodes** - Toutes les interactions doivent passer par les méthodes que vous définissez
* **Meilleure encapsulation** - La structure de données interne est complètement contrôlée par le code Go
* **Sécurité de type** - Aucun risque que le code PHP corrompe l'état interne avec de mauvais types
* **API plus propre** - Force à concevoir une interface publique appropriée

Cette approche fournit une meilleure encapsulation et empêche le code PHP de corrompre accidentellement l'état interne de vos objets Go. Toutes les interactions avec l'objet doivent passer par les méthodes que vous définissez explicitement.

#### Ajouter des Méthodes aux Classes

Puisque les propriétés ne sont pas directement accessibles, vous **devez définir des méthodes** pour interagir avec vos classes opaques. Utilisez la directive `//export_php:method` pour définir cela :

```go
//export_php:class User
type UserStruct struct {
    Name string
    Age  int
}

//export_php:method User::getName(): string
func (us *UserStruct) GetUserName() unsafe.Pointer {
    return frankenphp.PHPString(us.Name, false)
}

//export_php:method User::setAge(int $age): void
func (us *UserStruct) SetUserAge(age int64) {
    us.Age = int(age)
}

//export_php:method User::getAge(): int
func (us *UserStruct) GetUserAge() int64 {
    return int64(us.Age)
}

//export_php:method User::setNamePrefix(string $prefix = "User"): void
func (us *UserStruct) SetNamePrefix(prefix *C.zend_string) {
    us.Name = frankenphp.GoString(unsafe.Pointer(prefix)) + ": " + us.Name
}
```

#### Paramètres Nullables

Le générateur prend en charge les paramètres nullables en utilisant le préfixe `?` dans les signatures PHP. Quand un paramètre est nullable, il devient un pointeur dans votre fonction Go, vous permettant de vérifier si la valeur était `null` en PHP :

```go
//export_php:method User::updateInfo(?string $name, ?int $age, ?bool $active): void
func (us *UserStruct) UpdateInfo(name *C.zend_string, age *int64, active *bool) {
    // $name est null?
    if name != nil {
        us.Name = frankenphp.GoString(unsafe.Pointer(name))
    }
    
    // $age est null?
    if age != nil {
        us.Age = int(*age)
    }
    
    // $active est null?
    if active != nil {
        us.Active = *active
    }
}
```

**Points clés sur les paramètres nullables :**

* **Types primitifs nullables** (`?int`, `?float`, `?bool`) deviennent des pointeurs (`*int64`, `*float64`, `*bool`) en Go
* **Chaînes nullables** (`?string`) restent comme `*C.zend_string` mais peuvent être `nil`
* **Vérifiez `nil`** avant de déréférencer les valeurs de pointeur
* **PHP `null` devient Go `nil`** - quand PHP passe `null`, votre fonction Go reçoit un pointeur `nil`

> [!WARNING]
> Actuellement, les méthodes de classe ont les limitations suivantes. **Les tableaux et objets ne sont pas supportés** comme types de paramètres ou types de retour. Seuls les types scalaires sont supportés : `string`, `int`, `float`, `bool` et `void` (pour le type de retour). **Les types de paramètres nullables sont entièrement supportés** pour tous les types scalaires (`?string`, `?int`, `?float`, `?bool`).

Après avoir généré l'extension, vous serez autorisé à utiliser la classe et ses méthodes en PHP. Notez que vous **ne pouvez pas accéder aux propriétés directement** :

```php
<?php

$user = new User();

// ✅ Fonctionne - utilisation des méthodes
$user->setAge(25);
echo $user->getName();           // Output : (vide, valeur par défaut)
echo $user->getAge();            // Output : 25
$user->setNamePrefix("Employee");

// ✅ Fonctionne aussi - paramètres nullables
$user->updateInfo("John", 30, true);        // Tous les paramètres fournis
$user->updateInfo("Jane", null, false);     // L'âge est null
$user->updateInfo(null, 25, null);          // Le nom et actif sont null

// ❌ Ne fonctionnera PAS - accès direct aux propriétés
// echo $user->name;             // Erreur : Impossible d'accéder à la propriété privée
// $user->age = 30;              // Erreur : Impossible d'accéder à la propriété privée
```

Cette conception garantit que votre code Go a un contrôle complet sur la façon dont l'état de l'objet est accédé et modifié, fournissant une meilleure encapsulation et sécurité de type.

### Déclarer des Constantes

Le générateur prend en charge l'exportation de constantes Go vers PHP en utilisant deux directives : `//export_php:const` pour les constantes globales et `//export_php:classconstant` pour les constantes de classe. Cela vous permet de partager des valeurs de configuration, des codes de statut et d'autres constantes entre le code Go et PHP.

#### Constantes Globales

Utilisez la directive `//export_php:const` pour créer des constantes PHP globales :

```go
//export_php:const
const MAX_CONNECTIONS = 100

//export_php:const
const API_VERSION = "1.2.3"

//export_php:const
const STATUS_OK = iota

//export_php:const
const STATUS_ERROR = iota
```

#### Constantes de Classe

Utilisez la directive `//export_php:classconstant ClassName` pour créer des constantes qui appartiennent à une classe PHP spécifique :

```go
//export_php:classconstant User
const STATUS_ACTIVE = 1

//export_php:classconstant User
const STATUS_INACTIVE = 0

//export_php:classconstant User
const ROLE_ADMIN = "admin"

//export_php:classconstant Order
const STATE_PENDING = iota

//export_php:classconstant Order
const STATE_PROCESSING = iota

//export_php:classconstant Order
const STATE_COMPLETED = iota
```

Les constantes de classe sont accessibles en utilisant la portée du nom de classe en PHP :

```php
<?php

// Constantes globales
echo MAX_CONNECTIONS;    // 100
echo API_VERSION;        // "1.2.3"

// Constantes de classe
echo User::STATUS_ACTIVE;    // 1
echo User::ROLE_ADMIN;       // "admin"
echo Order::STATE_PENDING;   // 0
```

La directive prend en charge divers types de valeurs incluant les chaînes, entiers, booléens, flottants et constantes iota. Lors de l'utilisation de `iota`, le générateur assigne automatiquement des valeurs séquentielles (0, 1, 2, etc.). Les constantes globales deviennent disponibles dans votre code PHP comme constantes globales, tandis que les constantes de classe sont déclarées dans leurs classes respectives avec la visibilité publique. Lors de l'utilisation d'entiers, différentes notations possibles (binaire, hex, octale) sont supportées et dumpées telles quelles dans le fichier stub PHP.

Vous pouvez utiliser les constantes comme vous êtes habitué dans le code Go. Par exemple, prenons la fonction `repeat_this()` que nous avons déclarée plus tôt et changeons le dernier argument en entier :

```go
import (
    "C"
    "github.com/dunglas/frankenphp"
    "strings"
)

//export_php:const
const STR_REVERSE = iota

//export_php:const
const STR_NORMAL = iota

//export_php:classconstant StringProcessor
const MODE_LOWERCASE = 1

//export_php:classconstant StringProcessor
const MODE_UPPERCASE = 2

//export_php:function repeat_this(string $str, int $count, int $mode): string
func repeat_this(s *C.zend_string, count int64, mode int) unsafe.Pointer {
    str := frankenphp.GoString(unsafe.Pointer(s))

    result := strings.Repeat(str, int(count))
    if mode == STR_REVERSE { 
        // inverser la chaîne
    }

    if mode == STR_NORMAL {
        // no-op, juste pour montrer la constante
    }

    return frankenphp.PHPString(result, false)
}

//export_php:class StringProcessor
type StringProcessorStruct struct {
    // champs internes
}

//export_php:method StringProcessor::process(string $input, int $mode): string
func (sp *StringProcessorStruct) Process(input *C.zend_string, mode int64) unsafe.Pointer {
    str := frankenphp.GoString(unsafe.Pointer(input))
    
    switch mode {
    case MODE_LOWERCASE:
        str = strings.ToLower(str)
    case MODE_UPPERCASE:
        str = strings.ToUpper(str)
    }
    
    return frankenphp.PHPString(str, false)
}
```

### Générer l'Extension

C'est là que la magie opère, et votre extension peut maintenant être générée. Vous pouvez exécuter le générateur avec la commande suivante :

```console
GEN_STUB_FILE=php-src/build/gen_stub.php frankenphp extension-init my_extension.go 
```

> [!NOTE]
> N'oubliez pas de définir la variable d'environnement `GEN_STUB_FILE` sur le chemin du fichier `gen_stub.php` dans les sources PHP que vous avez téléchargées plus tôt. C'est le même script `gen_stub.php` mentionné dans la section d'implémentation manuelle.

Si tout s'est bien passé, un nouveau répertoire nommé `build` devrait avoir été créé. Ce répertoire contient les fichiers générés pour votre extension, incluant le fichier `my_extension.go` avec les stubs de fonction PHP générés.

### Intégrer l'Extension Générée dans FrankenPHP

Notre extension est maintenant prête à être compilée et intégrée dans FrankenPHP. Pour ce faire, référez-vous à la [documentation de compilation](compile.md) de FrankenPHP pour apprendre comment compiler FrankenPHP. Ajoutez le module en utilisant le flag `--with`, pointant vers le chemin de votre module :

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
CGO_CFLAGS=$(php-config --includes) \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
xcaddy build \
    --output frankenphp \
    --with github.com/my-account/my-module/build
```

Notez que vous pointez vers le sous-répertoire `/build` qui a été créé pendant l'étape de génération. Cependant, ce n'est pas obligatoire : vous pouvez aussi copier les fichiers générés dans le répertoire de votre module et pointer directement vers lui.

### Tester Votre Extension Générée

Vous pouvez créer un fichier PHP pour tester les fonctions et classes que vous avez créées. Par exemple, créez un fichier `index.php` avec le contenu suivant :

```php
<?php

// Utilisation des constantes globales
var_dump(repeat_this('Hello World', 5, STR_REVERSE));

// Utilisation des constantes de classe
$processor = new StringProcessor();
echo $processor->process('Hello World', StringProcessor::MODE_LOWERCASE);  // "hello world"
echo $processor->process('Hello World', StringProcessor::MODE_UPPERCASE);  // "HELLO WORLD"
```

Une fois que vous avez intégré votre extension dans FrankenPHP comme indiqué dans la section précédente, vous pouvez exécuter ce fichier de test en utilisant `./frankenphp php-server`, et vous devriez voir votre extension fonctionner.

## Implémentation Manuelle

Si vous voulez comprendre comment les extensions fonctionnent ou avez besoin d'un contrôle total sur votre extension, vous pouvez les écrire manuellement. Cette approche vous donne un contrôle complet mais nécessite plus de code intermédiaire.

### Fonction de Base

Nous allons voir comment écrire une extension PHP simple en Go qui définit une nouvelle fonction native. Cette fonction sera appelée depuis PHP et déclenchera une goroutine qui enregistrera un message dans les logs de Caddy. Cette fonction ne prend aucun paramètre et ne retourne rien.

#### Définir la Fonction Go

Dans votre module Go vide, vous devez définir une nouvelle fonction native qui sera appelée depuis PHP. Pour ce faire, créez un fichier avec le nom que vous voulez, par exemple, `extension.go`, et ajoutez le code suivant :

```go
package ext_go

//#include "extension.h"
import "C"
import (
    "unsafe"
    "github.com/caddyserver/caddy/v2"
    "github.com/dunglas/frankenphp"
)

func init() {
    frankenphp.RegisterExtension(unsafe.Pointer(&C.ext_module_entry))
}

//export go_print_something
func go_print_something() {
    go func() {
        caddy.Log().Info("Hello from a goroutine!")
    }()
}
```

La fonction `frankenphp.RegisterExtension()` simplifie le processus d'enregistrement d'extension en gérant la logique interne de PHP. La fonction `go_print_something` utilise la directive `//export` pour indiquer qu'elle sera accessible dans le code C que nous écrirons, grâce à CGO.

Dans cet exemple, notre nouvelle fonction déclenchera une goroutine qui enregistrera un message dans les logs de Caddy.

#### Définir la Fonction PHP

Pour permettre à PHP d'appeler notre fonction, nous devons définir une fonction PHP correspondante. Pour cela, nous créerons un fichier stub, par exemple, `extension.stub.php`, qui contiendra le code suivant :

```php
<?php

/** @generate-class-entries */

function go_print(): void {}
```

Ce fichier définit la signature de la fonction `go_print()`, qui sera appelée depuis PHP. La directive `@generate-class-entries` permet à PHP de générer automatiquement les entrées de fonction pour notre extension.

Ceci n'est pas fait manuellement mais en utilisant un script fourni dans les sources PHP (assurez-vous d'ajuster le chemin vers le script `gen_stub.php` selon l'emplacement de vos sources PHP) :

```bash
php ../php-src/build/gen_stub.php extension.stub.php
```

Ce script générera un fichier nommé `extension_arginfo.h` qui contient les informations nécessaires pour que PHP sache comment définir et appeler notre fonction.

#### Écrire le Pont entre Go et C

Maintenant, nous devons écrire le pont entre Go et C. Créez un fichier nommé `extension.h` dans le répertoire de votre module avec le contenu suivant :

```c
#ifndef _EXTENSION_H
#define _EXTENSION_H

#include <php.h>

extern zend_module_entry ext_module_entry;

#endif
```

Ensuite, créez un fichier nommé `extension.c` qui effectuera les étapes suivantes :

* Inclure les en-têtes PHP ;
* Déclarer notre nouvelle fonction PHP native `go_print()` ;
* Déclarer les métadonnées de l'extension.

Commençons par inclure les en-têtes requis :

```c
#include <php.h>
#include "extension.h"
#include "extension_arginfo.h"

// Contient les symboles exportés par Go
#include "_cgo_export.h"
```

Nous définissons ensuite notre fonction PHP comme une fonction de langage natif :

```c
PHP_FUNCTION(go_print)
{
    ZEND_PARSE_PARAMETERS_NONE();

    go_print_something();
}

zend_module_entry ext_module_entry = {
    STANDARD_MODULE_HEADER,
    "ext_go",
    ext_functions, /* Functions */
    NULL,          /* MINIT */
    NULL,          /* MSHUTDOWN */
    NULL,          /* RINIT */
    NULL,          /* RSHUTDOWN */
    NULL,          /* MINFO */
    "0.1.1",
    STANDARD_MODULE_PROPERTIES
};
```

Dans ce cas, notre fonction ne prend aucun paramètre et ne retourne rien. Elle appelle simplement la fonction Go que nous avons définie plus tôt, exportée en utilisant la directive `//export`.

Enfin, nous définissons les métadonnées de l'extension dans une structure `zend_module_entry`, telles que son nom, sa version et ses propriétés. Cette information est nécessaire pour que PHP reconnaisse et charge notre extension. Notez que `ext_functions` est un tableau de pointeurs vers les fonctions PHP que nous avons définies, et il a été automatiquement généré par le script `gen_stub.php` dans le fichier `extension_arginfo.h`.

L'enregistrement de l'extension est automatiquement géré par la fonction `RegisterExtension()` de FrankenPHP que nous appelons dans notre code Go.

### Usage Avancé

Maintenant que nous savons comment créer une extension PHP de base en Go, complexifions notre exemple. Nous allons maintenant créer une fonction PHP qui prend une chaîne comme paramètre et retourne sa version en majuscules.

#### Définir le Stub de Fonction PHP

Pour définir la nouvelle fonction PHP, nous modifierons notre fichier `extension.stub.php` pour inclure la nouvelle signature de fonction :

```php
<?php

/** @generate-class-entries */

/**
 * Convertit une chaîne en majuscules.
 *
 * @param string $string La chaîne à convertir.
 * @return string La version en majuscules de la chaîne.
 */
function go_upper(string $string): string {}
```

> [!TIP]
> Ne négligez pas la documentation de vos fonctions ! Vous êtes susceptible de partager vos stubs d'extension avec d'autres développeurs pour documenter comment utiliser votre extension et quelles fonctionnalités sont disponibles.

En régénérant le fichier stub avec le script `gen_stub.php`, le fichier `extension_arginfo.h` devrait ressembler à ceci :

```c
ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_go_upper, 0, 1, IS_STRING, 0)
    ZEND_ARG_TYPE_INFO(0, string, IS_STRING, 0)
ZEND_END_ARG_INFO()

ZEND_FUNCTION(go_upper);

static const zend_function_entry ext_functions[] = {
    ZEND_FE(go_upper, arginfo_go_upper)
    ZEND_FE_END
};
```

Nous pouvons voir que la fonction `go_upper` est définie avec un paramètre de type `string` et un type de retour `string`.

#### Jonglerie de Types entre Go et PHP/C

Votre fonction Go ne peut pas accepter directement une chaîne PHP comme paramètre. Vous devez la convertir en chaîne Go. Heureusement, FrankenPHP fournit des fonctions d'aide pour gérer la conversion entre les chaînes PHP et les chaînes Go, similaire à ce que nous avons vu dans l'approche du générateur.

Le fichier d'en-tête reste simple :

```c
#ifndef _EXTENSION_H
#define _EXTENSION_H

#include <php.h>

extern zend_module_entry ext_module_entry;

#endif
```

Nous pouvons maintenant écrire le pont entre Go et C dans notre fichier `extension.c`. Nous passerons la chaîne PHP directement à notre fonction Go :

```c
PHP_FUNCTION(go_upper)
{
    zend_string *str;

    ZEND_PARSE_PARAMETERS_START(1, 1)
        Z_PARAM_STR(str)
    ZEND_PARSE_PARAMETERS_END();

    zend_string *result = go_upper(str);
    RETVAL_STR(result);
}
```

Vous pouvez en apprendre plus sur `ZEND_PARSE_PARAMETERS_START` et l'analyse des paramètres dans la page dédiée du [PHP Internals Book](https://www.phpinternalsbook.com/php7/extensions_design/php_functions.html#parsing-parameters-zend-parse-parameters). Ici, nous disons à PHP que notre fonction prend un paramètre obligatoire de type `string` comme `zend_string`. Nous passons ensuite cette chaîne directement à notre fonction Go et retournons le résultat en utilisant `RETVAL_STR`.

Il ne reste qu'une chose à faire : implémenter la fonction `go_upper` en Go.

#### Implémenter la Fonction Go

Notre fonction Go prendra un `*C.zend_string` comme paramètre, le convertira en chaîne Go en utilisant la fonction d'aide de FrankenPHP, le traitera, et retournera le résultat comme un nouveau `*C.zend_string`. Les fonctions d'aide gèrent toute la complexité de gestion de mémoire et de conversion pour nous.

```go
import "strings"

//export go_upper
func go_upper(s *C.zend_string) *C.zend_string {
    str := frankenphp.GoString(unsafe.Pointer(s))
    
    upper := strings.ToUpper(str)
    
    return (*C.zend_string)(frankenphp.PHPString(upper, false))
}
```

Cette approche est beaucoup plus propre et sûre que la gestion manuelle de la mémoire. Les fonctions d'aide de FrankenPHP gèrent la conversion entre le format `zend_string` de PHP et les chaînes Go automatiquement. Le paramètre `false` dans `PHPString()` indique que nous voulons créer une nouvelle chaîne non persistante (libérée à la fin de la requête).

> [!TIP]
> Dans cet exemple, nous n'effectuons aucune gestion d'erreur, mais vous devriez toujours vérifier que les pointeurs ne sont pas `nil` et que les données sont valides avant de les utiliser dans vos fonctions Go.

### Intégrer l'Extension dans FrankenPHP

Notre extension est maintenant prête à être compilée et intégrée dans FrankenPHP. Pour ce faire, référez-vous à la [documentation de compilation](compile.md) de FrankenPHP pour apprendre comment compiler FrankenPHP. Ajoutez le module en utilisant le flag `--with`, pointant vers le chemin de votre module :

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
CGO_CFLAGS=$(php-config --includes) \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
xcaddy build \
    --output frankenphp \
    --with github.com/my-account/my-module
```

C'est tout ! Votre extension est maintenant intégrée dans FrankenPHP et peut être utilisée dans votre code PHP.

### Tester Votre Extension

Après avoir intégré votre extension dans FrankenPHP, vous pouvez créer un fichier `index.php` avec des exemples pour les fonctions que vous avez implémentées :

```php
<?php

// Tester la fonction de base
go_print();

// Tester la fonction avancée
echo go_upper("hello world") . "\n";
```

Vous pouvez maintenant exécuter FrankenPHP avec ce fichier en utilisant `./frankenphp php-server`, et vous devriez voir votre extension fonctionner.
