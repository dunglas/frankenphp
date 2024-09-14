#include "_cgo_export.h"
#include "watcher-c.h"

void process_event(struct wtr_watcher_event event, void *data) {
  go_handle_event((char *)event.path_name, event.effect_type, event.path_type, (uintptr_t)data);
}

void *start_new_watcher(char const *const path, uintptr_t data) {
  void *watcher = wtr_watcher_open(path, process_event, (void *)data);
  if (!watcher) {
    return NULL;
  }
  return watcher;
}

int stop_watcher(void *watcher) {
  if (!wtr_watcher_close(watcher)) {
    return 1;
  }
  return 0;
}