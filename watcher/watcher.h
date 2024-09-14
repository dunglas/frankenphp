#include <stdint.h>
#include <stdlib.h>

void *start_new_watcher(char const *const path, uintptr_t data);

int stop_watcher(void *watcher);