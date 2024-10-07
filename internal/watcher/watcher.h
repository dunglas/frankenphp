#include <stdint.h>
#include <stdlib.h>

uintptr_t start_new_watcher(char const *const path, uintptr_t data);

int stop_watcher(uintptr_t watcher);
